"use client";

import { ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";

import { useId, useRef, type ClipboardEvent } from "react";
import { indentSpacingOptions, type IndentSpacing, type IndentRun } from "@/lib/indent-content";

export function IndentParagraphs({ paragraphs, spacing = 1 }: { paragraphs: IndentRun[][]; spacing?: IndentSpacing }) {
  return paragraphs.map((runs, index) => <p key={index} style={{ lineHeight: 1.4 * spacing }} className="whitespace-pre-wrap [overflow-wrap:anywhere]">
    {runs.length ? runs.map((run, i) => <span key={i} style={{ fontWeight: run.bold ? 700 : undefined, fontStyle: run.italic ? "italic" : undefined, textDecoration: run.underline ? "underline" : undefined }}>{run.text}</span>) : <br />}
  </p>);
}

export function IndentRichEditor({ label, initial, onChange, spacing = 1, onSpacingChange }: {
  spacing?: IndentSpacing; onSpacingChange: (spacing: IndentSpacing) => void;
  label: string; initial: IndentRun[][]; onChange: (paragraphs: IndentRun[][]) => void;
}) {
  const labelId = useId();
  const editor = useRef<HTMLDivElement>(null);
  const initialParagraphs = useRef(initial);
  function read() {
    const paragraphs: IndentRun[][] = [[]];
    const next = () => paragraphs.push([]);
    function visit(node: Node, bold = false, italic = false, underline = false) {
      if (node.nodeType === Node.TEXT_NODE) {
        const text = node.textContent ?? "";
        if (text) paragraphs.at(-1)!.push({ text, ...(bold && { bold }), ...(italic && { italic }), ...(underline && { underline }) });
      }
      if (!(node instanceof HTMLElement)) return;
      const block = ["P", "DIV"].includes(node.tagName);
      if (block && paragraphs.at(-1)!.length) next();
      if (node.tagName === "BR" && node.nextSibling) next();
      node.childNodes.forEach(child => visit(child, bold || ["B", "STRONG"].includes(node.tagName) || Number(node.style.fontWeight) >= 600 || node.style.fontWeight === "bold", italic || ["I", "EM"].includes(node.tagName) || node.style.fontStyle === "italic", underline || node.tagName === "U" || node.style.textDecoration.includes("underline")));
      if (block) next();
    }
    editor.current?.childNodes.forEach(node => visit(node));
    if (paragraphs.length > 1 && !paragraphs.at(-1)?.length) paragraphs.pop();
    onChange(paragraphs);
  }
  function paste(event: ClipboardEvent<HTMLDivElement>) {
    event.preventDefault();
    document.execCommand("insertText", false, event.clipboardData.getData("text/plain"));
    read();
  }
  return <div className="space-y-2">
    <span id={labelId} className="block font-semibold">{label}</span>
    <div className="flex flex-wrap items-center gap-2">
      {([["bold", "Bold"], ["italic", "Italic"], ["underline", "Underline"], ["insertParagraph", "New paragraph"]] as const).map(([command, title]) => <button key={command} type="button"
        className="rounded border border-teal-300 px-3 py-1 text-teal-800 dark:text-teal-200"
        onMouseDown={event => event.preventDefault()} onClick={() => {
          editor.current?.focus();
          document.execCommand(command);
          read();
        }}>{title}</button>)}
      <ValidatedLabel className="flex items-center gap-2 text-sm">Line spacing
        <ValidatedSelect value={spacing} className="rounded border border-teal-300 bg-[var(--go-panel)] px-2 py-1"
          onChange={event => onSpacingChange(Number(event.target.value) as IndentSpacing)}>
          {indentSpacingOptions.map(option => <option key={option} value={option}>{option === 1 ? "Single" : option === 2 ? "Double" : option}</option>)}
        </ValidatedSelect>
      </ValidatedLabel>
    </div>
    <div ref={editor} contentEditable suppressContentEditableWarning role="textbox" aria-multiline="true"
      aria-labelledby={labelId} style={{ lineHeight: 1.4 * spacing }} className="min-h-24 max-h-80 overflow-y-auto rounded-lg border border-slate-400 bg-[var(--go-panel)] p-3"
      onInput={read} onPaste={paste}>
      <IndentParagraphs paragraphs={initialParagraphs.current} spacing={spacing} />
    </div>
  </div>;
}
