import "server-only";
import { randomUUID } from "node:crypto";
import { lock, rows, transaction } from "../db";
import { hashPassword, config } from "./security";
import { email, username } from "./domain";
import registry from "./permissions.json";

export async function initializeAuth() {
  config();
  await transaction(async db => {
    const [migration] = await rows<{ version: string; dirty: boolean }>(db, "SELECT version,dirty FROM public.schema_migrations");
    if (!migration || migration.dirty || Number(migration.version) < 96) throw new Error("Apply the owned Go migrations before starting Next.js Auth");
    await lock(db,"auth.administration");
    for (const p of registry) await db.query("INSERT INTO auth.permissions(id,code,description) VALUES($1,$2,$3) ON CONFLICT(code) DO UPDATE SET description=EXCLUDED.description", [randomUUID(),p.code,p.description]);
    const [role] = await rows<{ id: string }>(db, "INSERT INTO auth.roles(id,name,description,is_active) VALUES($1,'superadmin','Platform super administrator',true) ON CONFLICT(name) DO UPDATE SET name=roles.name RETURNING id", [randomUUID()]);
    for (const p of registry) await db.query("INSERT INTO auth.role_permissions(role_id,permission_id) SELECT $1,id FROM auth.permissions WHERE code=$2 ON CONFLICT DO NOTHING",[role.id,p.code]);
    const [bound] = await rows<{ id: string; is_superuser: boolean }>(db, "SELECT u.id,u.is_superuser FROM auth.users u JOIN auth.auth_bootstrap_identity b ON b.user_id=u.id WHERE b.singleton");
    let u = bound;
    if (!u) {
      const name = username(process.env.VTPL_BOOTSTRAP_SUPERADMIN_USERNAME ?? "superadmin");
      [u] = await rows(db,"SELECT id,is_superuser FROM auth.users WHERE username=$1",[name]);
      if (!u) {
        const password = process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD ?? "";
        if ([...password].length < 10) throw new Error("Bootstrap credentials required for a new database");
        const [count] = await rows<{ n: string }>(db,"SELECT count(*) AS n FROM auth.users WHERE is_superuser");
        if (Number(count.n) >= 2) throw new Error("Superadmin account limit reached");
        [u] = await rows(db,"INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,$4,true,true) RETURNING id,is_superuser",
          [randomUUID(),name,email(process.env.VTPL_BOOTSTRAP_SUPERADMIN_EMAIL ?? ""),await hashPassword(password)]);
      }
      await db.query("INSERT INTO auth.auth_bootstrap_identity(singleton,user_id) VALUES(true,$1)",[u.id]);
    }
    if (!u.is_superuser) {
      const [count] = await rows<{ n: string }>(db,"SELECT count(*) AS n FROM auth.users WHERE is_superuser");
      if (Number(count.n) >= 2) throw new Error("Superadmin account limit reached");
      await db.query("UPDATE auth.users SET is_superuser=true,updated_at=now() WHERE id=$1",[u.id]);
    }
    // Never overwrite existing password hashes, email addresses, username,
    // activation state, mandatory-change flags or credential versions.
    await db.query("INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2) ON CONFLICT DO NOTHING",[u.id,role.id]);
  });
}
