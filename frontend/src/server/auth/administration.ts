import { randomBytes, randomUUID } from "node:crypto";
import { rows, transaction, type DB } from "../db";
import { AuthError, forbidden, invalid, publicUser, reason, strongPassword, username, uuid, type User } from "./domain";
import { accountEvent, assignRoles, authorize, loadRoles, loadUser, revokeUser, setPassword } from "./repository";
import { config, hashPassword } from "./security";
import { publicURL, queueCredentialMail, requireMail } from "./mail";
import { stageEmail, sendVerification } from "./account";
import registry from "./permissions.json";
import presets from "./finance-presets.json";

export function users(token: string, query: URLSearchParams) {
  return transaction(async db => {
    await authorize(db, token, "admin.users.read");
    const page = integerQuery(query, "page", 1, 1, 2147483647), size = integerQuery(query, "page_size", 10, 1, 100);
    const status = query.get("status"), search = query.get("search") ?? "", role = query.get("role") ?? "";
    if (status !== null && !["active", "inactive"].includes(status)) invalid("Invalid active filter");
    if (query.has("sort") && !["username_asc", "username_desc"].includes(query.get("sort")!)) invalid("Invalid sort");
    if (search.length > 128 || role.length > 64 || query.has("search") && !search || query.has("role") && !role) invalid("Invalid filter length");
    const filter = `($1::boolean IS NULL OR u.is_active=$1) AND ($2::text='' OR lower(u.username) LIKE '%'||lower($2)||'%' OR lower(u.email) LIKE '%'||lower($2)||'%') AND
      ($3::text='' OR EXISTS(SELECT 1 FROM auth.user_roles ur JOIN auth.roles r ON r.id=ur.role_id WHERE ur.user_id=u.id AND r.name=$3))`;
    const args = [status === null ? null : status === "active", search, role];
    const [count] = await rows<{ total: string }>(db, `SELECT count(*) AS total FROM auth.users u WHERE ${filter}`, args);
    const ids = await rows<{ id: string }>(db, `SELECT u.id FROM auth.users u WHERE ${filter} ORDER BY u.username ${query.get("sort") === "username_desc" ? "DESC" : "ASC"} LIMIT $4 OFFSET $5`, [...args, size, (page-1)*size]);
    const items = [];
    for (const { id } of ids) items.push(publicUser((await loadUser(db, id))!));
    return { items, page, page_size: size, total: Number(count.total), total_pages: Math.ceil(Number(count.total)/size) };
  });
}
function integerQuery(query: URLSearchParams, name: string, fallback: number, min: number, max: number) {
  const raw = query.get(name), n = raw === null ? fallback : /^\d+$/.test(raw) ? Number(raw) : NaN;
  if (!Number.isSafeInteger(n) || n < min || n > max) invalid(`Invalid ${name}`);
  return n;
}
export function user(token: string, id: string) {
  uuid(id);
  return transaction(async db => {
    await authorize(db, token, "admin.users.read");
    const u = await loadUser(db, id);
    if (!u) throw new AuthError(404, "User not found");
    return publicUser(u);
  });
}
export type UserInput = { username: string; email: string; first_name: string; last_name: string; mobile: string; role_names: string[]; is_superuser: boolean };
export type UserUpdate = Partial<Pick<UserInput, "username" | "email" | "first_name" | "last_name" | "mobile" | "role_names">> & { expected_version?: number };
function protectTarget(target: User | null, by: User): asserts target is User {
  if (!target) throw new AuthError(404, "User not found");
  if (target.is_superuser && !by.is_superuser) throw forbidden();
}
async function issueCredential(db: DB, u: User, by: string, changeReason: string, reset: boolean) {
  requireMail();
  if (!u.is_active) invalid("Inactive users cannot receive a temporary password.");
  const raw = `Aa1!${randomBytes(24).toString("base64url")}`;
  await setPassword(db, u.id, await hashPassword(raw), by, true);
  const current = (await loadUser(db, u.id))!;
  const subject = reset ? "Your epcERP temporary password" : "Welcome to epcERP — your account is ready";
  const opening = reset ? "An authorised administrator has reset your epcERP password." : "Your Vishal Technopower epcERP account has been created.";
  const message = `Hello ${u.username},\n\n${opening}\n\nUsername: ${u.username}\nTemporary password: ${raw}\n\n${publicURL()}/login\n\nSign in within 24 hours. You must choose a new password before accessing your workspace. This temporary password replaces any earlier password.\n\nFor assistance, contact your administrator.\n\nRegards,\nVishal Technopower`;
  await queueCredentialMail(db, { ID: randomUUID(), UserID: u.id, Recipient: u.email, Subject: subject, Message: message }, current.credential_version, current.temporary_password_expires_at!, by);
  const action = reset ? "temporary_password_reset" : "user_created";
  if (u.is_superuser && !reset) changeReason = "New superadmin account; full platform access";
  await db.query(`INSERT INTO auth.auth_credential_changes(id,user_id,actor_id,action,reason,credential_version) SELECT $1,id,$3,$4,$5,credential_version FROM auth.users WHERE id=$2`, [randomUUID(), u.id, by, action, changeReason]);
  await accountEvent(db, u.id, by, action);
}
export function createUser(token: string, input: UserInput) {
  return transaction(async db => {
    const by = await authorize(db, token, "admin.users.write", true);
    requireMail();
    if (input.is_superuser) {
      if (!by.is_superuser) throw forbidden();
      const [count] = await rows<{ n: string }>(db, "SELECT count(*) AS n FROM auth.users WHERE is_superuser");
      if (Number(count.n) >= 2) throw new AuthError(409, "The system can have at most two superadmin accounts, including inactive accounts");
    }
    const id = randomUUID();
    await db.query(`INSERT INTO auth.users(id,username,email,first_name,last_name,mobile,password_hash,is_active,is_superuser,created_by_id,updated_by_id)
      VALUES($1,$2,$3,$4,$5,$6,'pending-credential',true,$7,$8,$8)`, [id, input.username, input.email, input.first_name, input.last_name, input.mobile, input.is_superuser, by.id]);
    await assignRoles(db, id, input.role_names, by);
    await issueCredential(db, (await loadUser(db, id))!, by.id, "New employee account", false);
    return publicUser((await loadUser(db, id))!);
  });
}
export async function updateUser(token: string, id: string, input: UserUpdate) {
  uuid(id);
  const staged = await transaction(async db => {
    const by = await authorize(db, token, "admin.users.write", true), u = await loadUser(db, id, true);
    protectTarget(u, by);
    if (input.expected_version !== undefined && input.expected_version !== u.edit_version) throw new AuthError(409, "This user changed while you were editing. Reload and review the saved details.");
    let verification: { raw: string; recipient: string } | undefined;
    if (input.email !== undefined && input.email !== u.email) verification = { raw: await stageEmail(db, u, input.email, by.id), recipient: input.email };
    if (input.username !== undefined && input.username.trim() !== "" && input.username.trim() !== u.username) {
      if (!by.is_superuser) throw forbidden();
      const validUname = username(input.username);
      const [existing] = await rows<{ id: string }>(db, "SELECT id FROM auth.users WHERE username=$1 AND id<>$2", [validUname, id]);
      if (existing) invalid("Username is already taken.");
      await db.query("UPDATE auth.users SET username=$2,updated_by_id=$3,updated_at=clock_timestamp() WHERE id=$1", [id, validUname, by.id]);
    }
    await db.query(`UPDATE auth.users SET first_name=COALESCE($2,first_name),last_name=COALESCE($3,last_name),mobile=COALESCE($4,mobile),updated_at=now(),updated_by_id=$5 WHERE id=$1`, [id, input.first_name, input.last_name, input.mobile, by.id]);
    if (input.role_names !== undefined) await assignRoles(db, id, input.role_names, by, u.roles.map(r => r.name));
    return { user: publicUser((await loadUser(db, id))!), verification };
  });
  if (staged.verification) await sendVerification(staged.verification.recipient, staged.verification.raw);
  return staged.user;
}
export function resetCredential(token: string, id: string, changeReason: string, directPassword?: string, newUsername?: string) {
  uuid(id); reason(changeReason);
  if (directPassword !== undefined && !strongPassword(directPassword)) {
    invalid("Use 8 to 128 characters including uppercase, lowercase, number and symbol.");
  }
  return transaction(async db => {
    const by = await authorize(db, token, "admin.users.write", true);
    if (id === by.id) invalid("Use your account security settings to change your own password.");
    const u = await loadUser(db, id, true); protectTarget(u, by);
    if (newUsername !== undefined && newUsername.trim() !== "" && newUsername.trim() !== u.username) {
      if (!by.is_superuser) throw forbidden();
      const validUname = username(newUsername);
      const [existing] = await rows<{ id: string }>(db, "SELECT id FROM auth.users WHERE username=$1 AND id<>$2", [validUname, id]);
      if (existing) invalid("Username is already taken.");
      await db.query("UPDATE auth.users SET username=$2,updated_by_id=$3,updated_at=clock_timestamp() WHERE id=$1", [id, validUname, by.id]);
    }
    if (directPassword !== undefined) {
      await setPassword(db, u.id, await hashPassword(directPassword), by.id, false);
      await db.query(`INSERT INTO auth.auth_credential_changes(id,user_id,actor_id,action,reason,credential_version) SELECT $1,id,$3,$4,$5,credential_version FROM auth.users WHERE id=$2`, [randomUUID(), u.id, by.id, "password_reset", changeReason]);
      await accountEvent(db, u.id, by.id, "password_reset");
    } else {
      await issueCredential(db, u, by.id, changeReason, true);
    }
  });
}
export function setActive(token: string, id: string, active: boolean, changeReason: string) {
  uuid(id); reason(changeReason);
  return transaction(async db => {
    const by = await authorize(db, token, "admin.users.write", true);
    if (!active && id === by.id) invalid("You cannot deactivate yourself");
    const u = await loadUser(db, id, true); protectTarget(u, by);
    if (u.is_active === active) return publicUser(u);
    if (!active && u.is_superuser) {
      const [n] = await rows<{ count: string }>(db, "SELECT count(*) FROM auth.users WHERE is_active AND is_superuser");
      if (Number(n.count) <= 1) invalid("At least one active superadmin must remain");
    }
    await db.query("UPDATE auth.users SET is_active=$2,credential_version=credential_version+1,updated_by_id=$3,updated_at=clock_timestamp() WHERE id=$1", [id, active, by.id]);
    await revokeUser(db, id);
    await db.query("INSERT INTO auth.auth_user_status_changes(id,user_id,actor_id,is_active,reason,credential_version) SELECT $1,id,$3,is_active,$4,credential_version FROM auth.users WHERE id=$2", [randomUUID(), id, by.id, changeReason]);
    if (!active) {
      const caseID = randomUUID();
      const inserted = await rows(db, `INSERT INTO auth.auth_offboarding_cases(id,user_id,manager_id,profile_version,state,reason,items,created_by_id,updated_by_id)
        SELECT $1,u.id,r.manager_id,COALESCE(r.version,0),'pending',$3,'[]'::jsonb,$4,$4 FROM auth.users u LEFT JOIN auth.auth_reporting_lines r ON r.user_id=u.id AND r.is_active WHERE u.id=$2
        ON CONFLICT(user_id) WHERE state IN ('pending','planning','approved') DO NOTHING RETURNING id`, [caseID, id, changeReason, by.id]);
      if (inserted.length) await db.query("INSERT INTO auth.auth_offboarding_changes(id,case_id,version,action,reason,snapshot,created_by_id) SELECT $1,c.id,c.version,'access_revoked',$4,to_jsonb(c),$3 FROM auth.auth_offboarding_cases c WHERE c.id=$2", [randomUUID(), caseID, by.id, changeReason]);
    }
    return publicUser((await loadUser(db, id))!);
  });
}
export function online(token: string) {
  return transaction(async db => {
    await authorize(db, token, "admin.users.read");
    const items = await rows(db, "SELECT a.user_id,u.username,a.current_page,a.last_seen_at FROM auth.user_activity a JOIN auth.users u ON u.id=a.user_id WHERE u.is_active AND a.last_seen_at>=clock_timestamp()-$1*interval '1 second' ORDER BY a.last_seen_at DESC", [config().presenceSeconds]);
    return { items, online_count: items.length };
  });
}
export function roles(token: string) { return transaction(async db => { await authorize(db, token, "admin.roles.read"); return loadRoles(db); }); }
export function permissions(token: string) {
  return transaction(async db => {
    await authorize(db, token, "admin.roles.read");
    const codes = new Set((await rows<{ code: string }>(db, "SELECT code FROM auth.permissions")).map(r => r.code));
    return registry.filter(p => p.code !== "system:admin" && codes.has(p.code));
  });
}
async function validCodes(db: DB, codes: string[]) {
  const registered = new Set((await rows<{ code: string }>(db, "SELECT code FROM auth.permissions")).map(r => r.code));
  for (const code of codes) {
    if (code === "system:admin") invalid("Full platform access is reserved for superadmin accounts; it cannot be granted by a role.");
    if (!registered.has(code) || !registry.some(p => p.code === code)) invalid("Unknown permission");
  }
}
export type RoleInput = { name: string; description: string; permission_codes: string[] };
export type RoleUpdate = Partial<Pick<RoleInput, "description" | "permission_codes">> & { is_active?: boolean; expected_version?: number };
async function saveRole(db: DB, by: User, input: RoleInput | RoleUpdate, id?: string) {
  if (input.permission_codes !== undefined) await validCodes(db, input.permission_codes);
  if (id) {
    const r = (await loadRoles(db)).find(r => r.id === id);
    if (!r) throw new AuthError(404, "Role not found");
    if (r.name === "superadmin" && !by.is_superuser) throw forbidden();
    const change = input as RoleUpdate;
    if (change.expected_version !== undefined && change.expected_version !== r.edit_version) throw new AuthError(409, "This role changed while you were editing. Reload and review its permissions.");
    await db.query("UPDATE auth.roles SET description=COALESCE($2,description),is_active=COALESCE($3,is_active),updated_at=now(),updated_by_id=$4 WHERE id=$1", [id, input.description, change.is_active, by.id]);
  } else {
    id = randomUUID();
    await db.query("INSERT INTO auth.roles(id,name,description,is_active,created_by_id,updated_by_id) VALUES($1,$2,$3,true,$4,$4)", [id, (input as RoleInput).name, input.description, by.id]);
  }
  if (input.permission_codes !== undefined) {
    await db.query("DELETE FROM auth.role_permissions WHERE role_id=$1", [id]);
    for (const code of input.permission_codes) await db.query("INSERT INTO auth.role_permissions(role_id,permission_id) SELECT $1,id FROM auth.permissions WHERE code=$2 ON CONFLICT DO NOTHING", [id, code]);
  }
  return (await loadRoles(db)).find(r => r.id === id)!;
}
export function createRole(token: string, input: RoleInput) { return transaction(async db => saveRole(db, await authorize(db, token, "admin.roles.write", true), input)); }
export function updateRole(token: string, id: string, input: RoleUpdate) { uuid(id); return transaction(async db => saveRole(db, await authorize(db, token, "admin.roles.write", true), input, id)); }
export function financePresets(token: string) {
  return transaction(async db => {
    const by = await authorize(db, token, "admin.roles.write", true), existing = await loadRoles(db), result = [];
    for (const p of presets) result.push(existing.find(r => r.name === p.name) ?? await saveRole(db, by, p));
    return result;
  });
}
export function history(token: string, entity: string | null) {
  if (entity) uuid(entity);
  return transaction(async db => {
    await authorize(db, token, "admin.audit.read");
    const out = await rows(db, `SELECT id,entity_type,entity_id,action,COALESCE(actor_id::text,'') AS actor_id,before_snapshot AS before,after_snapshot AS after,occurred_at
      FROM auth.administration_changes WHERE ($1::uuid IS NULL OR entity_id=$1) ORDER BY id DESC LIMIT 100`, [entity]);
    return out.map(v => ({ ...v, id: Number(v.id) }));
  });
}
