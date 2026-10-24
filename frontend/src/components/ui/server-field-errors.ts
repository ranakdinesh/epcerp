/** Preserve API field locations; never display request input or secret values. */
export class FieldValidationError extends Error {
  constructor(public readonly detail: unknown, message = "Please correct the highlighted fields.") {
    super(typeof detail === "string" ? detail : message);
  }
}
/** Keep workflow errors visible when an API field has no editable form control. */
export function serverErrorMessage(payload: unknown, fallback: string): string {
  if (!payload || typeof payload !== "object" || !("detail" in payload)) return fallback;
  const detail = (payload as {detail: unknown}).detail;
  if (typeof detail === "string") return detail;
  if (!Array.isArray(detail)) return fallback;
  const messages = detail.flatMap(issue => issue && typeof issue.msg === "string" ? [issue.msg.replace(/^Value error, /, "")] : []);
  return messages.slice(0, 5).join("; ") || fallback;
}
export function displayServerErrors(form: HTMLFormElement | undefined, payload: unknown): boolean {
  if (!form || !payload || typeof payload !== "object" || !("detail" in payload)) return false;
  const detail = (payload as {detail: unknown}).detail;
  if (!Array.isArray(detail)) return false;
  let first: HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement | undefined;
  let matched = 0;
  for (const issue of detail) {
    if (!issue || !Array.isArray(issue.loc) || typeof issue.msg !== "string") continue;
    const path = issue.loc.filter((part: unknown) => part !== "body").join(".");
    const exact = form.elements.namedItem(path);
    const leaf = path.split(".").at(-1);
    const candidates = Array.from(form.elements).filter(element => "name" in element && (element as HTMLInputElement).name === leaf);
    const control = exact ?? (candidates.length === 1 ? candidates[0] : null);
    if (!(control instanceof HTMLInputElement || control instanceof HTMLSelectElement || control instanceof HTMLTextAreaElement)) continue;
    control.setCustomValidity(issue.msg.replace(/^Value error, /, ""));
    control.dispatchEvent(new CustomEvent("invalid", {cancelable:true, detail: issue.msg.replace(/^Value error, /, "")}));
    first ??= control;
    matched++;
  }
  requestAnimationFrame(() => first?.focus());
  return matched > 0 && matched === detail.length;
}
