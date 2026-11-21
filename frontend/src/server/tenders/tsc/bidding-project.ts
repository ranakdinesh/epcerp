import 'server-only';
import { assignmentIdentities, assignmentIdentity } from '../../auth/access';
import { transaction, type DB } from '../../db';
import { AppError } from '../../errors';
import { projectReady } from '../../purchasing/approvals/service';
import { tenderAwardPending } from '../../purchasing/core/tender-award';
import { initializeProjectPurchase } from '../../purchasing/project-registration';
import * as projects from '../../projects/repository';
import { capture, id, projectName } from '../../projects/validation';
import { validate } from '../../projects/schedule';
import { authorizeTender } from '../documents';
import { lower } from '../intake/receipts';
import { decrypt } from './crypto';
import type { Answer, JobRow, Snapshot } from './types';

type Scope = Awaited<ReturnType<typeof authorizeTender>>;
type Principal = Scope['actor'];
type Client = { id: string; name: string; legal_name: string; code: string };
type Estimate = { estimated_value: number | null; due_date: string | null; approval_notes: string };

const creationPermissions = ['projects.master.read', 'projects.master.write', 'projects.portfolio.write', 'projects.purchase_assignments.write'];
const canCreate = (actor: Principal) => creationPermissions.every(permission => actor.allows(permission));
const canManageProject = (user: NonNullable<Awaited<ReturnType<typeof assignmentIdentity>>>) =>
    user.active && user.ready && user.allows('projects.master.read') && user.allows('projects.master.write');

async function clients(db: DB) {
    return (await db.query<Client>(
        'SELECT id, display_name AS name, legal_name, client_code AS code FROM clients WHERE is_active ORDER BY display_name, id'
    )).rows;
}

async function defaults(db: DB, draft: Scope['draft'], options: Client[]) {
    const job = (await db.query<JobRow>(
        `SELECT w.* FROM tender.tsc_workspaces w
         LEFT JOIN tender.tsc_eligibility_verdicts v ON v.job_id=w.id AND v.draft_id=w.draft_id AND v.human_verified
         WHERE w.draft_id=$1 AND w.draft_version=$2 AND w.state='ready'
         ORDER BY v.verified_at DESC NULLS LAST, w.created_at DESC, w.id LIMIT 1`,
        [draft.id, draft.version]
    )).rows[0];
    const snapshot = job && decrypt<Snapshot>(job.snapshot_cipher, job.snapshot_hash);
    const answers = job ? decrypt<Answer[]>(job.answers_cipher, job.answers_hash) : [];
    function answer(questionId: string, label: RegExp) {
        const question = snapshot?.questions.find(q => q.id === questionId) ?? snapshot?.questions.find(q => label.test(q.question.trim()));
        return answers.find(a => a.question_id === question?.id && a.status === 'answered' &&
            (a.accepted || a.origin === 'manual' || a.origin === 'upload'))?.answer.trim() ?? '';
    }
    const department = answer('particulars.q01', /^(?:name of (?:the )?)?(?:client|department|tender inviting authority)\s*:?$/i);
    const matches = department ? options.filter(client => [client.name, client.legal_name].some(name => lower(name.trim()) === lower(department))) : [];
    const amount = answer('particulars.q04', /^estimated tender value\s*:?$/i).replace(/^(?:₹|INR|Rs\.?)\s*/i, '').replaceAll(',', '');
    const due = answer('particulars.q09', /^tender due date(?: & time)?\s*:?$/i);
    const businessType = answer('', /^(?:project|business) type\s*:?$/i).toLowerCase();
    return {
        name: (answer('particulars.q02', /^name of (?:the )?(?:work|project)\s*:?$/i) || draft.name).slice(0, 255),
        client_id: matches.length === 1 ? matches[0].id : '',
        department,
        site_name: answer('particulars.q03', /^(?:location where the work will be executed|(?:project|site|work) location)\s*:?$/i).slice(0, 255),
        site_city: answer('', /^(?:project |site )?city\s*:?$/i).slice(0, 128),
        site_state: answer('', /^(?:project |site )?state\s*:?$/i).slice(0, 128),
        project_type: ['electrical', 'solar', 'ev', 'mixed'].includes(businessType) ? businessType : '',
        completion_requirement: answer('particulars.q07', /^completion period\s*:?$/i).slice(0, 500),
        scope_summary: answer('', /^scope of (?:the )?work\s*:?$/i).slice(0, 10000),
        estimated_value: /^(?:0|[1-9]\d{0,13})(?:\.\d{1,2})?$/.test(amount) ? amount : '',
        due_date: /^\d{4}-\d{2}-\d{2}$/.test(due) && !Number.isNaN(Date.parse(due)) && new Date(due).toISOString().slice(0, 10) === due ? due : '',
    };
}

