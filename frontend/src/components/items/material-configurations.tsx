"use client";
import { ValidatedInput, ValidatedLabel } from "@/components/ui/validated-controls";
import { useEffect, useState } from "react";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import {
  type Definition,
  request,
  button,
  secondary,
  panel,
  input,
} from "./item-data";
import { ConfigurationEditor } from "./configuration-editor";
import { DataGrid, GridAction, GridStatus } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ItemHistory } from "./item-history";
export function MaterialConfigurations() {
  const user = useAuthenticatedUser();
  const canWrite =
    user.permissions.includes("system:admin") ||
    user.permissions.includes("settings.material_catalogue.write");
  const [records, setRecords] = useState<Definition[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);
  const [editor, setEditor] = useState<Definition | null | undefined>(
    undefined,
  );
  const [reason, setReason] = useState("");
  const [viewId, setViewId] = useState<string | null>(null);
  useEffect(() => {
    let live = true;
    request<Definition[]>("/api/settings/material-catalogue")
      .then((r) => {
        if (live) {
          setRecords(r);
          setLoaded(true);
        }
      })
      .catch((e) => {
        if (live) setError(e.message);
      });
    return () => {
      live = false;
    };
  }, [reload]);
  return (
    <section className="space-y-5">
      <div className={panel}>
        <h2 className="text-2xl font-bold">Categories & configurations</h2>
        <p className="text-slate-600">
          Define what an item is before assigning its manufacturer and SKU.
        </p>
        <p>
          {records.filter((r) => r.data.kind === "category").length} categories
          · {records.filter((r) => r.data.kind === "type").length} configuration
          definitions
        </p>
        {canWrite && (
          <button className={button} onClick={() => setEditor(null)}>
            New definition
          </button>
        )}
      </div>
      {error && (
        <p role="alert" className="text-red-700">
          {error}
        </p>
      )}
      {!loaded && !error && <p>Loading configuration masters…</p>}
      {editor !== undefined && (
        <RecordDrawer
          title={
            editor
              ? "Edit categories & configurations"
              : "New categories & configurations"
          }
          onClose={() => setEditor(undefined)}
        >
          <ConfigurationEditor
            key={editor?.id ?? "new"}
            source={editor ?? undefined}
            records={records}
            onSaved={() => {
              setEditor(undefined);
              setReload((r) => r + 1);
            }}
            onClose={() => setEditor(undefined)}
          />
        </RecordDrawer>
      )}{" "}
      <DataGrid<Definition>
        label="Categories & configurations"
        rows={records}
        loading={!loaded}
        initialSort="name"
        status={(r) => (r.is_active ? "Active" : "Inactive")}
        columns={[
          {
            key: "code",
            label: "Code",
            value: (r) => r.code,
            className: "font-mono text-xs",
          },
          {
            key: "name",
            label: "Name",
            value: (r) => r.name,
            className: "font-semibold",
          },
          { key: "kind", label: "Kind", value: (r) => r.data.kind },
          {
            key: "category",
            label: "Category",
            value: (r) =>
              records.find((c) => c.code === r.data.category_code)?.name ?? "—",
          },
          {
            key: "uom",
            label: "UOM",
            value: (r) => (r.data.kind === "type" ? (r.data.uom ?? "—") : "—"),
          },
          {
            key: "revision",
            label: "Revision",
            value: (r) => r.data.revision ?? "—",
          },
          {
            key: "status",
            label: "Status",
            value: (r) => (r.is_active ? "Active" : "Inactive"),
            render: (r) => (
              <GridStatus value={r.is_active ? "Active" : "Inactive"} />
            ),
          },
        ]}
        actions={(r) => (
          <>
            <GridAction
              label={`View ${r.name}`}
              onClick={() => setViewId(r.id)}
            />
            {canWrite && r.data.kind === "type" && (
              <GridAction
                kind="edit"
                label={`New revision of ${r.name}`}
                onClick={() => setEditor(r)}
              />
            )}
          </>
        )}
      />
      {viewId && (
        <RecordDrawer
          title="Categories & configurations details"
          onClose={() => setViewId(null)}
        >
          {" "}
          {canWrite && (
            <ValidatedLabel className="block">
              Reason for activation/deactivation
              <ValidatedInput name="reason"
                className={input}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
              />
            </ValidatedLabel>
          )}{" "}
          <div className="space-y-4">
            {records
              .filter((r) => r.id === viewId)
              .map((r) => (
                <article className={panel} key={r.id}>
                  <div>
                    <span className="text-xs font-semibold text-blue-700">
                      {r.code} · {r.data.kind} ·{" "}
                      {r.is_active ? "Active" : "Inactive"}
                    </span>
                    <h3 className="text-lg font-bold">{r.name}</h3>
                  </div>
                  {r.data.kind === "type" && (
                    <>
                      <p className="text-sm">
                        Category {r.data.category_code} · {r.data.uom} ·
                        Revision {r.data.revision}
                      </p>
                      <details>
                        <summary className="cursor-pointer">
                          {r.data.fields?.length} specification fields
                        </summary>
                        <ul className="space-y-2 pt-3 text-sm">
                          {r.data.fields?.map((f) => (
                            <li key={f.code}>
                              <strong>
                                {f.label}
                                {f.required ? " *" : ""}
                              </strong>
                              <p className="text-slate-500">
                                {f.options.map((o) => o.label).join(", ")}
                              </p>
                            </li>
                          ))}
                        </ul>
                      </details>
                    </>
                  )}
                  {canWrite && (
                    <div className="flex flex-wrap gap-2">
                      {r.data.kind === "type" && (
                        <button
                          className={secondary}
                          onClick={() => {
                            setViewId(null);
                            setEditor(r);
                          }}
                        >
                          New revision
                        </button>
                      )}
                      <button
                        disabled={!reason.trim()}
                        className={secondary}
                        onClick={async () => {
                          try {
                            await request(
                              `/api/settings/material-catalogue/${r.id}`,
                              {
                                version: r.version,
                                is_active: !r.is_active,
                                reason,
                              },
                              "PATCH",
                            );
                            setReload((v) => v + 1);
                          } catch (e) {
                            setError((e as Error).message);
                          }
                        }}
                      >
                        {r.is_active ? "Deactivate" : "Reactivate"}
                      </button>
                    </div>
                  )}
                  <ItemHistory
                    url={`/api/settings/material-catalogue/${r.id}/history`}
                  />
                </article>
              ))}
          </div>
        </RecordDrawer>
      )}
    </section>
  );
}
