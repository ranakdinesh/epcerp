"use client";
import { useSearchParams } from "next/navigation";
import { Fragment, useDeferredValue, useEffect, useState, type ReactNode } from "react";
export type GridQuery = { search: string; status: string; sort: string; direction: "asc" | "desc"; page: number; size: number };
export type GridColumn<T> = {
  key: string;
  label: string;
  value: (row: T) => string | number;
  render?: (row: T) => ReactNode;
  className?: string;
  headerContent?: ReactNode;
  headerClassName?: string;
  sortable?: boolean;
};
export function DataGrid<T extends { id: string }>({
  label,
  rows,
  columns,
  actions,
  footer,
  loading = false,
  searchable = true,
  status,
  initialSort,
  toolbar,
  onRowClick,
  stateKey,
  grouping,
  paginate = true,
  remote,
}: {
  remote?: { total: number; onChange: (query: GridQuery) => void };
  label: string;
  rows: T[];
  columns: GridColumn<T>[];
  actions?: (row: T) => ReactNode;
  footer?: ReactNode;
  loading?: boolean;
  searchable?: boolean;
  paginate?: boolean;
  status?: (row: T) => string;
  initialSort?: string;
  toolbar?: ReactNode;
  onRowClick?: (row: T) => void;
  stateKey?: string;
  grouping?: { columnKey: string; key: (row: T) => string; label: (row: T) => string };
}) {
  const params = useSearchParams();
  function saved(key: string, fallback: string) { return stateKey ? params.get(`${stateKey}_${key}`) ?? fallback : fallback; }
  function remember(key: string, value: string) {
    if (!stateKey) return;
    const url = new URL(window.location.href);
    url.searchParams.set(`${stateKey}_${key}`, value);
    if (!["page", "size"].includes(key)) url.searchParams.set(`${stateKey}_page`, "1");
    window.history.replaceState(null, "", `${url.pathname}${url.search}`);
  }
  const [query, setQuery] = useState(() => saved("search", ""));
  const deferred = useDeferredValue(query);
  const [filter, setFilter] = useState(() => saved("filter", ""));
  const [sort, setSort] = useState(() => saved("sort", initialSort ?? columns[0].key));
  const [direction, setDirection] = useState<"asc" | "desc">(() => saved("direction", "asc") === "desc" ? "desc" : "asc");
  const [page, setPage] = useState(() => Math.max(1, Number(saved("page", "1")) || 1));
  const [size, setSize] = useState(() => [20,50,100].includes(Number(saved("size", "20"))) ? Number(saved("size", "20")) : 20);
  const column = columns.find((c) => c.key === sort) ?? columns[0];
  const remoteChange = remote?.onChange;
  useEffect(() => { remoteChange?.({ search: deferred, status: filter, sort, direction, page, size }); }, [remoteChange, deferred, filter, sort, direction, page, size]);
  const filtered = remote ? rows : rows
    .filter(
      (r) =>
        (!filter || status?.(r) === filter) &&
        (!deferred.trim() ||
          columns
            .map((c) => c.value(r))
            .join(" ")
            .toLocaleLowerCase()
            .includes(deferred.trim().toLocaleLowerCase())),
    )
    .sort(
      (a, b) => {
        if (grouping && grouping.key(a) !== grouping.key(b)) {
          const groupOrder = grouping.label(a).localeCompare(grouping.label(b), undefined, { numeric: true, sensitivity: "base" }) || grouping.key(a).localeCompare(grouping.key(b));
          return groupOrder * (sort === grouping.columnKey && direction === "desc" ? -1 : 1);
        }
        return String(column.value(a)).localeCompare(
          String(column.value(b)),
          undefined,
          { numeric: true, sensitivity: "base" },
        ) * (direction === "asc" ? 1 : -1);
      },
    );
  const total = remote?.total ?? filtered.length;
  const count = Math.max(1, Math.ceil(total / size));
  const current = remote ? page : Math.min(page, count);
  const visible = paginate && !remote ? filtered.slice((current - 1) * size, current * size) : filtered;
  return (
    <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white">
      {(searchable || status || toolbar || paginate) && <div className="data-grid-toolbar flex flex-wrap items-end justify-between gap-3 border-b border-slate-100 p-4">
        <div className="data-grid-toolbar-main flex flex-wrap items-end gap-3">
          {searchable && (
            <label className="data-grid-search text-xs font-semibold text-slate-600">
              Search {label.toLowerCase()}
              <input
                className="mt-1 block min-h-11 w-full rounded-lg border border-slate-200 bg-slate-50 px-3 text-sm sm:w-80"
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value);
                  remember("search", e.target.value);
                  setPage(1);
                }}
                placeholder="Search records"
              />
            </label>
          )}
          {status && (
            <label className="text-xs font-semibold text-slate-600">
              Status
              <select
                aria-label={`${label} status`}
                className="mt-1 block min-h-11 rounded-lg border border-slate-200 px-3 text-sm"
                value={filter}
                onChange={(e) => {
                  setFilter(e.target.value);
                  remember("filter", e.target.value);
                  setPage(1);
                }}
              >
                <option value="">All statuses</option>
                {(remote ? ["Active", "Inactive"] : [...new Set(rows.map(status))].sort()).map((s) => (
                  <option key={s}>{s}</option>
                ))}
              </select>
            </label>
          )}
          {toolbar}
        </div>
        {paginate && <label className="data-grid-rows flex min-h-11 items-center gap-2 text-sm text-slate-600">
          Rows
          <select
            aria-label={`${label} rows per page`}
            className="min-h-11 rounded-lg border border-slate-200 px-3"
            value={size}
            onChange={(e) => {
              setSize(Number(e.target.value));
              remember("size", e.target.value);
              remember("page", "1");
              setPage(1);
            }}
          >
            {[20, 50, 100].map((n) => (
              <option key={n}>{n}</option>
            ))}
          </select>
        </label>}
      </div>}
      <div
        className="overflow-x-auto focus-visible:outline-2 focus-visible:outline-blue-500"
        tabIndex={0}
        role="region"
        aria-label={`${label} grid`}
      >
        <table
          className="vtpl-data-table w-full min-w-[800px] text-left text-sm"
          aria-label={label}
          aria-busy={loading}
        >
          <thead className="bg-slate-50 text-xs uppercase text-slate-500">
            <tr>
              {columns.map((c) => (
                <th
                  key={c.key}
                  scope="col"
                  className={`px-4 py-3 ${c.headerClassName ?? ""}`}
                  aria-sort={
                    c.sortable === false ? undefined : sort === c.key
                      ? direction === "asc"
                        ? "ascending"
                        : "descending"
                      : "none"
                  }
                >
                  {c.sortable === false ? <span>{c.label}</span> : <button
                    type="button"
                    className="inline-flex min-h-8 items-center gap-2 whitespace-nowrap rounded font-semibold focus-visible:outline-2 focus-visible:outline-blue-500"
                    onClick={() => {
                      setDirection(
                        sort === c.key && direction === "asc" ? "desc" : "asc",
                      );
                      setSort(c.key);
                      remember("direction", sort === c.key && direction === "asc" ? "desc" : "asc");
                      remember("sort", c.key);
                      setPage(1);
                    }}
                  >
                    {c.label}
                    <span aria-hidden="true">
                      {sort === c.key
                        ? direction === "asc"
                          ? "↑"
                          : "↓"
                        : "↕"}
                    </span>
                  </button>}
                  {c.headerContent}
                </th>
              ))}
              {actions && <th scope="col" className="px-4 py-3 text-right">
                Actions
              </th>}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {visible.map((row, index) => (
              <Fragment key={row.id}>
              {grouping && (index === 0 || grouping.key(visible[index - 1]) !== grouping.key(row)) && (
                <tr data-grid-group={grouping.key(row)}>
                  <th colSpan={columns.length + (actions ? 1 : 0)} scope="rowgroup" className="border-t-2 border-blue-300 bg-blue-100 px-4 py-3 text-sm font-bold text-blue-950">
                    Group: {grouping.label(row)}
                  </th>
                </tr>
              )}
              <tr
                key={row.id}
                tabIndex={onRowClick ? 0 : undefined}
                className={`hover:bg-slate-50/80 ${onRowClick ? "cursor-pointer focus-visible:outline-2 focus-visible:outline-blue-600 focus-visible:-outline-offset-2" : ""}`}
                onClick={
                  onRowClick
                    ? (event) => {
                        if (
                          !(event.target as Element).closest(
                            "a,button,input,select,textarea,summary",
                          )
                        )
                          onRowClick(row);
                      }
                    : undefined
                }
                onKeyDown={
                  onRowClick
                    ? (event) => {
                        if (
                          event.target === event.currentTarget &&
                          ["Enter", " "].includes(event.key)
                        ) {
                          event.preventDefault();
                          onRowClick(row);
                        }
                      }
                    : undefined
                }
              >
                {columns.map((c) => (
                  <td
                    key={c.key}
                    className={`px-4 py-3 align-top ${c.className ?? ""}`}
                  >
                    {c.render ? c.render(row) : c.value(row)}
                  </td>
                ))}
                {actions && <td className="px-4 py-2 align-top">
                  <div className="flex justify-end gap-1 whitespace-nowrap">
                    {actions(row)}
                  </div>
                </td>}
              </tr>
              </Fragment>
            ))}
            {!visible.length && (
              <tr>
                <td
                  colSpan={columns.length + (actions ? 1 : 0)}
                  className="px-4 py-12 text-center text-slate-500"
                >
                  {loading
                    ? "Loading records…"
                    : "No records match the current filters."}
                </td>
              </tr>
            )}
          </tbody>
          {footer&&<tfoot className="border-t-2 border-slate-300 bg-slate-50 font-semibold">{footer}</tfoot>}
        </table>
      </div>
      {paginate && <div className="flex flex-wrap items-center justify-between gap-3 border-t border-slate-100 px-4 py-3 text-sm text-slate-500">
        <span role="status">
          Showing {visible.length ? (current - 1) * size + 1 : 0}–
          {visible.length ? Math.min((current - 1) * size + visible.length, total) : 0} of {total}
        </span>
        <div className="flex items-center gap-2">
          <button
            type="button"
            className="min-h-10 rounded-lg border px-3 disabled:opacity-40"
            disabled={loading || current === 1}
            onClick={() => { setPage(current - 1); remember("page", String(current - 1)); }}
          >
            Previous
          </button>
          <span>
            {current} / {count}
          </span>
          <button
            type="button"
            className="min-h-10 rounded-lg border px-3 disabled:opacity-40"
            disabled={loading || current >= count}
            onClick={() => { setPage(current + 1); remember("page", String(current + 1)); }}
          >
            Next
          </button>
        </div>
      </div>}
    </div>
  );
}
export function GridAction({
  label,
  kind = "view",
  onClick,
}: {
  label: string;
  kind?: "view" | "edit";
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onClick={onClick}
      className="grid min-h-11 min-w-11 place-items-center rounded-lg text-slate-500 hover:bg-blue-50 hover:text-blue-700 focus-visible:outline-2 focus-visible:outline-blue-500"
    >
      <svg
        aria-hidden="true"
        width="18"
        height="18"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.8"
      >
        {kind === "edit" ? (
          <>
            <path d="m16 3 5 5-12 12-6 1 1-6Z" />
            <path d="m14 5 5 5" />
          </>
        ) : (
          <>
            <path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7S2 12 2 12Z" />
            <circle cx="12" cy="12" r="3" />
          </>
        )}
      </svg>
    </button>
  );
}
export function GridStatus({ value }: { value: string }) {
  return (
    <span
      className={`inline-block whitespace-nowrap rounded-full px-2.5 py-1 text-xs font-semibold ${["Active", "approved"].includes(value) ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-600"}`}
    >
      {value}
    </span>
  );
}
