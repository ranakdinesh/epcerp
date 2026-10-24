"use client";
/* eslint-disable @next/next/no-img-element -- authenticated cookie-protected avatars bypass the public image optimizer */
import { createContext, useCallback, useContext, useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { UsernameEditor } from "@/components/auth/username-editor";
import { prepareProfilePhoto } from "@/lib/auth/profile-photo";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { ValidatedForm, ValidatedInput, ValidatedLabel } from "@/components/ui/validated-controls";
import { displayServerErrors } from "@/components/ui/server-field-errors";

type Contact = { first_name: string; last_name: string; mobile: string };
type Profile = { contact?: Contact; user_id: string; display_name: string; has_photo: boolean; version: number };
type Store = { profiles: Record<string, Profile>; load: (id: string) => Promise<void>; save: (profile: Profile) => void };
const ProfileContext = createContext<Store | null>(null);
export function PersonalProfileProvider({ children }: { children: ReactNode }) {
 const [profiles, setProfiles] = useState<Record<string, Profile>>({});
 const pending = useRef(new Map<string, Promise<void>>());
 const load = useCallback((id: string) => {
  if (!pending.current.has(id)) pending.current.set(id, fetch(`/api/auth/user_profile/${encodeURIComponent(id)}`).then(async r => {
   if (r.ok) { const p = await r.json() as Profile; setProfiles(v => ({ ...v, [id]: p })); }
  }).catch(() => { pending.current.delete(id); }));
  return pending.current.get(id)!;
 }, []);
 const save = useCallback((p: Profile) => setProfiles(v => ({ ...v, [p.user_id]: { ...p, contact: p.contact ?? v[p.user_id]?.contact } })), []);
 return <ProfileContext value={{ profiles, load, save }}>{children}</ProfileContext>;
}
function useProfile(id: string, active = true) {
 const store = useContext(ProfileContext);
 const load = store?.load;
 useEffect(() => { if (active && id && load) void load(id); }, [id, active, load]);
 return store?.profiles[id];
}
export function usePersonalProfile(id: string) { return useProfile(id); }
export function UserAvatar({ id, name, active = true, size = 36 }: { id: string; name: string; active?: boolean; size?: number }) {
 const p = useProfile(id, active);
 const [failedVersion, setFailedVersion] = useState<number | null>(null);
 const label = p?.display_name || name;
 return <span className="go-avatar" style={{ width: size, height: size }} aria-hidden="true">
  {p?.has_photo && failedVersion !== p.version ? <img src={`/api/auth/user_profile/${encodeURIComponent(id)}/photo?v=${p.version}`} alt="" onError={() => setFailedVersion(p.version)} /> : label.slice(0, 2).toUpperCase()}
 </span>;
}
export function UserIdentity({ id, name, active = true }: { id: string; name: string; active?: boolean }) {
 const p = useProfile(id, active);
 return <span className="go-user-identity"><UserAvatar id={id} name={name} active={active} /><span>{p?.display_name || name}</span></span>;
}
async function result(r: Response, form?: HTMLFormElement) {
 const body = r.status === 204 ? null : await r.json().catch(() => ({ detail: "Unable to process this request. Try a smaller photo or retry later." }));
 if (!r.ok) { if (form) displayServerErrors(form, body); throw new Error(typeof body?.detail === "string" ? body.detail : "Check the highlighted fields and try again."); }
 return body as Profile;
}
export function PersonalAccountDialog({ mode, close }: { mode: "profile" | "password"; close: () => void }) {
 const user = useAuthenticatedUser(); const profile = useProfile(user.id);
 const store = useContext(ProfileContext)!; const router = useRouter();
 const dialog = useRef<HTMLDialogElement>(null);
 const upload = useRef<HTMLInputElement>(null);
 const [section, setSection] = useState<"profile" | "security">(mode === "password" ? "security" : "profile");
 const [securityMode, setSecurityMode] = useState<"email" | "password">(mode === "password" ? "password" : "email");
 const [ready, setReady] = useState(false);
 const [confirmationError, setConfirmationError] = useState("");
 const [photoStage, setPhotoStage] = useState("");
 const [notice, setNotice] = useState(""); const [error, setError] = useState(""); const [busy, setBusy] = useState(false);
 const save = store.save;
 useEffect(() => {
  let active = true;
  fetch("/api/auth/user_profile/me").then(r => result(r)).then(p => { if (active) { save(p); setReady(true); } }).catch(() => { if (active) setError("Unable to load your profile. Close and reopen to retry."); });
  return () => { active = false; };
 }, [save]);
 useEffect(() => { const overflow = document.body.style.overflow; const element = dialog.current; element?.showModal(); document.body.style.overflow = "hidden"; return () => { element?.close(); document.body.style.overflow = overflow; }; }, []);
 async function run(work: () => Promise<void>) { if (busy) return; setBusy(true); setNotice(""); setError(""); try { await work(); } catch(e) { setError(e instanceof Error ? e.message : "Unable to save changes."); } finally { setBusy(false); } }
 async function details(e: FormEvent<HTMLFormElement>) { e.preventDefault(); const form = e.currentTarget; const data = new FormData(form); await run(async () => {
  store.save(await result(await fetch("/api/auth/user_profile/me", { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ display_name: data.get("display_name"), contact: { first_name: String(data.get("first_name")).trim(), last_name: String(data.get("last_name")).trim(), mobile: String(data.get("mobile")).trim() }, expected_contact: profile?.contact, version: profile?.version }) }), form)); setNotice("Your personal details have been saved."); router.refresh();
 }); }
 async function security(e: FormEvent<HTMLFormElement>, password: boolean) { e.preventDefault(); const form = e.currentTarget; const data = new FormData(form); if (password && data.get("password") !== data.get("confirmation")) { setConfirmationError("Passwords do not match."); form.querySelector<HTMLInputElement>('[name="confirmation"]')?.focus(); return; } await run(async () => {
  await result(await fetch(password ? "/api/auth/me/password" : "/api/auth/me/email", { method: password ? "POST" : "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(password ? { current_password: data.get("current_password"), password: data.get("password") } : { current_password: data.get("current_password"), email: data.get("email") }) }), form);
  // eslint-disable-next-line @next/next/no-location-assign-relative-destination -- a full navigation prevents stale dashboard refreshes after cookie removal
  form.reset(); if (password) { window.location.assign("/login?session=password-changed"); } else { setNotice("Email updated."); router.refresh(); }
 }); }
 async function photo(file?: File) { await run(async () => {
  try {
   setPhotoStage(file ? "Preparing photo…" : "Removing photo…");
   const prepared = file ? await prepareProfilePhoto(file) : undefined;
   if (prepared) setPhotoStage("Uploading photo…");
   store.save(await result(await fetch("/api/auth/user_profile/me/photo", prepared ? { method: "PUT", headers: { "Content-Type": prepared.type }, body: prepared } : { method: "DELETE" })));
   setNotice(file ? "Photo updated." : "Photo removed.");
  } finally { setPhotoStage(""); }
 }); }
 return <dialog ref={dialog} className="go-personal-dialog" aria-labelledby="personal-title" onClose={event => { /* Ignore a queued cleanup close after Strict Mode has reopened the dialog. */ if (!event.currentTarget.open) close(); }}>
  <div className="go-profile-topbar"><div><p className="go-profile-eyebrow">MY ACCOUNT</p><h2 id="personal-title">Account settings</h2></div><button type="button" className="go-icon-button" aria-label="Close personal account" onClick={() => dialog.current?.close()}>×</button></div>
  <div className="go-profile-identity">
   <UserAvatar id={user.id} name={user.username} size={80} />
   <div className="go-profile-summary"><h3>{profile?.display_name || user.username}</h3><p>@{user.username}<span className="go-profile-status">{user.is_active === false ? "Inactive" : "Active"}</span></p>
    <div className="go-profile-photo-actions"><button type="button" disabled={busy} onClick={() => upload.current?.click()}>{photoStage || (profile?.has_photo ? "Change photo" : "Upload photo")}</button>{profile?.has_photo && <button type="button" className="go-profile-remove" disabled={busy} onClick={() => void photo()}>Remove</button>}</div>
    <p className="go-profile-photo-help" id="photo-help">Choose a photo. We’ll resize it automatically.</p>
    <ValidatedInput ref={upload} className="sr-only" type="file" aria-label="Profile photo" aria-describedby="photo-help" accept="image/*" disabled={busy} onChange={e => { const file=e.target.files?.[0]; if(file) void photo(file); e.target.value=""; }} />
   </div>
  </div>
  <nav className="go-profile-tabs" aria-label="Account settings sections">{(["profile", "security"] as const).map(value => <button type="button" key={value} aria-current={section === value ? "page" : undefined} onClick={() => { setSection(value); setError(""); setNotice(""); }}>{value === "profile" ? "Personal details" : "Security"}</button>)}</nav>
  <div className="go-profile-body">
   {error && <p role="alert" className="go-form-error">{error}</p>}{notice && <p role="status" className="go-form-success">{notice}</p>}
   <section hidden={section !== "profile"}>
    <UsernameEditor key={user.username} disabled={busy} />
    <div className="go-profile-section-title"><h3>Personal information</h3><p>Keep your name and contact details up to date.</p></div>
    {!ready ? <p role="status">Loading personal details…</p> : <ValidatedForm onSubmit={details} className="go-personal-form go-profile-details">
     <fieldset disabled={busy}><div className="go-profile-fields">
      <ValidatedLabel>First name<ValidatedInput name="first_name" defaultValue={profile?.contact?.first_name || ""} autoComplete="given-name" maxLength={100} placeholder="Enter first name" required /></ValidatedLabel>
      <ValidatedLabel>Last name<ValidatedInput name="last_name" defaultValue={profile?.contact?.last_name || ""} autoComplete="family-name" maxLength={100} placeholder="Enter last name" required /></ValidatedLabel>
      <ValidatedLabel>Display name<ValidatedInput name="display_name" defaultValue={profile?.display_name || user.username} autoComplete="nickname" maxLength={100} required /></ValidatedLabel>
      <ValidatedLabel>Mobile number<ValidatedInput name="mobile" type="tel" defaultValue={profile?.contact?.mobile || ""} autoComplete="tel" maxLength={24} pattern={"(?=(?:[^0-9]*[0-9]){8,15}[^0-9]*$)\\+?[0-9\\(\\) \\-]+"} title="Use 8 to 15 digits, optionally with +, spaces, hyphens or parentheses." placeholder="e.g. +91 98765 43210" required /></ValidatedLabel>
     </div></fieldset>
     <div className="go-profile-contact-note"><span>Email address</span><strong>{user.email}</strong><button type="button" onClick={() => { setSection("security"); setSecurityMode("email"); }}>Change email →</button></div>
     <div className="go-profile-footer"><p>Changes apply to your employee record.</p><button type="submit" disabled={busy}>{busy ? "Saving…" : "Save changes"}</button></div>
    </ValidatedForm>}
   </section><section hidden={section !== "security"}>
    <div className="go-profile-section-title"><h3>Account security</h3><p>Use your current password to confirm security changes.</p></div>
    <div className="go-security-options" aria-label="Security options"><button type="button" aria-pressed={securityMode === "email"} onClick={() => { setSecurityMode("email"); setError(""); }}>Email address</button><button type="button" aria-pressed={securityMode === "password"} onClick={() => { setSecurityMode("password"); setError(""); }}>Password</button></div>
    {securityMode === "email" ? <ValidatedForm onSubmit={e => void security(e, false)} className="go-personal-form"><ValidatedLabel>Email address<ValidatedInput type="email" name="email" defaultValue={user.email} autoComplete="email" maxLength={255} required disabled={busy} /></ValidatedLabel><ValidatedLabel>Current password<ValidatedInput type="password" name="current_password" autoComplete="current-password" maxLength={128} required disabled={busy} /></ValidatedLabel><div className="go-profile-footer"><p>Your sign-in email will be updated.</p><button type="submit" disabled={busy}>{busy ? "Updating…" : "Update email"}</button></div></ValidatedForm> : <ValidatedForm onSubmit={e => void security(e, true)} className="go-personal-form"><ValidatedLabel>Current password<ValidatedInput type="password" name="current_password" autoComplete="current-password" maxLength={128} required disabled={busy} /></ValidatedLabel><div className="go-profile-fields"><ValidatedLabel>New password<ValidatedInput type="password" name="password" autoComplete="new-password" minLength={8} maxLength={128} pattern="(?=.*[a-z])(?=.*[A-Z])(?=.*[0-9])(?=.*[^a-zA-Z0-9]).{8,}" title="Use uppercase, lowercase, a number and a symbol." required disabled={busy} /></ValidatedLabel><ValidatedLabel>Confirm new password<ValidatedInput type="password" name="confirmation" autoComplete="new-password" maxLength={128} error={confirmationError} onChange={() => setConfirmationError("")} required disabled={busy} /></ValidatedLabel></div><p>Use at least 8 characters, with uppercase, lowercase, a number and a symbol. You’ll sign in again after changing your password.</p><div className="go-profile-footer"><button type="submit" disabled={busy}>{busy ? "Updating…" : "Change password"}</button></div></ValidatedForm>}
   </section>
  </div>
 </dialog>;
}
