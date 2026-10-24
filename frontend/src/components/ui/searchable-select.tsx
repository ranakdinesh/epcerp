"use client";

import { useId, useRef, useState } from "react";
import { ValidatedInput, ValidatedLabel } from "./validated-controls";
import { prefixOptions } from "./prefix-options";

type Option = { id: string; name: string };
export function SearchableSelect({ label, name, value, options, onChange, disabled = false, required = false, hint }: {
  label: string; name: string; value: string; options: Option[];
  onChange: (value: string) => void; disabled?: boolean; required?: boolean; hint?: string;
}) {
  const id = useId();
  const input = useRef<HTMLInputElement>(null);
  const [typed, setTyped] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [highlight, setHighlight] = useState(0);
  const filtered = prefixOptions(options, typed ?? "");
  function choose(option: Option) {
    onChange(option.name); setTyped(null); setOpen(false);
    input.current?.setCustomValidity("");
  }
  return <div className="relative min-w-0 text-sm font-bold text-slate-700" onBlur={event => {
    if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false);
  }}>
    <ValidatedLabel htmlFor={id} required={required}>{label}</ValidatedLabel>
    <div className="relative">
      <ValidatedInput ref={input} id={id} name={name} role="combobox" aria-autocomplete="list"
        aria-expanded={open} aria-controls={`${id}-options`} aria-activedescendant={open && filtered[highlight] ? `${id}-${filtered[highlight].id}` : undefined}
        autoComplete="off" required={required} disabled={disabled} value={typed ?? value}
        onFocus={() => { setOpen(true); setHighlight(0); }}
        onChange={event => {
          const query = event.target.value; setTyped(query); setOpen(true); setHighlight(0);
          const exact = options.find(option => option.name.toLowerCase() === query.trim().toLowerCase());
          event.target.setCustomValidity(query && !exact ? `Select ${label.toLowerCase()} from the dropdown.` : "");
          if (exact) onChange(exact.name);
          if (!query) onChange("");
        }}
        onKeyDown={event => {
          if (event.key === "ArrowDown" || event.key === "ArrowUp") {
            event.preventDefault(); setOpen(true);
            setHighlight(current => Math.max(0, Math.min(filtered.length - 1, current + (event.key === "ArrowDown" ? 1 : -1))));
          } else if (event.key === "Enter" && open && filtered[highlight]) {
            event.preventDefault(); choose(filtered[highlight]);
          } else if (event.key === "Escape" && open) { event.preventDefault(); event.stopPropagation(); setOpen(false); }
        }} placeholder={`Select ${label.toLowerCase()} or type to search`} className="field pr-10" />
      <button type="button" aria-label={`Show ${label.toLowerCase()} options`} disabled={disabled}
        onClick={() => { setTyped(null); setHighlight(0); input.current?.focus(); setOpen(true); }}
        className="absolute right-0 top-0 grid h-11 w-10 place-items-center text-slate-500">▾</button>
    </div>
    {open && !disabled ? <ul id={`${id}-options`} role="listbox" aria-label={`${label} options`}
      className="absolute z-50 mt-1 max-h-56 w-full overflow-y-auto rounded-xl border border-slate-200 bg-white p-1 shadow-xl">
      {filtered.map((option, index) => <li id={`${id}-${option.id}`} role="option" aria-selected={option.name === value} key={option.id}>
        <button type="button" tabIndex={-1} onMouseDown={event => event.preventDefault()} onClick={() => choose(option)}
          className={`min-h-11 w-full rounded-lg px-3 py-2 text-left text-sm hover:bg-blue-50 ${highlight === index ? "bg-blue-50 text-blue-800" : "text-slate-700"}`}>{option.name}</button>
      </li>)}
      {!filtered.length ? <li role="status" className="p-3 text-sm font-normal text-slate-500">No matching options.</li> : null}
    </ul> : null}
    {hint ? <span className="mt-1 block text-xs font-normal text-slate-500">{hint}</span> : null}
  </div>;
}
