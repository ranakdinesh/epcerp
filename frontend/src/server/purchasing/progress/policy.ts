import { invalid, isoDate } from '../core/validation';
import { quantity as decimal } from '../core/decimal';
import { indiaDate } from '../../personal-tasks/validation';
import type { Source, BO, Stage, Summary, Line, Package } from './types';
export const weights = [25, 25, 30, 10, 20, 25, 15, 50, 20, 10, 20, 240, 20, 10, 10, 10, 50, 10, 5, 5];
export const titles = ['BO item list', 'Purchase indent', 'Receive vendor quotations', 'Price comparison', 'Vendor onboarding & LOI', 'Technical documents', 'Submit to project team', 'Client technical approval', 'Purchase order release', 'Vendor PO acceptance', 'Advance payment request', 'Material inspection', 'Inspection report / release', 'Quality certificates', 'Client dispatch clearance', 'Material dispatch', 'Receipt at site', 'GRN / warranty', 'Project engineer confirmation', 'Accounts acceptance'];
export const done = (state: string) => ['completed', 'not_applicable'].includes(state);
export const percent = (units: number) => Math.floor((units * 10000 + 305) / 610) / 100;
function date(s: string) { if (!isoDate(s))
    invalid('date', 'Source contains an invalid ISO calendar date'); return Date.parse(s + 'T00:00:00Z'); }
function quantity(s: string) { if (!/^[0-9]{1,13}(\.[0-9]{1,3})?$/.test(s))
    invalid('quantity', 'Source quantity must be a positive exact decimal with at most three places'); return decimal(s); }
const normalizedQuantity = (s: string) => quantity(s.includes('.') ? s.replace(/0+$/, '').replace(/\.$/, '') : s);
function signal(state: string, due: string, today: string) { if (done(state))
    return 'complete'; if (due && due < today)
    return 'delayed'; if (state === 'blocked')
    return 'blocked'; if (!due)
    return 'unplanned'; return date(due) - date(today) <= 172800000 ? 'due_soon' : 'on_track'; }
export function laterCompletion(lines: Line[], packages: Package[]) { if (!lines.length || !packages.length)
    return []; const expected = new Map<string, bigint>(), actual = new Map<string, bigint>(), seen = new Set<string>(); try {
    for (const l of lines) {
        if (!l.demand_id || expected.has(l.demand_id))
            return [];
        expected.set(l.demand_id, normalizedQuantity(l.quantity));
    }
    for (const p of packages) {
        if (seen.has(p.id) || p.source_blockers.length || !expected.has(p.demand_id))
            return [];
        seen.add(p.id);
        actual.set(p.demand_id, (actual.get(p.demand_id) ?? BigInt(0)) + normalizedQuantity(p.quantity));
    }
}
catch {
    return [];
} for (const [id, n] of expected)
    if (actual.get(id) !== n)
        return []; return Array.from({ length: 17 }, (_, i) => i + 5).filter(stage => packages.every(p => done(p.evidence.find(f => f.number === stage)?.state ?? ''))); }
