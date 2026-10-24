"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { ValidatedForm, ValidatedInput, ValidatedLabel } from "@/components/ui/validated-controls";
import { displayServerErrors, serverErrorMessage } from "@/components/ui/server-field-errors";

export function UsernameEditor({ disabled = false }: { disabled?: boolean }) {
  const user = useAuthenticatedUser();
  const router = useRouter();
  const [username, setUsername] = useState(user.username);
  const [fieldError, setFieldError] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [checking, setChecking] = useState(false);
  const [saving, setSaving] = useState(false);
  const [available, setAvailable] = useState<boolean | null>(null);
  const pending = useRef<AbortController | null>(null);
  useEffect(() => () => pending.current?.abort(), []);

  async function check() {
    const value = username.trim();
    if (!/^[a-zA-Z0-9._-]{3,64}$/.test(value)) return;
    pending.current?.abort();
    const controller = new AbortController();
    pending.current = controller;
    setChecking(true);
    setError("");
    try {
      const response = await fetch(`/api/auth/me/username?username=${encodeURIComponent(value)}`, { signal: controller.signal });
      const body = await response.json();
      if (controller.signal.aborted) return;
      if (!response.ok) throw new Error(serverErrorMessage(body, "Unable to check username availability."));
      setAvailable(body.available);
      setFieldError(body.available ? "" : `${value} already exists. Please choose a different username.`);
    } catch (err) {
      if (!controller.signal.aborted) setError(err instanceof Error ? err.message : "Unable to check username availability.");
    } finally { if (!controller.signal.aborted) setChecking(false); }
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (saving) return;
    const form = event.currentTarget;
    pending.current?.abort();
    setChecking(false); setSaving(true); setError(""); setFieldError(""); setNotice("");
    try {
      const response = await fetch("/api/auth/me/username", { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username: username.trim(), expected_username: user.username }) });
      if (!response.ok) {
        const body = await response.json();
        const message = serverErrorMessage(body, "Unable to update your username.");
        if (response.status === 409 && message.includes("already exists")) { setFieldError(message); setAvailable(false); return; }
        if (response.status === 422 && displayServerErrors(form, body)) return;
        throw new Error(message);
      }
      setUsername(username.trim()); setAvailable(true); setNotice("Username updated. Use your new username or email to sign in."); router.refresh();
    } catch (err) { setError(err instanceof Error ? err.message : "Unable to update your username."); }
    finally { setSaving(false); }
  }

  return <ValidatedForm onSubmit={submit} className="go-personal-form go-username-form">
    <div className="go-username-row"><ValidatedLabel>Username<ValidatedInput name="username" autoComplete="username" value={username} minLength={3} maxLength={64} pattern="[a-zA-Z0-9._-]+" title="Use 3 to 64 letters, numbers, dots, underscores or hyphens." required disabled={disabled || saving} error={fieldError} onChange={event => { pending.current?.abort(); setChecking(false); setUsername(event.target.value); setAvailable(null); setFieldError(""); setError(""); setNotice(""); }} onBlur={() => void check()} /></ValidatedLabel>
    <button type="submit" disabled={disabled || saving || available === false || username.trim() === user.username}>{saving ? "Updating…" : "Update username"}</button></div>
    <p className="go-username-hint">Your sign-in name. Use 3–64 letters, numbers, dots, underscores or hyphens.</p>
    <p role="status" className="go-username-feedback">{checking ? "Checking availability…" : notice || (available && username.trim() !== user.username ? "Username is available." : "")}</p>
    {error && <p role="alert" className="go-form-error">{error}</p>}
  </ValidatedForm>;
}
