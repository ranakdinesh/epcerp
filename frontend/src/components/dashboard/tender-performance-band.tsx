import type { TenderSummary } from "@/server/tenders/summary";

const inr = new Intl.NumberFormat("en-IN", { style: "currency", currency: "INR", maximumFractionDigits: 2, minimumFractionDigits: 0 });
const percent = new Intl.NumberFormat("en-IN", { style: "percent", maximumFractionDigits: 1 });
const amount = (value: string | null, count: number) => value === null ? (count ? "Not recorded" : inr.format(0)) : inr.format(Number(value));

export function TenderPerformanceBand({ data, error }: { data: TenderSummary | null; error: string }) {
  const decided = data ? data.won + data.lost : 0;
  const winRate = data && decided ? data.won / decided : null;
  return (
    <section className="go-dashboard-panel go-tender-band" aria-labelledby="tender-performance-title">
      <div className="go-dashboard-section-heading">
        <div>
          <h2 id="tender-performance-title">Tender performance{data ? ` · ${data.year}` : ""}</h2>
          <p>Tender FMS records opened this calendar year (IST)</p>
        </div>
      </div>
      {error ? <p role="alert" className="go-dashboard-inline-error">{error}</p> : !data ? <p role="status">Loading tender performance…</p> : <>
        <dl className="go-tender-counts">
          <div><dt>Processed</dt><dd>{data.processed.toLocaleString("en-IN")}</dd></div>
          <div><dt>Cleared for bidding</dt><dd>{data.cleared.toLocaleString("en-IN")}</dd></div>
          <div className="go-tender-won"><dt>Won</dt><dd>{data.won.toLocaleString("en-IN")}</dd></div>
          <div className="go-tender-lost"><dt>Lost</dt><dd>{data.lost.toLocaleString("en-IN")}</dd></div>
          <div><dt>Win rate</dt><dd>{winRate === null ? "—" : percent.format(winRate)}</dd></div>
        </dl>
        {winRate !== null && <div className="go-tender-result-bar" aria-hidden="true">
          <span style={{ width: `${winRate * 100}%` }} />
          <span style={{ width: `${(1 - winRate) * 100}%` }} />
        </div>}
        <p className="go-chart-note">{decided ? "Win rate is based on recorded wins and losses." : "No wins or losses recorded yet."}</p>
        <dl className="go-tender-values">
          <div><dt>Processed estimated value</dt><dd>{amount(data.processed_value, data.processed)}</dd><small>{data.estimated} of {data.processed} tenders have an estimate</small></div>
          <div className="go-tender-won"><dt>Won bid value</dt><dd>{amount(data.won_value, data.won)}</dd></div>
          <div className="go-tender-lost"><dt>Lost bid value</dt><dd>{amount(data.lost_value, data.lost)}</dd></div>
        </dl>
      </>}
    </section>
  );
}
