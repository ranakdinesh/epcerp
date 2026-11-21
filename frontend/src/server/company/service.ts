import 'server-only';
import { pool, transaction } from '../db';
import { AppError } from '../errors';
import type {
  CompanyProfile,
  CompanyFinancialYear,
  CompanyCertificate,
  CompanyLicence,
  CompanyWorkExperience,
  FullCompanySetup,
} from './types';

const defaultProfile: CompanyProfile = {
  id: '',
  legal_name: '',
  trade_name: '',
  cin: '',
  pan: '',
  gstin: '',
  incorporation_date: null,
  registered_address: '',
  registered_state: '',
  registered_city: '',
  registered_pincode: '',
  epfo_registration: '',
  esic_registration: '',
  msme_udyam_number: '',
  msme_category: '',
  msme_activity: '',
  startup_recognition_number: '',
  iso_9001: false,
  iso_14001: false,
  iso_45001: false,
  non_blacklisted_declaration: true,
  clean_insolvency_declaration: true,
  version: 0,
  updated_at: new Date().toISOString(),
};

export async function getCompanyProfile(): Promise<FullCompanySetup> {
  const db = pool();
  const profileRes = await db.query<CompanyProfile>(
    `SELECT id, legal_name, trade_name, cin, pan, gstin, 
            to_char(incorporation_date, 'YYYY-MM-DD') as incorporation_date,
            registered_address, registered_state, registered_city, registered_pincode,
            epfo_registration, esic_registration, msme_udyam_number, msme_category,
            msme_activity, startup_recognition_number, iso_9001, iso_14001, iso_45001,
            non_blacklisted_declaration, clean_insolvency_declaration, version, updated_at
     FROM public.company_profile WHERE singleton = true LIMIT 1`
  );

  const profile = profileRes.rows[0] ?? defaultProfile;

  const financialsRes = await db.query<CompanyFinancialYear>(
    `SELECT id, financial_year, turnover_excluding_gst::float as turnover_excluding_gst,
            net_worth::float as net_worth, profit_after_tax::float as profit_after_tax,
            is_audited, version, updated_at
     FROM public.company_financial_years
     ORDER BY financial_year DESC`
  );

  const certificatesRes = await db.query<CompanyCertificate>(
    `SELECT id, kind, certificate_number, issuer_name, amount::float as amount,
            to_char(issued_on, 'YYYY-MM-DD') as issued_on,
            to_char(valid_until, 'YYYY-MM-DD') as valid_until,
            is_active, version, updated_at
     FROM public.company_certificates
     WHERE is_active = true
     ORDER BY updated_at DESC`
  );

  const licencesRes = await db.query<CompanyLicence>(
    `SELECT id, licence_number, state, class_grade, maximum_voltage_kv::float as maximum_voltage_kv,
            to_char(issued_on, 'YYYY-MM-DD') as issued_on,
            to_char(valid_until, 'YYYY-MM-DD') as valid_until,
            is_active, version, updated_at
     FROM public.company_licences
     WHERE is_active = true
     ORDER BY state ASC, maximum_voltage_kv DESC`
  );

  const experienceRes = await db.query<CompanyWorkExperience>(
    `SELECT id, project_title, client_name, client_type, scope_description,
            executed_value_excluding_gst::float as executed_value_excluding_gst,
            state, voltage_kv::float as voltage_kv, capacity_description,
            company_share_percent::float as company_share_percent,
            to_char(completed_on, 'YYYY-MM-DD') as completed_on,
            completion_certificate_ref, is_active, version, updated_at
     FROM public.company_work_experience
     WHERE is_active = true
     ORDER BY completed_on DESC`
  );

  return {
    profile,
    financials: financialsRes.rows,
    certificates: certificatesRes.rows,
    licences: licencesRes.rows,
    work_experience: experienceRes.rows,
  };
}

