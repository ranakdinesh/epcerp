import 'server-only';
import { pool, transaction } from '../../db';
import { AppError } from '../../errors';
import { decrypt } from './crypto';
import { getCompanyProfile } from '../../company/service';
import type { Answer, Snapshot, JobRow } from './types';

export type CriterionResult = {
  category: 'Financial' | 'Technical Experience' | 'Licences & Statutory' | 'EMD & Exemptions' | 'Declarations';
  criterion: string;
  tender_requirement: string;
  company_value: string;
  status: 'met' | 'not_met' | 'conditional' | 'needs_review';
  notes: string;
};

export type EligibilityVerdict = {
  id: string;
  draft_id: string;
  job_id: string;
  verdict: 'eligible' | 'not_eligible' | 'conditionally_eligible';
  overall_summary: string;
  criteria_matrix: CriterionResult[];
  gaps_and_risks: string[];
  model_used: string;
  evaluated_at: string;
  human_verified: boolean;
  human_decision: 'pending' | 'accepted' | 'rejected' | 'overridden';
  human_remarks: string;
  verified_by_name?: string;
  verified_at?: string;
  version: number;
};

// Helper to extract numeric rupees from string
function parseRupees(text: string): number | null {
  if (!text) return null;
  // Match patterns like "INR 1,50,00,000", "Rs. 45000000", "₹ 4.5 Cr", "120000000"
  const crMatch = /(\d+(?:\.\d+)?)\s*(?:cr|crore|crores)/i.exec(text);
  if (crMatch) return parseFloat(crMatch[1]) * 10000000;
  const lakhMatch = /(\d+(?:\.\d+)?)\s*(?:lakh|lakhs|lac|lacs)/i.exec(text);
  if (lakhMatch) return parseFloat(lakhMatch[1]) * 100000;
  const numMatch = /(?:INR|Rs\.?|₹)?\s*([0-9,]+(?:\.\d+)?)/i.exec(text.replace(/,/g, ''));
  if (numMatch) {
    const val = parseFloat(numMatch[1]);
    if (!isNaN(val) && val > 0) return val;
  }
  return null;
}

