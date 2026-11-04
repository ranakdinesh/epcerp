import { AppError as AuthError } from "../errors";
export { AppError as AuthError } from "../errors";
export const denied = () => new AuthError(401, "Invalid or expired credentials");
export const forbidden = () => new AuthError(403, "Insufficient permission");
export function invalid(message: string): never { throw new AuthError(422, message); }
export type Role = { id: string; name: string; description: string; is_active: boolean; edit_version: number; permissions: string[] };
export type User = {
  id: string; username: string; email: string; password_hash: string;
  first_name: string; last_name: string; mobile: string;
  is_active: boolean; is_superuser: boolean; must_change_password: boolean;
  temporary_password_expires_at: Date | null; credential_version: number;
  edit_version: number; created_at: Date; updated_at: Date; email_status: string; roles: Role[];
};
export function temporaryValid(user: User, now = Date.now()) {
  return !user.must_change_password || !!user.temporary_password_expires_at && user.temporary_password_expires_at.getTime() > now;
}
export function effectivePermissions(user: User): string[] {
  if (!user.is_active || user.must_change_password) return [];
  const codes = new Set(["account.profile.read", "account.profile.write", "account.security.write", "account.team.read", "account.offboarding.read", "account.offboarding.write"]);
  for (const role of user.roles) if (role.is_active) for (const code of role.permissions) if (code !== "system:admin") codes.add(code);
  if (user.is_superuser) codes.add("system:admin");
  return [...codes].sort();
}
export function allows(user: User, permission: string) {
  return user.is_active && !user.must_change_password && (user.is_superuser || effectivePermissions(user).includes(permission));
}
export function publicUser(user: User) {
  return { id: user.id, username: user.username, email: user.email, first_name: user.first_name,
    last_name: user.last_name, mobile: user.mobile, is_active: user.is_active,
    is_superuser: user.is_superuser, must_change_password: user.must_change_password,
    edit_version: user.edit_version, created_at: user.created_at, email_status: user.email_status,
    roles: user.roles.map(r => r.name).sort() };
}
export function identity(user: User) {
  const { id, username, email, first_name, last_name, mobile, is_active, is_superuser, must_change_password } = user;
  return { id, username, email, first_name, last_name, mobile, is_active, is_superuser, must_change_password, permissions: effectivePermissions(user) };
}
export function strongPassword(value: string) {
  const n = [...value].length;
  return n >= 8 && n <= 128 && /\p{Ll}/u.test(value) && /\p{Lu}/u.test(value) && /\p{Nd}/u.test(value) && /[^\p{L}\p{N}]/u.test(value);
}
export function uuid(value: string) {
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value)) invalid("Invalid UUID");
  return value.toLowerCase();
}
export function username(value: string) {
  value = value.trim();
  if (!/^[a-zA-Z0-9_.-]{3,64}$/.test(value)) invalid("Use 3 to 64 letters, numbers, dots, underscores or hyphens.");
  return value;
}
export function email(value: string) {
  value = value.trim().toLowerCase();
  if (value.length > 255 || !/^[^\s<>@]+@[^\s<>@]+\.[^\s<>@]+$/.test(value)) invalid("A valid email address is required");
  return value;
}
export function reason(value: unknown) {
  if (typeof value !== "string" || Buffer.byteLength(value.trim()) < 10 || Buffer.byteLength(value.trim()) > 1000) invalid("Provide a reason of 10 to 1000 characters.");
  return value.trim();
}
