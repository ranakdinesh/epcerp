"use client";
import {
  ChevronDownIcon,
  DocumentTextIcon,
  EllipsisVerticalIcon,
  PencilSquareIcon,
  PlusIcon,
} from "@heroicons/react/24/outline";
import {
  ValidatedInput,
  ValidatedLabel,
  ValidatedTextarea,
} from "@/components/ui/validated-controls";
import { createRecordId } from "@/lib/record-id";
import { useState, useRef, useId } from "react";
import type { Item, Line } from "./purchase-bo-workspace";

const blank = (): Line => ({
  id: createRecordId(),
  item_id: null,
  quantity: null,
  required_date: null,
  reference: "",
  notes: "",
  item_snapshot: {},
  demand_id: null,
});
export function BOLineGrid({
  lines,
  items,
  editable,
  busy,
  reserved,
  onChange,
}: {
  lines: Line[];
  items: Item[];
  editable: boolean;
  busy: boolean;
  reserved: Set<string>;
  onChange: (lines: Line[]) => void;
}) {
  const [editing, setEditing] = useState<string | null>(() =>
    editable
      ? (lines.find((l) => !l.item_id || !l.quantity || !l.required_date)?.id ??
        null)
      : null,
  );
  const [lineErrors, setLineErrors] = useState<Record<string, string>>({});
  const current = lines.find((l) => l.id === editing);
  const selected = (line: Line) =>
    items.find((i) => i.id === line.item_id) ?? line.item_snapshot;
  function patch(values: Partial<Line>) {
    onChange(lines.map((l) => (l.id === editing ? { ...l, ...values } : l)));
    setLineErrors({});
  }
  function validate() {
    if (!current) return true;
    const errors: Record<string, string> = {};
    const quantity = Number(current.quantity);
    const unit = (selected(current).uom ?? "").toLowerCase();
    if (!current.item_id) errors.item = "Select an item.";
    else if (
      reserved.has(current.item_id) ||
      lines.some(
        (line) => line.id !== current.id && line.item_id === current.item_id,
      )
    )
      errors.item =
        "This item is already added. Edit its existing row instead.";
    if (!Number.isFinite(quantity) || quantity <= 0)
      errors.quantity = "Enter a positive quantity.";
    else if (
      ["nos", "each", "pcs", "piece", "pieces", "number", "numbers"].includes(
        unit,
      ) &&
      !Number.isInteger(quantity)
    )
      errors.quantity = "This unit requires a whole-number quantity.";
    if (!current.required_date)
      errors.required_date = "Set the required-at-site date.";
    setLineErrors(errors);
    return Object.keys(errors).length === 0;
  }
  function add() {
    if (!validate()) return;
    const line = blank();
    onChange([...lines, line]);
    setEditing(line.id);
    setLineErrors({});
  }
  const rows = lines.filter((l) => l.id !== editing);
  return (
    <div className="bo-line-grid">
      <div className="bo-grid-heading">
        <div>
          <h3>
            Items{" "}
            <span className="bo-count">
              {lines.filter((l) => l.item_id).length}
            </span>
          </h3>
        </div>
        {editable && (
          <button
            className="primary"
            disabled={busy || lines.length >= 100}
            onClick={add}
          >
            <PlusIcon aria-hidden="true" /> Add item
          </button>
        )}
      </div>
      {rows.length > 0 && (
        <div
          className="bo-table-scroll"
          role="region"
          aria-label="BO item register"
          tabIndex={0}
        >
          <table
            aria-label="BO line items"
            className="vtpl-data-table bo-summary-table"
          >
            <colgroup>
              {[22, 27, 8, 6, 11, 8, 6, 12].map((width, index) => (
                <col key={index} style={{ width: `${width}%` }} />
              ))}
            </colgroup>
            <thead>
              <tr>
                <th>Item / SKU</th>
                <th>Technical Specifications</th>
                <th>Quantity</th>
                <th>Unit</th>
                <th>Required date</th>
                <th>Reference</th>
                <th>Notes</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((line, index) => {
                const item = selected(line);
                return (
                  <tr key={line.id}>
                    <td>
                      <strong className="bo-grid-name">
                        {item.description?.split(/[·\n]/)[0].trim() ||
                          "Item not selected"}
                      </strong>
                      <span className="bo-grid-sku">
                        {item.sku || "SKU not selected"}
                      </span>
                    </td>
                    <td>
                      <span
                        className="bo-grid-description"
                        style={{ whiteSpace: "pre-wrap" }}
                        title={item.description}
                      >
                        {item.description || "Incomplete draft"}
                      </span>
                    </td>
                    <td className="bo-number">{line.quantity ?? "—"}</td>
                    <td>{item.uom || "—"}</td>
                    <td>
                      {line.required_date
                        ? line.required_date.split("-").reverse().join("/")
                        : "Not set"}
                    </td>
                    <td>{line.reference || "—"}</td>
                    <td>
                      {line.notes ? (
                        <NoteDetail
                          notes={line.notes}
                          label={`Notes for ${item.sku || `item ${index + 1}`}`}
                        />
                      ) : (
                        <span className="bo-muted">—</span>
                      )}
                    </td>
                    <td>
                      <div className="bo-row-actions">
                        {editable ? (
                          <>
                            <button
                              disabled={busy}
                              aria-label={`Edit ${item.sku || `item ${index + 1}`}`}
                              onClick={() => {
                                if (
                                  current &&
                                  !current.item_id &&
                                  !current.quantity &&
                                  !current.required_date &&
                                  !current.reference.trim() &&
                                  !current.notes.trim()
                                ) {
                                  onChange(
                                    lines.filter(
                                      (row) => row.id !== current.id,
                                    ),
                                  );
                                } else if (!validate()) return;
                                setEditing(line.id);
                                setLineErrors({});
                              }}
                            >
                              <PencilSquareIcon aria-hidden="true" /> Edit
                            </button>
                            <RemoveItemMenu
                              label={`More actions for ${item.sku || `item ${index + 1}`}`}
                              published={!!line.demand_id}
                              busy={busy}
                              onRemove={() =>
                                onChange(
                                  lines.filter((row) => row.id !== line.id),
                                )
                              }
                            />
                          </>
                        ) : (
                          <span className="bo-muted">Read-only</span>
                        )}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
      {current && editable && (
        <fieldset className="bo-entry" disabled={busy}>
          <div className="bo-entry-heading">
            <h4>{current.item_id ? "Edit item" : "Add an item"}</h4>
            <button
              disabled={!!current.demand_id}
              aria-label="Remove current item"
              onClick={() => {
                onChange(lines.filter((l) => l.id !== editing));
                setEditing(null);
                setLineErrors({});
              }}
            >
              Remove
            </button>
          </div>
          <div className="bo-entry-fields">
            <ValidatedLabel required className="bo-entry-item">
              Item name / SKU
              {current.demand_id ? (
                <ValidatedInput
                  aria-label="Published item SKU"
                  readOnly
                  value={`${selected(current).description} · ${selected(current).sku}`}
                />
              ) : (
                <ItemSearch
                  key={current.id}
                  value={
                    selected(current).description
                      ? `${selected(current).description} · ${selected(current).sku}`
                      : ""
                  }
                  items={items}
                  used={
                    new Set([
                      ...reserved,
                      ...lines
                        .filter((l) => l.id !== current.id && l.item_id)
                        .map((l) => l.item_id!),
                    ])
                  }
                  onSelect={(item) =>
                    patch({
                      item_id: item?.id ?? null,
                      item_snapshot: item ?? {},
                      quantity: null,
                    })
                  }
                  onError={(message) =>
                    setLineErrors((current) => ({ ...current, item: message }))
                  }
                />
              )}
              {lineErrors.item ? (
                <span className="mt-1 block text-sm text-red-600" role="alert">
                  {lineErrors.item}
                </span>
              ) : null}
            </ValidatedLabel>
            <ValidatedLabel
              className="bo-entry-spec"
              style={{ gridColumn: "1 / -1" }}
            >
              Technical specifications & scope details
              <ValidatedTextarea
                aria-label="Item technical specification"
                rows={3}
                value={selected(current).description ?? ""}
                placeholder="Enter complete technical specifications, size/rating, standards, and supply scope..."
                onChange={(e) =>
                  patch({
                    item_snapshot: {
                      ...selected(current),
                      description: e.target.value,
                    },
                  })
                }
              />
            </ValidatedLabel>
            <ValidatedLabel>
              Quantity
              <ValidatedInput
                required
                error={lineErrors.quantity}
                aria-label="Item quantity"
                type="number"
                min="0.001"
                step="0.001"
                value={current.quantity ?? ""}
                onChange={(e) => patch({ quantity: e.target.value })}
              />
            </ValidatedLabel>
            <ValidatedLabel>
              Unit
              <ValidatedInput
                aria-label="Item unit"
                readOnly
                value={selected(current).uom ?? ""}
                placeholder="Automatic"
              />
            </ValidatedLabel>
            <ValidatedLabel>
              Required at site
              <ValidatedInput
                required
                error={lineErrors.required_date}
                aria-label="Item required date"
                type="date"
                value={current.required_date ?? ""}
                onChange={(e) => patch({ required_date: e.target.value })}
              />
            </ValidatedLabel>
            <ValidatedLabel>
              BOQ / drawing reference (optional)
              <ValidatedInput
                aria-label="Item reference"
                maxLength={500}
                value={current.reference}
                placeholder="e.g. BOQ line 12 / E-04"
                onChange={(e) => patch({ reference: e.target.value })}
              />
            </ValidatedLabel>
            <ValidatedLabel className="bo-entry-notes">
              Notes (optional)
              <ValidatedTextarea
                aria-label="Item notes"
                rows={2}
                maxLength={4000}
                value={current.notes}
                placeholder="Delivery instructions or remarks for this item"
                onChange={(e) => patch({ notes: e.target.value })}
              />
            </ValidatedLabel>
          </div>
          <div className="bo-entry-actions">
            <small>Row changes are included when you save the draft.</small>
            <button
              className="primary"
              onClick={() => {
                if (validate()) setEditing(null);
              }}
            >
              Done
            </button>
            <button onClick={add} disabled={lines.length >= 100}>
              Done & add next
            </button>
          </div>
        </fieldset>
      )}
      {!current && !lines.length && <p className="bo-empty">No items added.</p>}
    </div>
  );
}
export function ItemSearch({
  required=false,
  error,
  inputName="text",
  value,
  items,
  used,
  onSelect,
  onError,
}: {
  required?:boolean;
  error?:string;
  inputName?:string;
  value: string;
  items: Item[];
  used: Set<string>;
  onSelect: (item: Item | undefined) => void;
  onError: (text: string) => void;
}) {
  const [text, setText] = useState(value);
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState<string | null>(null);
  const input = useRef<HTMLInputElement>(null);
  const list = useRef<HTMLDivElement>(null);
  const id = useId();
  const matches = items.filter((item) =>
    `${item.description} ${item.sku}`
      .toLowerCase()
      .includes(query.trim().toLowerCase()),
  );
  const available = matches.filter((item) => !used.has(item.id));
  function choose(item: Item) {
    if (used.has(item.id)) return;
    setText(`${item.description} · ${item.sku}`);
    setQuery("");
    setOpen(false);
    setActive(null);
    onSelect(item);
  }
  function move(direction: number) {
    setOpen(true);
    if (!available.length) return;
    const index = available.findIndex((item) => item.id === active);
    const next =
      available[
        index < 0
          ? direction > 0
            ? 0
            : available.length - 1
          : (index + direction + available.length) % available.length
      ];
    setActive(next.id);
    requestAnimationFrame(() => {
      list.current
        ?.querySelector('[aria-selected="true"]')
        ?.scrollIntoView({ block: "nearest" });
    });
  }
  return (
    <div
      className="bo-item-picker"
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false);
      }}
    >
      <div className="bo-picker-control">
        <div className="bo-picker-input">
        <ValidatedInput
          name={inputName}
          required={required}
          error={error}
          ref={input}
          role="combobox"
          aria-label="Item name or SKU"
          aria-autocomplete="list"
          aria-expanded={open}
          aria-controls={id}
          aria-activedescendant={open && active ? `${id}-${active}` : undefined}
          autoComplete="off"
          value={text}
          placeholder="Search by name or SKU"
          onFocus={() => {
            setOpen(true);
            setQuery("");
            setActive(null);
          }}
          onChange={(event) => {
            const value = event.target.value;
            setText(value);
            setQuery(value);
            setOpen(true);
            setActive(null);
            onSelect(undefined);
            const duplicate = items.find(
              (item) => used.has(item.id) && item.sku === value,
            );
            if (duplicate)
              onError(
                "This item is already added. Edit its existing row instead.",
              );
          }}
          onKeyDown={(event) => {
            if (event.key === "ArrowDown" || event.key === "ArrowUp") {
              event.preventDefault();
              move(event.key === "ArrowDown" ? 1 : -1);
            } else if (event.key === "Enter" && open) {
              event.preventDefault();
              const item = available.find((item) => item.id === active);
              if (item) choose(item);
            } else if (event.key === "Escape") {
              event.preventDefault();
              setOpen(false);
              setActive(null);
            }
          }}
        />
        </div>
        <button
          type="button"
          aria-label="Show Item Bank dropdown"
          aria-expanded={open}
          aria-controls={id}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => {
            if (open) {
              setOpen(false);
              return;
            }
            input.current?.focus();
            setQuery("");
            setActive(null);
            setOpen(true);
          }}
        >
          <ChevronDownIcon aria-hidden="true" />
        </button>
      </div>
      {open && (
        <div className="bo-picker-menu" data-label-ignore>
          <div ref={list} id={id} role="listbox" aria-label="Item Bank items">
            {matches.map((item) => (
              <div
                key={item.id}
                id={`${id}-${item.id}`}
                role="option"
                aria-selected={active === item.id}
                aria-disabled={used.has(item.id)}
                className="bo-picker-option"
                onMouseDown={(event) => event.preventDefault()}
                onClick={(event) => {
                  event.preventDefault();
                  choose(item);
                }}
              >
                <strong>{item.description.split(" · ")[0]}</strong>
                {item.description.includes(" · ")&&<span className="bo-picker-specs" title={item.description}>{item.description.split(" · ").slice(1).join(" · ")}</span>}
                <span className="bo-picker-sku" title={item.sku}>{item.sku}</span>
                <small>
                  {item.uom}
                  {used.has(item.id) ? " · Already added" : ""}
                </small>
              </div>
            ))}
          </div>
          {!matches.length && (
            <p role="status">
              {items.length
                ? "No matching items. Search another name or SKU."
                : "No items are available in the Item Bank."}
            </p>
          )}
          {!!matches.length && !available.length && (
            <p role="status">
              These items have already been added. Edit their existing rows.
            </p>
          )}
        </div>
      )}
    </div>
  );
}

function NoteDetail({ notes, label }: { notes: string; label: string }) {
  const [open, setOpen] = useState(false);
  const tip = useRef<HTMLSpanElement>(null),
    trigger = useRef<HTMLButtonElement>(null);
  const id = useId();
  function show() {
    const box = trigger.current?.getBoundingClientRect();
    if (box && tip.current) {
      tip.current.style.left = `${Math.max(16, Math.min(box.left, window.innerWidth - 316))}px`;
      tip.current.style.top = `${Math.max(16, Math.min(box.bottom + 8, window.innerHeight - 256))}px`;
      tip.current.showPopover();
      setOpen(true);
    }
  }
  function hide() {
    tip.current?.hidePopover();
    setOpen(false);
  }
  return (
    <span className="bo-note-wrap" onMouseEnter={show} onMouseLeave={hide}>
      <button
        ref={trigger}
        className="bo-note-button"
        aria-label={label}
        aria-describedby={open ? id : undefined}
        aria-expanded={open}
        onFocus={show}
        onBlur={hide}
        onClick={show}
        onKeyDown={(e) => {
          if (e.key === "Escape") hide();
        }}
      >
        <DocumentTextIcon aria-hidden="true" />
      </button>
      <span
        ref={tip}
        id={id}
        popover="manual"
        role="tooltip"
        className="bo-note-tooltip"
      >
        {notes}
      </span>
    </span>
  );
}

function RemoveItemMenu({
  label,
  published,
  busy,
  onRemove,
}: {
  label: string;
  published: boolean;
  busy: boolean;
  onRemove: () => void;
}) {
  const trigger = useRef<HTMLButtonElement>(null);
  const menu = useRef<HTMLDivElement>(null);
  const id = useId();
  const [open, setOpen] = useState(false);
  function show() {
    const box = trigger.current?.getBoundingClientRect();
    if (!box || !menu.current) return;
    menu.current.style.left = `${Math.max(12, Math.min(box.right - 190, window.innerWidth - 202))}px`;
    menu.current.style.top = `${Math.min(box.bottom + 6, window.innerHeight - 160)}px`;
    menu.current.showPopover();
  }
  return (
    <span className="bo-row-menu">
      <button
        ref={trigger}
        type="button"
        aria-label={label}
        aria-expanded={open}
        aria-controls={id}
        onClick={() => (open ? menu.current?.hidePopover() : show())}
      >
        <EllipsisVerticalIcon aria-hidden="true" />
      </button>
      <div
        ref={menu}
        id={id}
        popover="auto"
        className="bo-row-popover"
        onToggle={(event) => setOpen(event.newState === "open")}
      >
        <button
          type="button"
          className="bo-remove"
          disabled={busy || published}
          onClick={onRemove}
        >
          Remove item
        </button>
        {published ? <small>Published item cannot be removed.</small> : null}
      </div>
    </span>
  );
}
