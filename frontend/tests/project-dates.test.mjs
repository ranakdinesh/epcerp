import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const output=ts.transpileModule(readFileSync(new URL('../src/components/projects/project-date-model.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText;
const {projectDateVariance,calendarDaysToTarget,recalculateProjectDates,handoverDateError}=await import(`data:text/javascript;base64,${Buffer.from(output).toString('base64')}`);
const project=overrides=>({planned_start_date:'2026-10-01',planned_completion_date:'2026-10-30',actual_start_date:null,is_active:true,lifecycle_status:'active',...overrides});
const form=overrides=>({...project(),schedule_start_basis:'loi_date',loi_date:'2026-10-01',po_date:null,site_handover_date:null,planned_duration_days:30,net_running_days:25,...overrides});
test('actual start and actual basis preserve the baseline and fixed completion',()=>{
 for (const [key,value] of [['actual_start_date','2026-10-09'],['schedule_start_basis','actual_start_date']]) {
  const next=recalculateProjectDates(form({actual_start_date:'2026-10-09'}),key,value);
  assert.equal(next.planned_start_date,'2026-10-01');assert.equal(next.planned_completion_date,'2026-10-30');assert.equal(next.planned_duration_days,30);
  assert.equal(calendarDaysToTarget(next.actual_start_date,next.planned_completion_date),22);
 }
});
test('missing duration derives from planned dates, explicit duration still changes the plan',()=>{
 assert.equal(recalculateProjectDates(form({planned_duration_days:null}),'actual_start_date','2026-10-09').planned_duration_days,30);
 assert.equal(recalculateProjectDates(form(),'planned_duration_days',31).planned_completion_date,'2026-10-31');
 assert.equal(recalculateProjectDates(form(),'planned_completion_date','2026-11-01').planned_duration_days,32);
});
test('capacity includes the start day across leap days and years and clamps beyond-target starts',()=>{
 assert.equal(calendarDaysToTarget('2024-02-28','2024-03-01'),3);
 assert.equal(calendarDaysToTarget('2026-12-31','2027-01-01'),2);
 assert.equal(calendarDaysToTarget('2026-10-31','2026-10-30'),0);
 assert.equal(calendarDaysToTarget('2026-10-30','2026-10-30'),1);
 assert.equal(calendarDaysToTarget(null,'2026-10-30'),null);
});
test('late start and completion overdue are independent date facts, not summed',()=>{
 const status=projectDateVariance(project({actual_start_date:'2026-10-03'}),'2026-11-01');
 assert.equal(status.startDays,2);assert.equal(status.completionDays,2);assert.equal(status.reasons.length,2);
});
test('exact start/finish dates have no overdue days; midnight changes overdue by one',()=>{
 assert.equal(projectDateVariance(project({actual_start_date:'2026-10-01'}),'2026-10-30').late,false);
 assert.equal(projectDateVariance(project({actual_start_date:'2026-10-01'}),'2026-10-31').completionDays,1);
 assert.equal(projectDateVariance(project(),'2026-10-01').startDays,0);
});
test('future actual start is not treated as started and missing baseline is not guessed',()=>{
 const status=projectDateVariance(project({actual_start_date:'2026-10-09'}),'2026-10-07');
 assert.equal(status.startDays,6);assert.match(status.reasons[0],/Start overdue/);
 assert.equal(projectDateVariance(project({planned_start_date:null,planned_completion_date:null}),'2026-10-07').late,false);
});
test('closed and deactivated projects do not produce live late warnings',()=>{
 for(const values of [{is_active:false},{lifecycle_status:'closed'}])assert.equal(projectDateVariance(project(values),'2026-11-01').late,false);
});

test('irrelevant milestone edits preserve a calculated running-days value and explicit target',()=>{
 const next=recalculateProjectDates(form({planned_completion_date:'2026-11-02'}),'po_date','2026-10-15');
 assert.equal(next.net_running_days,25);assert.equal(next.planned_completion_date,'2026-11-02');
 assert.equal(recalculateProjectDates(form(),'actual_start_date','2026-10-09').net_running_days,25);
});

test('missing legacy plan rebuilds from LOI and duration when actual start changes',()=>{
 const next=recalculateProjectDates(form({planned_start_date:null,planned_completion_date:null,planned_duration_days:30,net_running_days:null}),'actual_start_date','2026-10-09');
 assert.equal(next.planned_start_date,'2026-10-01');assert.equal(next.planned_completion_date,'2026-10-30');
 assert.equal(calendarDaysToTarget(next.actual_start_date,next.planned_completion_date),22);
});
test('LOI autofills an editable baseline and preserves an explicit override',()=>{
 const automatic=recalculateProjectDates(form({planned_start_date:null,planned_completion_date:null,loi_date:null}),'loi_date','2026-10-01');
 assert.equal(automatic.planned_start_date,'2026-10-01');assert.equal(automatic.planned_completion_date,'2026-10-30');
 const overridden=recalculateProjectDates(automatic,'planned_start_date','2026-10-03');
 assert.equal(overridden.planned_start_date,'2026-10-03');assert.equal(overridden.planned_completion_date,'2026-11-01');
 const changed=recalculateProjectDates(overridden,'loi_date','2026-10-02');
 assert.equal(changed.planned_start_date,'2026-10-03');assert.equal(changed.planned_completion_date,'2026-11-01');
 const actual=recalculateProjectDates(changed,'actual_start_date','2026-10-09');
 assert.equal(actual.planned_start_date,'2026-10-03');assert.equal(actual.planned_completion_date,'2026-11-01');
});
test('changing basis seeds its milestone, clearing actual restores planned-date calculation',()=>{
 const next=recalculateProjectDates(form({po_date:'2026-10-05',planned_start_date:'2026-10-03'}),'schedule_start_basis','po_date');
 assert.equal(next.planned_start_date,'2026-10-05');
 const cleared=recalculateProjectDates({...next,actual_start_date:'2026-10-09'},'actual_start_date',null);
 assert.equal(cleared.actual_start_date,null);assert.equal(cleared.planned_start_date,'2026-10-05');
});
test('site handover cannot precede completion; equal, later and optional empty dates are valid',()=>{
 assert.match(handoverDateError(form({site_handover_date:'2026-10-29'})),/on or after/);
 for(const site_handover_date of [null,'2026-10-30','2026-10-31'])assert.equal(handoverDateError(form({site_handover_date})), '');
});