export async function saveCompanyProfile(
  data: Partial<CompanyProfile> & { expected_version?: number },
  actorId?: string
): Promise<CompanyProfile> {
  return transaction(async (db) => {
    const existing = (
      await db.query<{ id: string; version: number }>(
        `SELECT id, version FROM public.company_profile WHERE singleton = true FOR UPDATE`
      )
    ).rows[0];

    if (!existing) {
      const inserted = await db.query<CompanyProfile>(
        `INSERT INTO public.company_profile (
          legal_name, trade_name, cin, pan, gstin, incorporation_date,
          registered_address, registered_state, registered_city, registered_pincode,
          epfo_registration, esic_registration, msme_udyam_number, msme_category,
          msme_activity, startup_recognition_number, iso_9001, iso_14001, iso_45001,
          non_blacklisted_declaration, clean_insolvency_declaration, version, updated_by_id
        ) VALUES (
          $1, $2, $3, $4, $5, $6,
          $7, $8, $9, $10,
          $11, $12, $13, $14,
          $15, $16, $17, $18, $19,
          $20, $21, 1, $22
        ) RETURNING id, legal_name, trade_name, cin, pan, gstin,
                    to_char(incorporation_date, 'YYYY-MM-DD') as incorporation_date,
                    registered_address, registered_state, registered_city, registered_pincode,
                    epfo_registration, esic_registration, msme_udyam_number, msme_category,
                    msme_activity, startup_recognition_number, iso_9001, iso_14001, iso_45001,
                    non_blacklisted_declaration, clean_insolvency_declaration, version, updated_at`,
        [
          data.legal_name || 'My Company',
          data.trade_name || '',
          data.cin || '',
          data.pan || '',
          data.gstin || '',
          data.incorporation_date || null,
          data.registered_address || '',
          data.registered_state || '',
          data.registered_city || '',
          data.registered_pincode || '',
          data.epfo_registration || '',
          data.esic_registration || '',
          data.msme_udyam_number || '',
          data.msme_category || '',
          data.msme_activity || '',
          data.startup_recognition_number || '',
          data.iso_9001 ?? false,
          data.iso_14001 ?? false,
          data.iso_45001 ?? false,
          data.non_blacklisted_declaration ?? true,
          data.clean_insolvency_declaration ?? true,
          actorId || null,
        ]
      );
      return inserted.rows[0];
    }

    if (data.expected_version !== undefined && existing.version !== data.expected_version) {
      throw new AppError(409, 'Company profile has been updated by another user. Please refresh and retry.');
    }

    const updated = await db.query<CompanyProfile>(
      `UPDATE public.company_profile SET
        legal_name = COALESCE($1, legal_name),
        trade_name = COALESCE($2, trade_name),
        cin = COALESCE($3, cin),
        pan = COALESCE($4, pan),
        gstin = COALESCE($5, gstin),
        incorporation_date = COALESCE($6, incorporation_date),
        registered_address = COALESCE($7, registered_address),
        registered_state = COALESCE($8, registered_state),
        registered_city = COALESCE($9, registered_city),
        registered_pincode = COALESCE($10, registered_pincode),
        epfo_registration = COALESCE($11, epfo_registration),
        esic_registration = COALESCE($12, esic_registration),
        msme_udyam_number = COALESCE($13, msme_udyam_number),
        msme_category = COALESCE($14, msme_category),
        msme_activity = COALESCE($15, msme_activity),
        startup_recognition_number = COALESCE($16, startup_recognition_number),
        iso_9001 = COALESCE($17, iso_9001),
        iso_14001 = COALESCE($18, iso_14001),
        iso_45001 = COALESCE($19, iso_45001),
        non_blacklisted_declaration = COALESCE($20, non_blacklisted_declaration),
        clean_insolvency_declaration = COALESCE($21, clean_insolvency_declaration),
        version = version + 1,
        updated_at = clock_timestamp(),
        updated_by_id = $22
      WHERE singleton = true
      RETURNING id, legal_name, trade_name, cin, pan, gstin,
                to_char(incorporation_date, 'YYYY-MM-DD') as incorporation_date,
                registered_address, registered_state, registered_city, registered_pincode,
                epfo_registration, esic_registration, msme_udyam_number, msme_category,
                msme_activity, startup_recognition_number, iso_9001, iso_14001, iso_45001,
                non_blacklisted_declaration, clean_insolvency_declaration, version, updated_at`,
      [
        data.legal_name,
        data.trade_name,
        data.cin,
        data.pan,
        data.gstin,
        data.incorporation_date || null,
        data.registered_address,
        data.registered_state,
        data.registered_city,
        data.registered_pincode,
        data.epfo_registration,
        data.esic_registration,
        data.msme_udyam_number,
        data.msme_category,
        data.msme_activity,
        data.startup_recognition_number,
        data.iso_9001,
        data.iso_14001,
        data.iso_45001,
        data.non_blacklisted_declaration,
        data.clean_insolvency_declaration,
        actorId || null,
      ]
    );
    return updated.rows[0];
  });
}

