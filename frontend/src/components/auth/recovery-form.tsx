"use client";

import Link from "next/link";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { useState, useSyncExternalStore } from "react";
import { ValidatedForm, ValidatedInput, ValidatedLabel } from "@/components/ui/validated-controls";

function subscribeHash(callback: () => void) { window.addEventListener("hashchange", callback); return () => window.removeEventListener("hashchange", callback); }
function readToken() { return new URLSearchParams(window.location.hash.slice(1)).get("token") ?? ""; }
const inputClass = "mt-2 w-full rounded-xl border border-slate-300 bg-slate-50 px-4 py-3 text-sm text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-800/30";

export default function RecoveryForm({ reset = false }: { reset?: boolean }) {
  const token = useSyncExternalStore(subscribeHash, readToken, () => "");
  const [busy, setBusy] = useState(false);
  const [success, setSuccess] = useState("");
  const [error, setError] = useState("");
  const [passwordError, setPasswordError] = useState("");
  const [confirmError, setConfirmError] = useState("");
  const [emailError, setEmailError] = useState("");

  async function submit(event: React.SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const password = String(data.get("password") ?? "");
    const confirm = String(data.get("confirm") ?? "");
    setError(""); setPasswordError(""); setConfirmError(""); setEmailError("");
    if (reset && password !== confirm) {
      setConfirmError("Passwords must match."); form.querySelector<HTMLInputElement>("#recovery-confirm")?.focus(); return;
    }
    setBusy(true);
    try {
      const response = await fetch(`/api/auth/${reset ? "reset-password" : "forgot-password"}`, {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify(reset ? { token, password } : { email: String(data.get("email") ?? "").trim() }),
      });
      const result = response.status === 204 ? {} : await response.json();
      if (!response.ok) {
        if (Array.isArray(result.detail)) {
          if (!displayServerErrors(form, result)) setError("This reset link is invalid or expired. Request a new link.");
        } else if (reset && result.message === "Choose a different password.") {
          setPasswordError(result.message); form.querySelector<HTMLInputElement>("#recovery-password")?.focus();
        } else setError(result.message ?? "Unable to complete your request. Please try again.");
        return;
      }
      if (reset) {
        window.history.replaceState(null, "", "/reset-password");
        form.reset();
      }
      setSuccess(reset ? "Your password has been updated. Sign in with your new password." : result.message);
    } catch { setError("Unable to reach password recovery. Please try again."); }
    finally { setBusy(false); }
  }

  return <main className="min-h-screen bg-white px-6 py-8 font-sans text-slate-900 sm:px-10">
    {/* eslint-disable-next-line @next/next/no-img-element */}
    <img src="/vtpl-logo.svg" alt="Vishal Technopower" className="h-11 w-auto" />
    <section aria-label={reset ? "Reset password" : "Forgot password"} className="mx-auto mt-16 w-full max-w-[440px] rounded-3xl border border-slate-200 bg-gradient-to-b from-white to-slate-50 p-6 shadow-xl shadow-slate-200/50 sm:p-8">
      <h1 className="text-2xl font-extrabold tracking-tight">{reset ? "Reset password" : "Forgot password?"}</h1>
      <p className="mt-3 text-sm leading-6 text-slate-600">{reset ? "Choose a new password for your epcERP account." : "Enter the email address registered to your account. We’ll email you a link to reset your password."}</p>
      {success ? <p role="status" className="mt-6 rounded-xl bg-emerald-50 p-4 text-sm text-emerald-800">{success}</p> : reset && !token ? <p role="alert" className="mt-6 rounded-xl bg-amber-50 p-4 text-sm text-amber-900">Open the reset link from your email, or request a new link below.</p> : <ValidatedForm onSubmit={submit} className="mt-6 space-y-5">
        {reset ? <>
          <div><ValidatedLabel htmlFor="recovery-password" required>New password</ValidatedLabel>
            <ValidatedInput id="recovery-password" name="password" type="password" autoComplete="new-password" required minLength={8} maxLength={128} pattern={"(?=.*\\p{Ll})(?=.*\\p{Lu})(?=.*\\p{Nd})(?=.*[^\\p{L}\\p{N}]).{8,128}"} title="Use 8 to 128 characters including uppercase, lowercase, number and symbol." aria-describedby="password-guidance" error={passwordError} onChange={() => setPasswordError("")} className={inputClass} />
            <p id="password-guidance" className="mt-2 text-xs leading-5 text-slate-600">8–128 characters, with uppercase, lowercase, a number and a symbol.</p>
          </div>
          <div><ValidatedLabel htmlFor="recovery-confirm" required>Confirm new password</ValidatedLabel>
            <ValidatedInput id="recovery-confirm" name="confirm" type="password" autoComplete="new-password" required maxLength={128} error={confirmError} onChange={() => setConfirmError("")} className={inputClass} />
          </div>
        </> : <div><ValidatedLabel htmlFor="recovery-email" required>Registered email</ValidatedLabel>
          <ValidatedInput id="recovery-email" name="email" type="email" autoComplete="email" required maxLength={255} error={emailError} onChange={() => setEmailError("")} className={inputClass} />
        </div>}
        {error && <p role="alert" className="rounded-xl border border-red-100 bg-red-50 p-3 text-sm text-red-700">{error}</p>}
        <button disabled={busy} className="min-h-11 w-full rounded-xl bg-[#1e3a8a] px-4 py-3 text-sm font-bold text-white hover:bg-blue-900 disabled:opacity-60">{busy ? "Please wait…" : reset ? "Reset password" : "Send reset link"}</button>
      </ValidatedForm>}
      <div className="mt-6 flex flex-wrap justify-between gap-4 text-sm font-semibold text-[#1e3a8a]">
        <Link href="/login?session=recovery" className="inline-flex min-h-11 items-center hover:underline">Back to login</Link>
        {reset && !success && <Link href="/forgot-password" className="inline-flex min-h-11 items-center hover:underline">Request a new link</Link>}
      </div>
    </section>
  </main>;
}
