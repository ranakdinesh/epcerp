"use client";

import { Children, cloneElement, isValidElement, useEffect, useId, useRef, useState, type ComponentProps, type ReactElement, type ReactNode, type SubmitEvent } from "react";

/** Shared feedback for native constraints. Business rules pass an explicit error. */
type Control = HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement;
export function controlError(control: Control): string {
  if (!control.willValidate) return "";
  const labelNode = control.labels?.[0]?.cloneNode(true) as HTMLElement | undefined;
  labelNode?.querySelectorAll('input,select,textarea,[data-field-error],[data-label-ignore],[role="listbox"],[role="menu"]').forEach(node => node.remove());
  const label = control.getAttribute("aria-label")?.trim() || labelNode?.textContent?.replaceAll("*", "").trim() || "This field";
  if (control.required && !control.value.trim()) return `${label} is required.`;
  const validity = control.validity;
  if (validity.typeMismatch) return control instanceof HTMLInputElement && control.type === "email" ? "Enter a valid email address." : "Enter a valid URL, including https://.";
  if (validity.tooShort) return `Enter at least ${(control as HTMLInputElement).minLength} characters.`;
  if (validity.tooLong) return `Enter no more than ${(control as HTMLInputElement).maxLength} characters.`;
  if (validity.rangeUnderflow) return `Enter a value of at least ${(control as HTMLInputElement).min}.`;
  if (validity.rangeOverflow) return `Enter a value no greater than ${(control as HTMLInputElement).max}.`;
  if (validity.stepMismatch) return "Enter a value matching the allowed increment.";
  if (validity.patternMismatch) return control.title || "Enter a value in the required format.";
  if (validity.badInput) return "Enter a valid number.";
  return validity.valid ? "" : control.validationMessage;
}

type Feedback = { error?: string };
function useFeedback(error?: string) {
  const [message, setMessage] = useState("");
  const id = useId();
  return { id, message: error || message, update: (target: Control, serverMessage?: string) => setMessage(serverMessage ?? controlError(target)) };
}
function ErrorText({ id, children }: { id: string; children: string }) {
  return children ? <span id={id} aria-live="polite" className="mt-1 block text-sm font-normal text-red-600" data-field-error>{children}</span> : null;
}
export function ValidatedInput({ error, onChange, onBlur, onInvalid, ...props }: ComponentProps<"input"> & Feedback) {
  const feedback = useFeedback(error);
  const id = `${feedback.id}-error`;
  return <><input {...props} aria-invalid={Boolean(feedback.message) || props["aria-invalid"]} aria-describedby={[props["aria-describedby"], feedback.message ? id : ""].filter(Boolean).join(" ") || undefined}
    onChange={event => { event.currentTarget.setCustomValidity(""); feedback.update(event.currentTarget); onChange?.(event); }}
    onBlur={event => { feedback.update(event.currentTarget); onBlur?.(event); }}
    onInvalid={event => { event.preventDefault(); feedback.update(event.currentTarget, event.nativeEvent instanceof CustomEvent && typeof event.nativeEvent.detail === "string" ? event.nativeEvent.detail : undefined); onInvalid?.(event); }}
  /><ErrorText id={id}>{feedback.message}</ErrorText></>;
}
export function ValidatedSelect({ error, onChange, onBlur, onInvalid, ...props }: ComponentProps<"select"> & Feedback) {
  const feedback = useFeedback(error); const id = `${feedback.id}-error`;
  return <><select {...props} aria-invalid={Boolean(feedback.message) || props["aria-invalid"]} aria-describedby={[props["aria-describedby"], feedback.message ? id : ""].filter(Boolean).join(" ") || undefined}
    onChange={event => { event.currentTarget.setCustomValidity(""); feedback.update(event.currentTarget); onChange?.(event); }}
    onBlur={event => { feedback.update(event.currentTarget); onBlur?.(event); }}
    onInvalid={event => { event.preventDefault(); feedback.update(event.currentTarget, event.nativeEvent instanceof CustomEvent && typeof event.nativeEvent.detail === "string" ? event.nativeEvent.detail : undefined); onInvalid?.(event); }}
  /><ErrorText id={id}>{feedback.message}</ErrorText></>;
}
export function ValidatedTextarea({ error, onChange, onBlur, onInvalid, ...props }: ComponentProps<"textarea"> & Feedback) {
  const feedback = useFeedback(error); const id = `${feedback.id}-error`;
  return <><textarea {...props} aria-invalid={Boolean(feedback.message) || props["aria-invalid"]} aria-describedby={[props["aria-describedby"], feedback.message ? id : ""].filter(Boolean).join(" ") || undefined}
    onChange={event => { event.currentTarget.setCustomValidity(""); feedback.update(event.currentTarget); onChange?.(event); }}
    onBlur={event => { feedback.update(event.currentTarget); onBlur?.(event); }}
    onInvalid={event => { event.preventDefault(); feedback.update(event.currentTarget, event.nativeEvent instanceof CustomEvent && typeof event.nativeEvent.detail === "string" ? event.nativeEvent.detail : undefined); onInvalid?.(event); }}
  /><ErrorText id={id}>{feedback.message}</ErrorText></>;
}
function requiresValue(children: ReactNode): boolean {
  return Children.toArray(children).some(child => {
    if (!isValidElement(child)) return false;
    const props = child.props as { required?: boolean; children?: ReactNode };
    return Boolean(props.required) || requiresValue(props.children);
  });
}
function containsStar(children: ReactNode): boolean {
  return Children.toArray(children).some(child => typeof child === "string" ? child.includes("*") : isValidElement(child) && containsStar((child.props as { children?: ReactNode }).children));
}
function redStars(children: ReactNode): ReactNode {
  return Children.map(children, child => {
    if (typeof child === "string" && child.includes("*")) return child.split("*").map((part, index) => <span key={index}>{index > 0 ? <RequiredMark /> : null}{part}</span>);
    if (isValidElement(child) && typeof child.type === "string" && child.type !== "option") {
      const element = child as ReactElement<{children?: ReactNode}>;
      return element.props.children ? cloneElement(element, {}, redStars(element.props.children)) : child;
    }
    return child;
  });
}
export function RequiredMark() { return <span aria-hidden="true" className="text-red-600"> *</span>; }
export function ValidatedLabel({ children, required: explicitRequired, ...props }: ComponentProps<"label"> & { required?: boolean }) {
  const required = explicitRequired || requiresValue(children);
  let marked = false;
  const content = required && !containsStar(children) ? Children.map(children, child => {
    if (!marked && ((typeof child === "string" && child.trim()) || (isValidElement(child) && ["span", "strong"].includes(String(child.type))))) {
      marked = true;
      return <>{child}<RequiredMark /></>;
    }
    return child;
  }) : redStars(children);
  return <label {...props}>{content}</label>;
}
export function ValidatedForm({ onSubmit, ...props }: ComponentProps<"form">) {
  function submit(event: SubmitEvent<HTMLFormElement>) {
    const form = event.currentTarget;
    // Native validation reports every invalid control; messages stay next to inputs.
    const controls = Array.from(form.elements).filter((element): element is Control => element instanceof HTMLInputElement || element instanceof HTMLSelectElement || element instanceof HTMLTextAreaElement);
    const invalid = controls.filter(control => controlError(control));
    const emptyGroups = Array.from(form.querySelectorAll<HTMLElement>("[data-required-selection]")).filter(group => !group.closest("fieldset:disabled") && (group.dataset.selectionCount !== undefined ? Number(group.dataset.selectionCount) === 0 : !group.querySelector("input:checked")));
    if (invalid.length || emptyGroups.length) {
      emptyGroups.forEach(group => group.dispatchEvent(new Event("invalid", {cancelable: true})));
      invalid.forEach(control => control.dispatchEvent(new Event("invalid", { cancelable: true })));
      event.preventDefault();
      (invalid[0] ?? emptyGroups[0].querySelector<HTMLElement>("input,button") ?? emptyGroups[0]).focus();
      return;
    }
    onSubmit?.(event);
  }
  return <form {...props} noValidate onSubmit={submit} />;
}