export function buildBO(bo: BO, s: Source, today: string): Summary {
    const out: Summary = { bo_id: bo.id, name: bo.name, progress_percent: 0, signal: 'on_track', completion_date: '', issues: [], scope_covered: false, steps: [] }, demands = new Map<string, bigint | null>(), lineIDs = new Set<string>();
    let coverage = bo.lines.length > 0;
    for (const l of bo.lines) {
        if (!l.id || lineIDs.has(l.id))
            invalid('line', 'Use unique current BO lines');
        lineIDs.add(l.id);
        let n: null | bigint = null;
        if (!l.quantity && bo.state !== 'completed')
            coverage = false;
        else
            n = quantity(l.quantity);
        if (!l.demand_id) {
            coverage = false;
            continue;
        }
        if (demands.has(l.demand_id))
            invalid('demand', 'BO lines must map to distinct demand identities');
        demands.set(l.demand_id, n);
    }
    const pkgIDs = new Set<string>(), quantities = new Map<string, bigint>(), facts = bo.packages.map(p => new Map(p.evidence.map(f => [f.number, f]))), plans = bo.packages.map(p => new Map(p.coordination.map(c => [c.number, c])));
    for (const [i, p] of bo.packages.entries()) {
        if (!p.id || pkgIDs.has(p.id))
            invalid('package', 'Use unique current work packages');
        pkgIDs.add(p.id);
        if (!demands.has(p.demand_id))
            invalid('demand', 'Work package must belong to a current BO demand');
        quantities.set(p.demand_id, (quantities.get(p.demand_id) ?? BigInt(0)) + quantity(p.quantity));
        if (facts[i].size !== p.evidence.length || p.evidence.some(f => f.number < 2 || f.number > 20 || !['not_started', 'in_progress', 'blocked', 'completed', 'not_applicable'].includes(f.state) || f.state === 'not_applicable' && (f.number !== 11 || !f.details.trim())))
            invalid('evidence', 'Use unique owned stage facts; N/A is only the advance disposition');
        if (plans[i].size !== p.coordination.length || p.coordination.some(c => c.number < 2 || c.number > 20 || !['source_linked', 'not_started', 'preparing', 'blocked'].includes(c.state)))
            invalid('coordination', 'Coordination cannot declare specialist completion');
        for (const c of p.coordination) {
            for (const d of [c.planned_date, c.resolution_due])
                if (d)
                    date(d);
            if (c.number === 20 && c.planned_date > out.completion_date)
                out.completion_date = c.planned_date;
        }
    }
    for (const [id, n] of demands)
        if (n === null || quantities.get(id) !== n)
            coverage = false;
    out.scope_covered = coverage;
    const grouped = new Map(bo.grouped.map(g => [g.number, g]));
    if (grouped.size !== bo.grouped.length || bo.grouped.some(g => g.number < 2 || g.number > 4 || !['not_started', 'in_progress', 'blocked', 'completed'].includes(g.state)))
        invalid('grouped', 'Current grouped evidence may supply only stages 2–4');
    let earned = 0, complete = true;
    for (let number = 1; number <= 20; number++) {
        const stage: Stage = { number, title: titles[number - 1], weight: weights[number - 1] / 10, contribution_percent: percent(weights[number - 1]), state: 'not_started', details: [], due_date: '', signal: '', missing_dates: !coverage, days_overdue: 0, blockers: [] };
        if (number === 1)
            stage.state = bo.state === 'completed' && bo.lines.length ? 'completed' : 'in_progress';
        else {
            let all = coverage && bo.packages.length > 0, anyWork = false, blocked = false, allNA = true;
            for (const [i, p] of bo.packages.entries()) {
                const f = facts[i].get(number), c = plans[i].get(number), finished = !!f && done(f.state);
                all = all && finished;
                allNA = allNA && f?.state === 'not_applicable';
                blocked = blocked || f?.state === 'blocked' || !finished && c?.state === 'blocked';
                anyWork = anyWork || !!f && f.state !== 'not_started' || c?.state === 'preparing';
                if (f?.details) {
                    stage.details.push(f.details);
                    if (f.state === 'blocked')
                        stage.blockers.push(f.details);
                }
                if (!finished && (p.source_blockers.length || s.blockers.length)) {
                    blocked = true;
                    stage.blockers.push(...p.source_blockers, ...s.blockers);
                }
                if (!finished && c?.state === 'blocked' && c.block_reason)
                    stage.blockers.push(c.block_reason);
                if (!c?.planned_date)
                    stage.missing_dates = true;
                const g = grouped.get(number), outstanding = g ? !done(g.state) : !finished;
                if (outstanding && c?.planned_date && (!stage.due_date || c.planned_date < stage.due_date))
                    stage.due_date = c.planned_date;
            }
            stage.state = all ? (number === 11 && allNA ? 'not_applicable' : 'completed') : blocked ? 'blocked' : anyWork ? 'in_progress' : 'not_started';
        }
        const g = grouped.get(number);
        if (g) {
            stage.state = g.state;
            stage.details.push(g.details);
        }
        if (done(stage.state)) {
            stage.due_date = '';
            stage.blockers = [];
            earned += weights[number - 1];
        }
        else
            complete = false;
        if (!done(stage.state) && (stage.blockers.length || s.blockers.length)) {
            stage.state = 'blocked';
            if (number === 1)
                stage.blockers.push(...s.blockers);
        }
        stage.signal = signal(stage.state, stage.due_date, today);
        if (stage.due_date && !done(stage.state) && stage.due_date < today)
            stage.days_overdue = Math.floor((date(today) - date(stage.due_date)) / 86400000);
        out.steps.push(stage);
    }
    out.progress_percent = percent(earned);
    if (!out.completion_date || out.steps[19].missing_dates)
        out.issues.push('Purchase completion date not fully planned (Step 20)');
    if (!s.project_start || !s.project_end)
        out.issues.push('Project start/end dates missing');
    if (out.completion_date && s.project_end && out.completion_date > s.project_end)
        out.issues.push('Purchase completion is after project end');
    if (out.completion_date && s.project_start && out.completion_date < s.project_start)
        out.issues.push('Purchase completion is before project start');
    if (s.project_start && s.project_end && s.project_start > s.project_end)
        out.issues.push('Project date range is invalid');
    if (!coverage)
        out.issues.push('Current purchase packages do not cover all BO lines and quantities');
    out.signal = complete ? 'complete' : out.steps.some(s => s.signal === 'delayed') || !!s.project_end && s.project_end < today || !!out.completion_date && out.completion_date < today ? 'delayed' : out.steps.some(s => s.signal === 'blocked') ? 'blocked' : out.steps.some(s => s.signal === 'due_soon') ? 'due_soon' : out.issues.length ? 'attention' : out.steps.some(s => !done(s.state) && s.missing_dates) ? 'unplanned' : 'on_track';
    return out;
}
export function build(s: Source, at = new Date()) { const today = indiaDate(at); date(today); for (const d of [s.project_start, s.project_end])
    if (d)
        date(d); const ids = new Set<string>(), packages = s.bos.map(bo => { if (!bo.id || ids.has(bo.id))
    invalid('bo', 'Use unique current BO identifiers'); ids.add(bo.id); return buildBO(bo, s, today); }), mean = packages.reduce((n, p) => n + p.progress_percent, 0) / packages.length * 100, floor = Math.floor(mean), rounded = mean - floor === .5 ? (floor % 2 ? floor + 1 : floor) : Math.round(mean); return { project_id: s.project_id, as_of: today, project_start: s.project_start, project_end: s.project_end, progress_percent: packages.length ? rounded / 100 : null, packages, source_blockers: [...s.blockers] }; }