export async function biddingProjectId(db: DB, draft: string) {
    return (await db.query<{ project_id: string | null }>(
        `SELECT COALESCE(
          (SELECT project_id FROM tender.bidding_authorizations WHERE draft_id=$1),
          (SELECT project_id FROM tender.bid_costings WHERE draft_id=$1)
        ) AS project_id`, [draft]
    )).rows[0].project_id;
}

function purchaseAccess(actor: Principal, project: projects.Project) {
    const hasView = actor.allows('projects.purchase_workflow.view') && actor.allows('projects.purchase_workflow.read');
    const manages = actor.id === project.project_manager_id || actor.allows('projects.purchase_portfolio.write');
    return {
        can_open_purchase: hasView && (manages || actor.allows('projects.purchase_portfolio.read') || actor.id === project.procurement_executive_id),
        can_start_purchase: hasView && manages && actor.allows('projects.purchase_assignments.write') && project.is_active &&
            !['closed', 'on_hold'].includes(project.lifecycle_status),
    };
}

function summary(actor: Principal, project: projects.Project) {
    return {
        id: project.id, number: project.project_number, name: project.name, purchase_enabled: project.purchase_enabled,
        can_open_project: actor.allows('projects.master.view') && actor.allows('projects.master.read') &&
            (actor.id === project.project_manager_id || ['projects.portfolio.read', 'projects.portfolio.write', 'projects.manager_assignments.write'].some(permission => actor.allows(permission))),
        ...purchaseAccess(actor, project),
    };
}

export function biddingProject(token: string, draftId: string) {
    return transaction(async db => {
        const { actor, draft } = await authorizeTender(db, token, draftId);
        const projectId = await biddingProjectId(db, draft.id);
        const project = projectId ? await projects.owner(db, await projects.get(db, projectId)) : null;
        if (project || !canCreate(actor)) return { project: project ? { ...summary(actor, project), quotations_approved: project.purchase_enabled && await projectReady(db, project.id), tender_award_pending: await tenderAwardPending(db, project.id) } : null, setup: null };
        const options = await clients(db);
        const managers = (await assignmentIdentities(db, [])).filter(canManageProject).map(user => ({ id: user.id, name: user.name }));
        return { project: null, setup: { defaults: await defaults(db, draft, options), clients: options.map(({ id, name, code }) => ({ id, name, code })), managers, actor_id: actor.id } };
    });
}