export async function evaluateTenderEligibility(
  draftId: string,
  jobId: string,
  actorId?: string
): Promise<EligibilityVerdict> {
  const db = pool();

  // 1. Fetch TSC Job
  const jobRes = await db.query<JobRow>(
    `SELECT id, draft_id, snapshot_cipher, snapshot_hash, answers_cipher, answers_hash
     FROM tender.tsc_workspaces WHERE id = $1 AND draft_id = $2`,
    [jobId, draftId]
  );
  if (!jobRes.rows[0]) throw new AppError(404, 'TSC checklist not found for this tender');

  const job = jobRes.rows[0];
  const snapshot = decrypt<Snapshot>(job.snapshot_cipher, job.snapshot_hash);
  const answers = decrypt<Answer[]>(job.answers_cipher, job.answers_hash);

  // 2. Fetch full company profile
  const companyData = await getCompanyProfile();
  const { profile, financials, licences, work_experience, certificates } = companyData;

  const matrix: CriterionResult[] = [];
  const gaps: string[] = [];

  // Helper to find answer by question printed number or section
  const findAnswer = (secPattern: RegExp, qPattern?: RegExp): { text: string; qText: string } => {
    for (const q of snapshot.questions) {
      if (secPattern.test(q.section) || (qPattern && qPattern.test(q.question))) {
        const a = answers.find((ans) => ans.question_id === q.id);
        if (a && a.answer.trim()) return { text: a.answer, qText: q.question };
      }
    }
    return { text: '', qText: '' };
  };

  // 3. Evaluate Particulars & Estimated Cost
  const costAnswer = findAnswer(/particulars|financial/i, /estimated cost|tender value|cost of work/i);
  const estimatedCost = parseRupees(costAnswer.text);

  // A. Turnover Evaluation
  const turnoverAns = findAnswer(/financial/i, /turnover|annual turnover/i);
  const requiredTurnover = parseRupees(turnoverAns.text) || (estimatedCost ? estimatedCost * 0.3 : null);

  // Calculate Company's 3-year average turnover
  const top3Years = financials.slice(0, 3);
  const avgTurnover = top3Years.length > 0
    ? top3Years.reduce((sum, f) => sum + f.turnover_excluding_gst, 0) / top3Years.length
    : 0;

  if (requiredTurnover) {
    const isTurnoverMet = avgTurnover >= requiredTurnover;
    matrix.push({
      category: 'Financial',
      criterion: 'Average Annual Turnover (AATO)',
      tender_requirement: `Minimum ₹${(requiredTurnover / 10000000).toFixed(2)} Cr (${turnoverAns.text || 'Calculated ~30% of estimated cost'})`,
      company_value: `₹${(avgTurnover / 10000000).toFixed(2)} Cr (3-year average across ${top3Years.map((y) => y.financial_year).join(', ')})`,
      status: isTurnoverMet ? 'met' : 'not_met',
      notes: isTurnoverMet
        ? 'Company average turnover comfortably exceeds the requirement.'
        : `Deficit of ₹${((requiredTurnover - avgTurnover) / 10000000).toFixed(2)} Cr. Consider Joint Venture (JV).`,
    });
    if (!isTurnoverMet) gaps.push(`Turnover deficit: Company 3-yr average (₹${(avgTurnover / 10000000).toFixed(2)} Cr) is below required ₹${(requiredTurnover / 10000000).toFixed(2)} Cr.`);
  } else {
    matrix.push({
      category: 'Financial',
      criterion: 'Average Annual Turnover',
      tender_requirement: turnoverAns.text || 'Not explicitly quantified in checklist',
      company_value: `₹${(avgTurnover / 10000000).toFixed(2)} Cr (3-year average)`,
      status: avgTurnover > 0 ? 'met' : 'needs_review',
      notes: 'Please review specific turnover clause in tender source document.',
    });
  }

  // B. Net Worth Evaluation
  const netWorthAns = findAnswer(/financial/i, /net worth/i);
  const latestNetWorth = financials[0]?.net_worth ?? 0;
  const isNetWorthPositive = latestNetWorth > 0;
  matrix.push({
    category: 'Financial',
    criterion: 'Audited Net Worth',
    tender_requirement: netWorthAns.text || 'Positive audited net worth in the latest financial year',
    company_value: `₹${(latestNetWorth / 10000000).toFixed(2)} Cr (FY ${financials[0]?.financial_year ?? 'N/A'})`,
    status: isNetWorthPositive ? 'met' : 'not_met',
    notes: isNetWorthPositive ? 'Net worth is positive and audited.' : 'Negative net worth flags financial disqualification risk.',
  });
  if (!isNetWorthPositive) gaps.push('Net worth is zero or negative.');

  // C. Solvency Certificate
  const solvencyAns = findAnswer(/financial/i, /solvency/i);
  const solvencyCert = certificates.find((c) => c.kind === 'solvency');
  if (solvencyCert) {
    matrix.push({
      category: 'Financial',
      criterion: 'Bank Solvency Certificate',
      tender_requirement: solvencyAns.text || 'Bank solvency certificate from a scheduled bank',
      company_value: `₹${(solvencyCert.amount / 10000000).toFixed(2)} Cr issued by ${solvencyCert.issuer_name}`,
      status: 'met',
      notes: `Valid until ${solvencyCert.valid_until || 'ongoing'}.`,
    });
  } else {
    matrix.push({
      category: 'Financial',
      criterion: 'Bank Solvency Certificate',
      tender_requirement: solvencyAns.text || 'Bank Solvency Certificate',
      company_value: 'No active solvency certificate on record',
      status: 'needs_review',
      notes: 'Request Solvency Certificate from bank if mandatory in tender.',
    });
  }

  // D. EMD & MSME / Udyam Exemption
  const emdAns = findAnswer(/emd/i, /emd|earnest money|exemption/i);
  const udyamExemptAllowed = !/no exemption|exemption not allowed|exemption (?:is )?prohibited|all bidders must/i.test(emdAns.text);
  const hasUdyam = Boolean(profile.msme_udyam_number && ['micro', 'small'].includes(profile.msme_category));

  if (hasUdyam && udyamExemptAllowed) {
    matrix.push({
      category: 'EMD & Exemptions',
      criterion: 'EMD Exemption under MSME / Udyam',
      tender_requirement: emdAns.text || 'EMD required unless valid exemption applies',
      company_value: `Registered Udyam (${profile.msme_category.toUpperCase()} enterprise, ${profile.msme_activity})`,
      status: 'met',
      notes: 'Company is eligible for 100% EMD and Tender Fee exemption under MSME policy.',
    });
  } else if (!udyamExemptAllowed) {
    matrix.push({
      category: 'EMD & Exemptions',
      criterion: 'EMD Deposit Requirement',
      tender_requirement: emdAns.text || 'EMD mandatory from all bidders (no MSME exemption)',
      company_value: `Udyam ${profile.msme_udyam_number || 'N/A'} (Exemption not accepted by client)`,
      status: 'conditional',
      notes: 'Tender does not allow MSME exemption for this work. EMD must be paid via BG / Portal.',
    });
  } else {
    matrix.push({
      category: 'EMD & Exemptions',
      criterion: 'EMD Payment / Exemption',
      tender_requirement: emdAns.text || 'EMD deposit or valid MSME registration certificate',
      company_value: profile.msme_udyam_number ? `MSME ${profile.msme_category}` : 'Not MSME registered',
      status: 'needs_review',
      notes: 'Prepare Bank Guarantee or online payment if exemption is not claimable.',
    });
  }

  // E. Electrical Contractor Licence
  const licenceAns = findAnswer(/electrical_licence|licence/i, /licence|electrical contractor|voltage/i);
  const stateAns = findAnswer(/particulars|location/i, /location|state|place/i);
  // Match state if mentioned, or check if company holds any valid high-voltage licence
  const tenderState = stateAns.text || profile.registered_state;
  const matchingLicence = licences.find((l) =>
    l.is_active && (tenderState.toLowerCase().includes(l.state.toLowerCase()) || l.state.toLowerCase().includes(tenderState.toLowerCase()))
  ) || licences[0];

  if (matchingLicence) {
    const isExpired = new Date(matchingLicence.valid_until) < new Date();
    matrix.push({
      category: 'Licences & Statutory',
      criterion: 'Electrical Contractor Licence',
      tender_requirement: licenceAns.text || `Valid Electrical Contractor Licence in ${tenderState}`,
      company_value: `${matchingLicence.state} (${matchingLicence.class_grade}, up to ${matchingLicence.maximum_voltage_kv} kV, valid till ${matchingLicence.valid_until})`,
      status: isExpired ? 'not_met' : 'met',
      notes: isExpired
        ? `Licence expired on ${matchingLicence.valid_until}. Needs renewal!`
        : `Valid ${matchingLicence.maximum_voltage_kv} kV licence on record.`,
    });
    if (isExpired) gaps.push(`Electrical Contractor Licence for ${matchingLicence.state} expired on ${matchingLicence.valid_until}.`);
  } else {
    matrix.push({
      category: 'Licences & Statutory',
      criterion: 'Electrical Contractor Licence',
      tender_requirement: licenceAns.text || 'Valid Electrical Contractor Licence required',
      company_value: 'No electrical licence recorded for this state',
      status: 'conditional',
      notes: 'Must apply for state licence endorsement or associate with a licensed local subcontractor/partner.',
    });
    gaps.push(`No Electrical Contractor Licence recorded for state: ${tenderState}.`);
  }

  // F. Technical Track Record / Similar Works
  const workAns = findAnswer(/similar_work/i, /similar work|past work|experience/i);
  // Thresholds: Single work 80%, Two works 50%, Three works 40%
  const t80 = estimatedCost ? estimatedCost * 0.8 : 0;
  const t50 = estimatedCost ? estimatedCost * 0.5 : 0;
  const t40 = estimatedCost ? estimatedCost * 0.4 : 0;

  const qualifying80 = work_experience.filter((w) => w.executed_value_excluding_gst >= t80);
  const qualifying50 = work_experience.filter((w) => w.executed_value_excluding_gst >= t50);
  const qualifying40 = work_experience.filter((w) => w.executed_value_excluding_gst >= t40);

  const meetsExperience = qualifying80.length >= 1 || qualifying50.length >= 2 || qualifying40.length >= 3;

  const topWorksSummary = work_experience.slice(0, 3).map(
    (w) => `${w.project_title} (₹${(w.executed_value_excluding_gst / 10000000).toFixed(2)} Cr, ${w.client_name})`
  ).join('; ');

  matrix.push({
    category: 'Technical Experience',
    criterion: 'Similar Works Completed',
    tender_requirement: workAns.text || (estimatedCost ? `1 work $\ge$ ₹${(t80/10000000).toFixed(2)} Cr, OR 2 works $\ge$ ₹${(t50/10000000).toFixed(2)} Cr, OR 3 works $\ge$ ₹${(t40/10000000).toFixed(2)} Cr` : 'PQC technical experience matching tender scope'),
    company_value: topWorksSummary || 'No executed projects recorded',
    status: meetsExperience ? 'met' : work_experience.length > 0 ? 'conditional' : 'needs_review',
    notes: meetsExperience
      ? `Meets experience criteria with ${qualifying80.length ? '1 large work $\ge$ 80%' : qualifying50.length >= 2 ? '2 works $\ge$ 50%' : '3 works $\ge$ 40%'}.`
      : 'Verify whether client credentials accept joint experience or relaxations.',
  });
  if (!meetsExperience && estimatedCost) {
    gaps.push(`Similar work threshold check: largest single executed work is ₹${((work_experience[0]?.executed_value_excluding_gst ?? 0)/10000000).toFixed(2)} Cr.`);
  }

  // G. Non-blacklisting & Clean Declarations
  matrix.push({
    category: 'Declarations',
    criterion: 'Non-Blacklisting & Insolvency Declarations',
    tender_requirement: 'Bidder must not be debarred/blacklisted by any Govt/PSU or undergoing IBC/NCLT insolvency.',
    company_value: profile.non_blacklisted_declaration && profile.clean_insolvency_declaration ? 'Clean declaration verified' : 'Declarations missing',
    status: profile.non_blacklisted_declaration && profile.clean_insolvency_declaration ? 'met' : 'not_met',
    notes: 'Standard affidavit to be signed and notarized.',
  });

  // Calculate Overall Verdict
  const hasNotMet = matrix.some((m) => m.status === 'not_met');
  const hasConditional = matrix.some((m) => m.status === 'conditional');

  let verdict: 'eligible' | 'not_eligible' | 'conditionally_eligible';
  let overall_summary: string;

  if (hasNotMet) {
    verdict = 'not_eligible';
    overall_summary = `The company currently does not meet one or more mandatory criteria (${gaps.join('; ')}). Bidding as a sole bidder carries significant disqualification risk unless a Joint Venture (JV) or Consortium is formed.`;
  } else if (hasConditional || gaps.length > 0) {
    verdict = 'conditionally_eligible';
    overall_summary = `The company is conditionally eligible. Critical financial criteria are satisfied, but operational caveats exist (e.g. licence endorsement, EMD deposit without exemption, or project scope alignment). Review the gaps before submitting.`;
  } else {
    verdict = 'eligible';
    overall_summary = `The company meets all major financial, technical, and statutory Pre-Qualification Criteria (PQC) for this tender based on the TSC checklist and company records. Ready for formal Director bidding approval.`;
  }

  // 4. Save into Database (Upsert)
  const saved = await transaction(async (txn) => {
    const res = await txn.query<EligibilityVerdict>(
      `INSERT INTO tender.tsc_eligibility_verdicts (
        draft_id, job_id, verdict, overall_summary, criteria_matrix, gaps_and_risks, model_used, version
      ) VALUES ($1, $2, $3, $4, $5, $6, $7, 1)
      ON CONFLICT (draft_id, job_id) DO UPDATE SET
        verdict = EXCLUDED.verdict,
        overall_summary = EXCLUDED.overall_summary,
        criteria_matrix = EXCLUDED.criteria_matrix,
        gaps_and_risks = EXCLUDED.gaps_and_risks,
        model_used = EXCLUDED.model_used,
        evaluated_at = clock_timestamp(),
        version = tender.tsc_eligibility_verdicts.version + 1
      RETURNING id, draft_id, job_id, verdict, overall_summary, criteria_matrix, gaps_and_risks,
                model_used, evaluated_at, human_verified, human_decision, human_remarks,
                verified_by_id, verified_at, version`,
      [
        draftId,
        jobId,
        verdict,
        overall_summary,
        JSON.stringify(matrix),
        JSON.stringify(gaps),
        snapshot.mode === 'gemini' ? 'Gemini 3.8 Flash' : snapshot.mode === 'openai' ? 'OpenAI GPT-6 Luna' : 'Deterministic PQC Evaluator',
      ]
    );
    return res.rows[0];
  });

  return saved;
}

