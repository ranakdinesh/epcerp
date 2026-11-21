import 'server-only';
import type { DB } from '../db';
import { invalid, text, blankCalculation, type Capture } from './validation';
const day = 86400000;
export function date(value: string, field: string) {
    const ms = Date.parse(value + 'T00:00:00Z');
    if (!/^\d{4}-\d{2}-\d{2}$/.test(value) || value.slice(0,4)==='0000' || !Number.isFinite(ms) || new Date(ms).toISOString().slice(0,10)!==value) invalid(field,'Use a valid date in YYYY-MM-DD format');
    return ms;
}
const amount = /^(0|[1-9][0-9]{0,13})(\.[0-9]{1,2})?$/;
function whole(s: string, field: string, min: number) { if (!/^[+-]?\d+$/.test(s) || !Number.isSafeInteger(Number(s)) || Number(s) < min || Number(s) > 36500) invalid(field,'Enter days within the supported range'); return Number(s); }
export function validate(c: Capture, mode: 'create' | 'draft' | 'schedule', name = '') {
    c = { ...c };
    const complete = mode !== 'draft';
    if (mode !== 'schedule') {
        const limits = {short_name:255,project_type:32,site_name:255,site_state:128,site_city:128,scope_summary:10000,currency:3,loi_number:64,po_number:64,schedule_start_basis:32,remarks:10000} as const;
        for (const [key,max] of Object.entries(limits)) c[key as keyof typeof limits] = text(c[key as keyof typeof limits],key,max);
        c.short_name ||= name;
        c.currency ||= 'INR';
        c.currency = [...c.currency].map(ch => [...ch.toUpperCase()].length===1 ? ch.toUpperCase() : ch).join('');
        if (!/^[A-Z]{3}$/.test(c.currency)) invalid('currency','Use a three-letter currency code');
        if ((complete || c.contract_value) && !amount.test(c.contract_value)) invalid('contract_value','Enter a nonnegative project value with at most 14 integer and 2 decimal digits');
        if ((complete || c.project_type) && !['electrical','solar','ev','mixed'].includes(c.project_type)) invalid('project_type','Select a business type');
        if (complete && !c.site_state) invalid('site_state','Select the site state');
        if (complete && !c.site_city) invalid('site_city','Select the city / location');
    }
    if ((!c.planned_duration_days || c.actual_start_date) && c.planned_completion_date) c.planned_duration_days='1';
    let duration = 0;
    if (complete || c.planned_duration_days) { duration = whole(c.planned_duration_days,'planned_duration_days',1); c.planned_duration_days=String(duration); }
    if (c.safety_buffer_days) c.safety_buffer_days=String(whole(c.safety_buffer_days,'safety_buffer_days',0));
    if (c.safety_buffer_percent && (!amount.test(c.safety_buffer_percent) || Number(c.safety_buffer_percent)>100)) invalid('safety_buffer_percent','Enter a percentage from 0 to 100');
    for (const field of ['planned_start_date','planned_completion_date','loi_date','po_date','actual_start_date','actual_end_date','site_handover_date'] as const) if(c[field]) date(c[field],field);
    if(c.actual_end_date && (!c.actual_start_date || c.actual_end_date<c.actual_start_date)) invalid('actual_end_date','Actual end requires an actual start and cannot precede it');
    const basis: Record<string,string> = {manual:c.planned_start_date,loi_date:c.loi_date,po_date:c.po_date,site_handover_date:c.site_handover_date,actual_start_date:c.actual_start_date};
    c.schedule_start_basis ||= 'loi_date';
    if (!Object.hasOwn(basis,c.schedule_start_basis)) invalid('schedule_start_basis','Select a start date basis');
    const selected=basis[c.schedule_start_basis];
    if (!complete && (!selected || (!c.planned_duration_days && !c.planned_completion_date))) return c;
    if (!selected) invalid(c.schedule_start_basis==='manual'?'planned_start_date':c.schedule_start_basis,'The date matching the selected basis is required');
    c.planned_start_date ||= selected;
    const start=date(c.planned_start_date,'planned_start_date');
    if(!c.planned_completion_date){ const finish=new Date(start+(duration-1)*day).toISOString();if(finish.length!==24)invalid('planned_duration_days','Calculated finish exceeds supported dates');c.planned_completion_date=finish.slice(0,10); }
    const finish=date(c.planned_completion_date,'planned_completion_date');
    if(finish<start || finish-start>36500*day) invalid('planned_completion_date','Completion must follow start and be within 100 years');
    if(c.site_handover_date && c.site_handover_date<c.planned_completion_date) invalid('site_handover_date','Site handover must be on or after planned completion');
    return c;
}
export async function calculate(db: DB, input: Capture) {
    const c = { ...input }, calc=blankCalculation();
    const selected: Record<string,string>={loi_date:c.loi_date,po_date:c.po_date,actual_start_date:c.actual_start_date,site_handover_date:c.site_handover_date};
    calc.contractual_start=selected[c.schedule_start_basis] ?? c.planned_start_date;
    const gap=(a:string,b:string)=>a&&b?(date(a,'date')-date(b,'date'))/day:null;
    calc.contractual_start_gap=gap(c.actual_start_date,calc.contractual_start);calc.planned_start_gap=gap(c.actual_start_date,c.planned_start_date);
    if(!c.actual_start_date || !c.planned_completion_date){c.planned_duration_days='';c.safety_buffer_days='';c.schedule_calculation=calc;return c;}
    const holidays = new Set((await db.query('SELECT holiday_date::text AS date FROM work_calendar_holidays WHERE is_active')).rows.map(r=>r.date));
    let calendar=0,weekends=0,holiday=0,working=0;const end=date(c.planned_completion_date,'planned_completion_date'),start=date(c.actual_start_date,'actual_start_date');
    if(end-start>36500*day)invalid('actual_start_date','Working calendar range exceeds 100 years');
    for(let n=start;n<=end;n+=day){const d=new Date(n),weekday=d.getUTCDay(),week=Math.floor((d.getUTCDate()-1)/7)+1;calendar++;if(weekday===0 || weekday===6 && [2,4].includes(week))weekends++;else if(holidays.has(d.toISOString().slice(0,10)))holiday++;else working++;}
    const [whole,fraction='']=c.safety_buffer_percent.split('.'),bp=Number(whole||'0')*100+Number((fraction+'00').slice(0,2));
    const reserved=Math.floor((working*bp+9999)/10000),execution=working-reserved;
    Object.assign(calc,{calendar_days:calendar,weekend_days:weekends,holiday_days:holiday,completion_days:working,reserved_buffer_days:reserved,execution_days:execution});
    c.planned_duration_days=String(working);c.safety_buffer_days=String(execution);c.schedule_calculation=calc;return c;
}