export async function ensureBiddingProject(db: DB, { actor, draft }: Scope, currentProjectId: string | null, managerId: string | null, estimate: Estimate, fields: Record<string, unknown>) {
    if (currentProjectId) return summary(actor, await projects.owner(db, await projects.get(db, currentProjectId)));
    if (!canCreate(actor)) throw new AppError(403, 'Project creation permission is required to authorize this tender for bidding');
    await db.query('SELECT pg_advisory_xact_lock_shared(70623124)');
    const suggested = await defaults(db, draft, await clients(db));
    const name = projectName(fields.name ?? suggested.name);
    const client = id(fields.client_id || suggested.client_id, 'client_id');
    const registered = (await db.query('SELECT is_active FROM clients WHERE id=$1 FOR SHARE', [client])).rows[0];
    if (!registered?.is_active) throw new AppError(422, 'Select an active registered client');

    let manager = fields.project_manager_id ? id(fields.project_manager_id, 'project_manager_id') : managerId || actor.id;
    let managerUser = await assignmentIdentity(db, manager);
    if (!fields.project_manager_id && (!managerUser || !canManageProject(managerUser)) && manager !== actor.id) {
        manager = actor.id;
        managerUser = await assignmentIdentity(db, manager);
    }
    if (!managerUser || !canManageProject(managerUser)) throw new AppError(422, 'Select an active project manager with project read and write access');

    const remarks = [
        'Created from Tender FMS bidding authorization.',
        estimate.estimated_value != null ? `Estimated tender value (INR): ${estimate.estimated_value}` : '',
        estimate.due_date ? `Bid submission due: ${estimate.due_date}` : '',
        suggested.completion_requirement ? `Tender completion requirement: ${suggested.completion_requirement}` : '',
        estimate.approval_notes,
    ].filter(Boolean).join('\n');
    const details = validate(capture({
        site_name: fields.site_name ?? suggested.site_name,
        site_city: fields.site_city ?? suggested.site_city,
        site_state: fields.site_state ?? suggested.site_state,
        project_type: fields.project_type ?? suggested.project_type,
        scope_summary: fields.scope_summary ?? suggested.scope_summary,
        remarks, purchase_enabled: false,
    }), 'draft', name);
    try {
        const saved = await projects.insert(db, {
            name, nameKey: lower(name), client, manager, actor: actor.id, capture: details,
            reason: 'Initial project manager assignment at bidding authorization',
        });
        return summary(actor, saved);
    } catch (error) {
        if ((error as { constraint?: string }).constraint === 'projects_normalized_name_key') {
            throw new AppError(422, 'A project with this name already exists. Enter a distinct project name.');
        }
        throw error;
    }
}

export function startBiddingPurchase(token: string, draftId: string) {
    return transaction(async db => {
        const { actor } = await authorizeTender(db, token, draftId, 'write');
        const approval = (await db.query<{ project_id: string | null; decision: string }>(
            'SELECT project_id, decision FROM tender.bidding_authorizations WHERE draft_id=$1', [draftId]
        )).rows[0];
        if (!approval?.project_id || approval.decision !== 'authorized') {
            throw new AppError(409, 'Authorize the tender for bidding and create its project first');
        }
        await db.query('SELECT pg_advisory_xact_lock_shared(70623124)');
        const before = await projects.owner(db, await projects.get(db, approval.project_id, true));
        if (!purchaseAccess(actor, before).can_start_purchase) throw new AppError(403, 'Purchase FMS setup permission is required for this project');
        const client = (await db.query('SELECT is_active FROM clients WHERE id=$1 FOR SHARE', [before.client_id])).rows[0];
        if (!client?.is_active) throw new AppError(422, 'Select an active registered client on the project before starting Purchase FMS');
        const workspace = (await db.query('SELECT is_active FROM purchase_fms_workspaces WHERE project_id=$1 FOR UPDATE', [before.id])).rows[0];
        if (workspace?.is_active === false) throw new AppError(409, 'The project Purchase FMS workspace is inactive');
        if (!workspace) await initializeProjectPurchase(db, before.id, null, actor.id);
        if (before.purchase_enabled) return summary(actor, before);
        await db.query(
            'UPDATE projects SET purchase_enabled=true, version=version+1, updated_at=clock_timestamp(), updated_by_id=$2 WHERE id=$1',
            [before.id, actor.id]
        );
        const saved = await projects.get(db, before.id);
        await projects.recordMetadataChange(db, before, saved, lower, 'Purchase FMS initiated from Tender FMS', actor.id);
        return summary(actor, saved);
    });
}
