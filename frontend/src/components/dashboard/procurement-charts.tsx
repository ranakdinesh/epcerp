import type {PurchaseProject} from "@/components/purchase-fms/data";
import {projectChart,progressCoverage,averagePurchaseProgress} from "./dashboard-model";
export function ProcurementCharts({rows,total,loading,error}:{rows:PurchaseProject[];total:number|null;loading:boolean;error:string}){
 const states=projectChart(rows),coverage=progressCoverage(rows),maximum=Math.max(1,...states.map(s=>s.count));
 const caption=total!==null?`${rows.length} of ${total} projects in the current view`:"Current project view";
 const average=averagePurchaseProgress(rows),fraction=(average??0)/100;
 return <div className="go-dashboard-charts"><section className="go-dashboard-panel" aria-labelledby="project-status-title"><div className="go-dashboard-section-heading"><h2 id="project-status-title">Project status</h2><span>{caption}</span></div>
 {loading?<p role="status" className="go-dashboard-placeholder">Loading project status…</p>:error?<p className="go-dashboard-placeholder">Project status unavailable.</p>:<div className="go-status-chart" role="img" aria-label={states.map(s=>`${s.label}: ${s.count}`).join(", ")}>{states.filter(s=>s.count>0||["draft","active","on_hold","closed"].includes(s.key)).map(s=><div className="go-status-chart-row" key={s.key}><span>{s.label}</span><div className="go-status-track"><div className={`go-status-bar go-tone-${s.tone}`} style={{width:`${s.count/maximum*100}%`}}/></div><strong>{s.count}</strong></div>)}</div>}
 <p className="go-chart-note">Project lifecycle, separate from purchasing completion.</p></section>
 <section className="go-dashboard-panel" aria-labelledby="purchase-progress-title"><div className="go-dashboard-section-heading"><h2 id="purchase-progress-title">Purchasing progress</h2></div><div className="go-progress-chart">
 <div className="go-progress-ring"><svg viewBox="0 0 160 160" aria-hidden="true"><circle cx="80" cy="80" r="62" fill="none" stroke="var(--dash-track)" strokeWidth="16"/>{coverage.recorded>0&&!loading&&!error&&<circle cx="80" cy="80" r="62" fill="none" stroke="var(--dash-teal)" strokeWidth="16" strokeDasharray={`${fraction*389.56} 389.56`} transform="rotate(-90 80 80)"/>}</svg><span>{loading||error||!coverage.recorded?"—":`${average}%`}</span></div>
 <strong>{loading?"Loading recorded progress…":error?"Progress unavailable":!coverage.total?"No projects in this view":!coverage.recorded?"No purchase progress recorded":"Average earned purchase progress"}</strong>
 <p>{coverage.recorded?`${coverage.recorded} projects in this view · ${coverage.missing} without recorded progress.`:"Progress appears from saved execution evidence."}</p></div><p className="go-chart-note">{caption} · Configured stage weights · All 20 stages use recorded completion evidence and current demand coverage.</p></section></div>;
}