/** Required multi-selection groups participate in the same form validation pass. */
export function RequiredSelection({ as = "fieldset", selectionCount, onInvalid, children, className }: { as?: "fieldset" | "div"; selectionCount?: number; onInvalid: () => void; children: ReactNode; className?: string }) {
  const ref = useRef<HTMLElement | null>(null);
  useEffect(() => {
    const element = ref.current;
    element?.addEventListener("invalid", onInvalid);
    return () => element?.removeEventListener("invalid", onInvalid);
  }, [onInvalid]);
  return as === "div"
    ? <div ref={element => { ref.current = element; }} className={className} data-required-selection data-selection-count={selectionCount}>{children}</div>
    : <fieldset ref={element => { ref.current = element; }} className={className} data-required-selection data-selection-count={selectionCount}>{children}</fieldset>;
}

/** One validation message for a required radio group; choosing an option clears it. */
export function ValidatedRadioGroup({ name, label, options, value, onChange, disabled = false, required = true, className = "" }: {
  name: string;
  label: string;
  options: readonly { value: string; label: string; ariaLabel?: string }[];
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  required?: boolean;
  className?: string;
}) {
  const id = useId();
  const [message, setMessage] = useState("");
  return <fieldset className={className} disabled={disabled} onInvalid={event => { event.preventDefault(); setMessage("Choose one option."); }}>
    <legend className="sr-only">{label}</legend>
    <div className="flex flex-wrap gap-3">{options.map(option => <label key={option.value} style={{ display: "inline-flex", alignItems: "center", gap: "0.5rem", minHeight: "44px" }}>
      <input type="radio" name={name} required={required} value={option.value} checked={value === option.value} aria-label={option.ariaLabel} aria-invalid={Boolean(message)} aria-describedby={message ? id : undefined} style={{ width: "1rem", height: "1rem", minHeight: 0, padding: 0, margin: 0, flexShrink: 0 }} onChange={() => { setMessage(""); onChange(option.value); }} />
      {option.label}
    </label>)}</div>
    <ErrorText id={id}>{message}</ErrorText>
  </fieldset>;
}