export async function saveFinancialYear(
  data: {
    id?: string;
    financial_year: string;
    turnover_excluding_gst: number;
    net_worth: number;
    profit_after_tax: number;
    is_audited?: boolean;
    expected_version?: number;
  },
  actorId?: string
): Promise<CompanyFinancialYear> {
  return transaction(async (db) => {
    if (data.id) {
      const existing = (
        await db.query<{ id: string; version: number }>(
          `SELECT id, version FROM public.company_financial_years WHERE id = $1 FOR UPDATE`,
          [data.id]
        )
      ).rows[0];
      if (!existing) throw new AppError(404, 'Financial year record not found');
      if (data.expected_version !== undefined && existing.version !== data.expected_version) {
        throw new AppError(409, 'Financial record was changed. Refresh and retry.');
      }
      const updated = await db.query<CompanyFinancialYear>(
        `UPDATE public.company_financial_years SET
          financial_year = $1,
          turnover_excluding_gst = $2,
          net_worth = $3,
          profit_after_tax = $4,
          is_audited = COALESCE($5, true),
          version = version + 1,
          updated_at = clock_timestamp(),
          updated_by_id = $6
        WHERE id = $7
        RETURNING id, financial_year, turnover_excluding_gst::float as turnover_excluding_gst,
                  net_worth::float as net_worth, profit_after_tax::float as profit_after_tax,
                  is_audited, version, updated_at`,
        [
          data.financial_year,
          data.turnover_excluding_gst,
          data.net_worth,
          data.profit_after_tax,
          data.is_audited,
          actorId || null,
          data.id,
        ]
      );
      return updated.rows[0];
    } else {
      const inserted = await db.query<CompanyFinancialYear>(
        `INSERT INTO public.company_financial_years (
          financial_year, turnover_excluding_gst, net_worth, profit_after_tax, is_audited, version, updated_by_id
        ) VALUES ($1, $2, $3, $4, COALESCE($5, true), 1, $6)
        ON CONFLICT (financial_year) DO UPDATE SET
          turnover_excluding_gst = EXCLUDED.turnover_excluding_gst,
          net_worth = EXCLUDED.net_worth,
          profit_after_tax = EXCLUDED.profit_after_tax,
          is_audited = EXCLUDED.is_audited,
          version = public.company_financial_years.version + 1,
          updated_at = clock_timestamp(),
          updated_by_id = EXCLUDED.updated_by_id
        RETURNING id, financial_year, turnover_excluding_gst::float as turnover_excluding_gst,
                  net_worth::float as net_worth, profit_after_tax::float as profit_after_tax,
                  is_audited, version, updated_at`,
        [
          data.financial_year,
          data.turnover_excluding_gst,
          data.net_worth,
          data.profit_after_tax,
          data.is_audited,
          actorId || null,
        ]
      );
      return inserted.rows[0];
    }
  });
}

export async function deleteFinancialYear(id: string): Promise<void> {
  const db = pool();
  await db.query(`DELETE FROM public.company_financial_years WHERE id = $1`, [id]);
}

