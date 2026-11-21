"use client";

import Link from "next/link";
import { useCallback, useState } from "react";
import { useDashboardSource } from "@/components/dashboard/use-dashboard-source";
import { UserIdentity } from "@/components/personal-profile";
import { purchaseProjectHref, type Dashboard, type PurchaseProject } from "@/components/purchase-fms/data";
import { actionButtonClass } from "@/components/ui/action-button";
import { DataGrid, GridAction, GridStatus, type GridQuery } from "@/components/ui/data-grid";
import { ProcurementOwnerEditor } from "./project-procurement-ownership";

export function ProjectPurchaseRegister() {
  const [query, setQuery] = useState({ page: 1, size: 20, search: "", sort: "number", direction: "asc" });
  const [revision, setRevision] = useState(0);
  const [selected, setSelected] = useState<PurchaseProject | null>(null);
  const [notice, setNotice] = useState("");
  const params = new URLSearchParams({
    page: String(query.page), page_size: String(query.size), search: query.search,
    sort: query.sort, direction: query.direction, enabled_only: "true",
  });
  const { data, error, loading } = useDashboardSource<Dashboard>(`/api/purchase-fms?${params}`, revision);
  const changeQuery = useCallback(({ page, size, search, sort, direction }: GridQuery) => {
    setQuery({ page, size, search, sort, direction });
  }, []);

  return <section className="space-y-4">
    <header>
      <p className="go-eyebrow">Project delivery</p>
      <h1 className="text-2xl font-bold">Projects</h1>
      <p className="text-sm text-slate-600">Assign Purchase FMS ownership to an eligible Procurement Executive or yourself.</p>
    </header>
    {notice && <p role="status" className="rounded-lg bg-green-50 p-3 text-sm text-green-800">{notice}</p>}
    {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
    <DataGrid<PurchaseProject>
      label="Purchase FMS projects"
      rows={data?.items ?? []}
      loading={loading}
      remote={{ total: data?.total ?? 0, onChange: changeQuery }}
      initialSort="number"
      toolbar={<button type="button" className={actionButtonClass("secondary")} onClick={() => setRevision(v => v + 1)}>Refresh projects</button>}
      columns={[
        { key: "number", label: "Project number", value: p => p.project_number, render: p => <Link className="font-semibold text-blue-700 underline" href={purchaseProjectHref(p.id, p.next_step)}>{p.project_number}</Link> },
        { key: "name", label: "Project", value: p => p.name },
        { key: "owner", label: "Purchase assignee", sortable: false, value: p => p.purchase_manager_id ?? "Unassigned",
          render: p => p.purchase_manager_id ? <UserIdentity id={p.purchase_manager_id} name="Purchase assignee" /> : "Unassigned" },
        { key: "state", label: "Status", value: p => p.lifecycle_status, render: p => <GridStatus value={p.lifecycle_status.replaceAll("_", " ")} /> },
      ]}
      actions={p => p.can_assign && <GridAction kind="edit" label={`Assign Purchase FMS for ${p.name}`} onClick={() => { setNotice(""); setSelected(p); }} />}
    />
    {selected && <ProcurementOwnerEditor project={selected} close={() => setSelected(null)} saved={() => {
      setSelected(null);
      setNotice("Purchase FMS assignment saved.");
      setRevision(v => v + 1);
    }} />}
  </section>;
}
