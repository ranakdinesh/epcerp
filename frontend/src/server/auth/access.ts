import "server-only";
import type { DB } from "../db";
import { authorize, resolve, loadUser } from "./repository";
import { allows, forbidden, type User } from "./domain";
import { actor, lock, rows } from "../db";
// Published contract for other server modules. Credential data stays inside Auth.
export async function requirePermission(db: DB, token: string, permission: string, write = false) {
    const user = await authorize(db, token, permission, write);
    return { id: user.id, is_superuser: user.is_superuser };
}
function capabilities(user: User) {
    return { id: user.id, active: user.is_active, superuser: user.is_superuser,
        allows: (permission: string) => allows(user, permission) };
}
export async function requireAnyPermission(db: DB, token: string, permissions: string[], write = false) {
    if (write)
        await lock(db, "auth.administration");
    const user = await resolve(db, token);
    if (!permissions.some(permission => allows(user, permission)))
        throw forbidden();
    if (write)
        await actor(db, user.id);
    return capabilities(user);
}
// Live directory contract: credential fields never leave Auth.
export async function userCapabilities(db: DB, id: string) {
    const user = await loadUser(db, id);
    return user ? capabilities(user) : null;
}
export async function documentReviewCandidates(db: DB) {
    const candidates = [];
    for (const { id } of await rows<{
        id: string;
    }>(db, "SELECT id FROM auth.users WHERE is_active AND NOT is_superuser ORDER BY id")) {
        const user = await userCapabilities(db, id);
        if (user && ["view", "read", "write", "auto_approve"].every(action => user.allows(`accounts.vendor_verification.${action}`)))
            candidates.push(id);
    }
    return candidates;
}