export async function saveLicence(
  data: {
    id?: string;
    licence_number: string;
    state: string;
    class_grade?: string;
    maximum_voltage_kv: number;
    issued_on?: string | null;
    valid_until: string;
    expected_version?: number;
  },
  actorId?: string
): Promise<CompanyLicence> {
  return transaction(async (db) => {
    if (data.id) {
      const existing = (
        await db.query<{ id: string; version: number }>(
          `SELECT id, version FROM public.company_licences WHERE id = $1 FOR UPDATE`,
          [data.id]
        )
      ).rows[0];
      if (!existing) throw new AppError(404, 'Licence not found');
      if (data.expected_version !== undefined && existing.version !== data.expected_version) {
        throw new AppError(409, 'Licence was updated by another user. Refresh and retry.');
      }
      const updated = await db.query<CompanyLicence>(
        `UPDATE public.company_licences SET
          licence_number = $1,
          state = $2,
          class_grade = COALESCE($3, 'Class A'),
          maximum_voltage_kv = $4,
          issued_on = $5,
          valid_until = $6,
          version = version + 1,
          updated_at = clock_timestamp(),
          updated_by_id = $7
        WHERE id = $8
        RETURNING id, licence_number, state, class_grade, maximum_voltage_kv::float as maximum_voltage_kv,
                  to_char(issued_on, 'YYYY-MM-DD') as issued_on,
                  to_char(valid_until, 'YYYY-MM-DD') as valid_until,
                  is_active, version, updated_at`,
        [
          data.licence_number,
          data.state,
          data.class_grade,
          data.maximum_voltage_kv,
          data.issued_on || null,
          data.valid_until,
          actorId || null,
          data.id,
        ]
      );
      return updated.rows[0];
    } else {
      const inserted = await db.query<CompanyLicence>(
        `INSERT INTO public.company_licences (
          licence_number, state, class_grade, maximum_voltage_kv, issued_on, valid_until, version, updated_by_id
        ) VALUES ($1, $2, COALESCE($3, 'Class A'), $4, $5, $6, 1, $7)
        RETURNING id, licence_number, state, class_grade, maximum_voltage_kv::float as maximum_voltage_kv,
                  to_char(issued_on, 'YYYY-MM-DD') as issued_on,
                  to_char(valid_until, 'YYYY-MM-DD') as valid_until,
                  is_active, version, updated_at`,
        [
          data.licence_number,
          data.state,
          data.class_grade,
          data.maximum_voltage_kv,
          data.issued_on || null,
          data.valid_until,
          actorId || null,
        ]
      );
      return inserted.rows[0];
    }
  });
}

export async function deleteLicence(id: string): Promise<void> {
  const db = pool();
  await db.query(`UPDATE public.company_licences SET is_active = false, updated_at = clock_timestamp() WHERE id = $1`, [id]);
}

export async function saveWorkExperience(
  data: {
    id?: string;
    project_title: string;
    client_name: string;
    client_type: 'central_govt' | 'state_govt' | 'psu' | 'public_ltd' | 'private';
    scope_description?: string;
    executed_value_excluding_gst: number;
    state?: string;
    voltage_kv?: number | null;
    capacity_description?: string;
    company_share_percent?: number;
    completed_on: string;
    completion_certificate_ref?: string;
    expected_version?: number;
  },
  actorId?: string
): Promise<CompanyWorkExperience> {
  return transaction(async (db) => {
    if (data.id) {
      const existing = (
        await db.query<{ id: string; version: number }>(
          `SELECT id, version FROM public.company_work_experience WHERE id = $1 FOR UPDATE`,
          [data.id]
        )
      ).rows[0];
      if (!existing) throw new AppError(404, 'Experience record not found');
      if (data.expected_version !== undefined && existing.version !== data.expected_version) {
        throw new AppError(409, 'Record was updated by another user. Refresh and retry.');
      }
      const updated = await db.query<CompanyWorkExperience>(
        `UPDATE public.company_work_experience SET
          project_title = $1,
          client_name = $2,
          client_type = $3,
          scope_description = COALESCE($4, ''),
          executed_value_excluding_gst = $5,
          state = COALESCE($6, ''),
          voltage_kv = $7,
          capacity_description = COALESCE($8, ''),
          company_share_percent = COALESCE($9, 100.00),
          completed_on = $10,
          completion_certificate_ref = COALESCE($11, ''),
          version = version + 1,
          updated_at = clock_timestamp(),
          updated_by_id = $12
        WHERE id = $13
        RETURNING id, project_title, client_name, client_type, scope_description,
                  executed_value_excluding_gst::float as executed_value_excluding_gst,
                  state, voltage_kv::float as voltage_kv, capacity_description,
                  company_share_percent::float as company_share_percent,
                  to_char(completed_on, 'YYYY-MM-DD') as completed_on,
                  completion_certificate_ref, is_active, version, updated_at`,
        [
          data.project_title,
          data.client_name,
          data.client_type,
          data.scope_description,
          data.executed_value_excluding_gst,
          data.state,
          data.voltage_kv || null,
          data.capacity_description,
          data.company_share_percent,
          data.completed_on,
          data.completion_certificate_ref,
          actorId || null,
          data.id,
        ]
      );
      return updated.rows[0];
    } else {
      const inserted = await db.query<CompanyWorkExperience>(
        `INSERT INTO public.company_work_experience (
          project_title, client_name, client_type, scope_description,
          executed_value_excluding_gst, state, voltage_kv, capacity_description,
          company_share_percent, completed_on, completion_certificate_ref,
          version, updated_by_id
        ) VALUES ($1, $2, $3, COALESCE($4, ''), $5, COALESCE($6, ''), $7, COALESCE($8, ''), COALESCE($9, 100.00), $10, COALESCE($11, ''), 1, $12)
        RETURNING id, project_title, client_name, client_type, scope_description,
                  executed_value_excluding_gst::float as executed_value_excluding_gst,
                  state, voltage_kv::float as voltage_kv, capacity_description,
                  company_share_percent::float as company_share_percent,
                  to_char(completed_on, 'YYYY-MM-DD') as completed_on,
                  completion_certificate_ref, is_active, version, updated_at`,
        [
          data.project_title,
          data.client_name,
          data.client_type,
          data.scope_description,
          data.executed_value_excluding_gst,
          data.state,
          data.voltage_kv || null,
          data.capacity_description,
          data.company_share_percent,
          data.completed_on,
          data.completion_certificate_ref,
          actorId || null,
        ]
      );
      return inserted.rows[0];
    }
  });
}