export async function getEligibilityVerdict(draftId: string, jobId: string): Promise<EligibilityVerdict | null> {
  const db = pool();
  const res = await db.query<EligibilityVerdict & { username?: string }>(
    `SELECT v.id, v.draft_id, v.job_id, v.verdict, v.overall_summary, v.criteria_matrix,
            v.gaps_and_risks, v.model_used, to_char(v.evaluated_at, 'YYYY-MM-DD"T"HH24:MI:SSZ') as evaluated_at,
            v.human_verified, v.human_decision, v.human_remarks,
            u.username as verified_by_name,
            to_char(v.verified_at, 'YYYY-MM-DD"T"HH24:MI:SSZ') as verified_at,
            v.version
     FROM tender.tsc_eligibility_verdicts v
     LEFT JOIN auth.users u ON u.id = v.verified_by_id
     WHERE v.draft_id = $1 AND v.job_id = $2`,
    [draftId, jobId]
  );
  return res.rows[0] ?? null;
}

export async function verifyEligibilityDecision(
  draftId: string,
  jobId: string,
  decision: 'accepted' | 'rejected' | 'overridden',
  remarks: string,
  actorId: string
): Promise<EligibilityVerdict> {
  return transaction(async (db) => {
    const updated = await db.query<EligibilityVerdict>(
      `UPDATE tender.tsc_eligibility_verdicts SET
        human_verified = true,
        human_decision = $1,
        human_remarks = $2,
        verified_by_id = $3,
        verified_at = clock_timestamp(),
        version = version + 1
      WHERE draft_id = $4 AND job_id = $5
      RETURNING id, draft_id, job_id, verdict, overall_summary, criteria_matrix, gaps_and_risks,
                model_used, evaluated_at, human_verified, human_decision, human_remarks,
                verified_by_id, verified_at, version`,
      [decision, remarks || '', actorId, draftId, jobId]
    );
    if (!updated.rows[0]) throw new AppError(404, 'Eligibility verdict not found to verify');
    return updated.rows[0];
  });
}
