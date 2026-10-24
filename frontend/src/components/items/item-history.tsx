"use client";
import { useState } from "react";
import { request, secondary } from "./item-data";
export function ItemHistory({ url }: { url: string }) {
  const [entries, setEntries] = useState<
    | {
        id: string;
        action: string;
        reason: string;
        created_at: string;
        created_by_id: string | null;
        before: unknown;
        after: unknown;
      }[]
    | null
  >(null);
  const [error, setError] = useState("");
  return (
    <div>
      <button
        type="button"
        className={secondary}
        onClick={async () => {
          if (entries) {
            setEntries(null);
            return;
          }
          try {
            setEntries(await request(url));
          } catch (e) {
            setError((e as Error).message);
          }
        }}
      >
        History
      </button>
      {error && <p role="alert">{error}</p>}
      {entries && (
        <ol className="mt-3 space-y-3">
          {entries.map((e) => (
            <li key={e.id} className="rounded-lg bg-slate-50 p-3 text-sm">
              <strong>{e.action}</strong> ·{" "}
              {new Date(e.created_at).toLocaleString()}
              <p>{e.reason}</p>
              <p className="text-xs text-slate-500">
                Actor: {e.created_by_id ?? "Reference migration"}
              </p>
              <details>
                <summary>Recorded changes</summary>
                <pre className="overflow-auto text-xs">
                  {JSON.stringify(
                    { before: e.before, after: e.after },
                    null,
                    2,
                  )}
                </pre>
              </details>
            </li>
          ))}
          {!entries.length && <li>No history recorded.</li>}
        </ol>
      )}
    </div>
  );
}