export async function deleteWorkExperience(id: string): Promise<void> {
  const db = pool();
  await db.query(`UPDATE public.company_work_experience SET is_active = false, updated_at = clock_timestamp() WHERE id = $1`, [id]);
}

export async function saveCertificate(
  data: {
    id?: string;
    kind: 'solvency' | 'working_capital' | 'bank_guarantee_limit' | 'iso_certificate';
    certificate_number: string;
    issuer_name: string;
    amount: number;
    issued_on?: string | null;
    valid_until?: string | null;
    expected_version?: number;
  },
  actorId?: string
): Promise<CompanyCertificate> {
  return transaction(async (db) => {
    if (data.id) {
      const existing = (
        await db.query<{ id: string; version: number }>(
          `SELECT id, version FROM public.company_certificates WHERE id = $1 FOR UPDATE`,
          [data.id]
        )
      ).rows[0];
      if (!existing) throw new AppError(404, 'Certificate not found');
      if (data.expected_version !== undefined && existing.version !== data.expected_version) {
        throw new AppError(409, 'Certificate record was modified. Refresh and retry.');
      }
      const updated = await db.query<CompanyCertificate>(
        `UPDATE public.company_certificates SET
          kind = $1,
          certificate_number = $2,
          issuer_name = $3,
          amount = $4,
          issued_on = $5,
          valid_until = $6,
          version = version + 1,
          updated_at = clock_timestamp(),
          updated_by_id = $7
        WHERE id = $8
        RETURNING id, kind, certificate_number, issuer_name, amount::float as amount,
                  to_char(issued_on, 'YYYY-MM-DD') as issued_on,
                  to_char(valid_until, 'YYYY-MM-DD') as valid_until,
                  is_active, version, updated_at`,
        [
          data.kind,
          data.certificate_number,
          data.issuer_name,
          data.amount,
          data.issued_on || null,
          data.valid_until || null,
          actorId || null,
          data.id,
        ]
      );
      return updated.rows[0];
    } else {
      const inserted = await db.query<CompanyCertificate>(
        `INSERT INTO public.company_certificates (
          kind, certificate_number, issuer_name, amount, issued_on, valid_until, version, updated_by_id
        ) VALUES ($1, $2, $3, $4, $5, $6, 1, $7)
        RETURNING id, kind, certificate_number, issuer_name, amount::float as amount,
                  to_char(issued_on, 'YYYY-MM-DD') as issued_on,
                  to_char(valid_until, 'YYYY-MM-DD') as valid_until,
                  is_active, version, updated_at`,
        [
          data.kind,
          data.certificate_number,
          data.issuer_name,
          data.amount,
          data.issued_on || null,
          data.valid_until || null,
          actorId || null,
        ]
      );
      return inserted.rows[0];
    }
  });
}

export async function deleteCertificate(id: string): Promise<void> {
  const db = pool();
  await db.query(`UPDATE public.company_certificates SET is_active = false, updated_at = clock_timestamp() WHERE id = $1`, [id]);
}