export type RegisterState = {
    state: string;
    has_items: boolean;
    indent_complete: boolean;
    quotes_complete: boolean;
    approval_complete: boolean;
    pending: number;
    returned: number;
    approved: number;
    later_completed: number[];
};
export function registerCompletion(records: RegisterState[]) { const out = { pending: 0, returned: 0, approved: 0, percent: null as number | null, completed: [] as number[], next: 'PUR-01' }; if (!records.length)
    return out; for (const r of records) {
    out.pending += r.pending;
    out.returned += r.returned;
    out.approved += r.approved;
} out.percent = 0; if (records.some(r => r.state !== 'completed' || !r.has_items))
    return out; let units = weights[0]; const allIndents = records.every(r => r.indent_complete), allQuotes = allIndents && records.every(r => r.quotes_complete), allApprovals = records.every(r => r.approval_complete); for (const [n, complete] of [[2, allIndents], [3, allQuotes], [4, allApprovals]] as const)
    if (complete) {
        units += weights[n - 1];
        out.completed.push(n);
    } for (let n = 5; n <= 20; n++)
    if (records.every(r => r.later_completed.includes(n))) {
        units += weights[n - 1];
        out.completed.push(n);
    } const client = records.every(r => r.later_completed.includes(21)); if (client)
    out.completed.push(21); out.percent = percent(units); out.completed.unshift(1); out.next = allApprovals ? 'PUR-05' : allQuotes ? 'PUR-04' : allIndents ? 'PUR-03' : 'PUR-02'; if (allApprovals) {
    for (let n = 5; n <= 20; n++) {
        if (!out.completed.includes(n)) {
            out.next = 'PUR-' + String(n).padStart(2, '0');
            break;
        }
        if (n === 20)
            out.next = 'PUR-20';
    }
} if (out.next === 'PUR-09' && !client)
    out.next = 'PUR-PO-CLIENT'; return out; }
