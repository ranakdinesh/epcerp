import "server-only";
import { AuthError, denied, invalid, reason } from "./domain";
import * as session from "./sessions";
import * as admin from "./administration";
import * as account from "./account";
import { address, body, boolean, contact, list, names, text, userName, version } from "./validation";
import { requireMail } from "./mail";
import { limitPublic } from "./rate-limit";

// HTTP is only an adapter. Services can also be called by server actions and,
// later, versioned mobile endpoints without duplicating authorization policies.
export function ownsAuthPath(path: string) {
  return /^\/(users|roles)(\/|$)/.test(path) || /^\/auth\/(login|refresh|logout|me|activity|forgot-password|reset-password|verify-email|administration-history)(\/|$)/.test(path);
}
const response = (value?: unknown, status = value === undefined ? 204 : 200) => new Response(value === undefined ? null : JSON.stringify(value), {
  status, headers: { "Content-Type": "application/json", "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer" },
});
export async function handleAuth(request: Request): Promise<Response> {
  try {
    const url = new URL(request.url), path = url.pathname.replace(/^\/api\/v1/, ""), method = request.method;
    const auth = /^Bearer ([^\s]+)$/i.exec(request.headers.get("authorization") ?? "");
    const token = auth?.[1] ?? "";
    const route = `${method} ${path}`;
    if (method === "POST" && path === "/auth/login") await limitPublic("login",100,60);
    if (method === "POST" && ["/auth/forgot-password","/auth/reset-password","/auth/verify-email"].includes(path)) await limitPublic(path,5,900);
    if (route === "POST /auth/login") {
      const p = await body(request, ["identifier", "password"], 4096);
      return response(await session.login(text(p,"identifier",3,255), text(p,"password",8,128)));
    }
    if (route === "POST /auth/refresh" || route === "POST /auth/logout") {
      const p = await body(request,["refresh_token"],4096), raw = text(p,"refresh_token",1,512,false);
      if (path.endsWith("refresh")) return response(await session.refresh(raw));
      await session.logout(raw); return response();
    }
    if (route === "POST /auth/forgot-password") {
      const p = await body(request,["email"],4096), email = address(p);
      requireMail();
      // Delivery failures must not disclose whether an email belongs to an account.
      try { await account.requestRecovery(email); } catch { console.error("Password recovery delivery unavailable"); }
      return response({ message: "If an active account uses this email, password reset instructions will be sent. Check your inbox and spam folder." },202);
    }
    if (route === "POST /auth/reset-password") {
      const p = await body(request,["token","password"],4096);
      await account.completeRecovery(text(p,"token",64,64,false),text(p,"password",8,128,false)); return response();
    }
    if (route === "POST /auth/verify-email") {
      const p = await body(request,["token"],4096); await account.verifyEmail(text(p,"token",64,64,false)); return response();
    }
    if (!token) throw denied();
    if (route === "GET /auth/me") return response(await session.me(token));
    if (route === "POST /auth/activity") {
      const p = await body(request,["current_page"]), page = text(p,"current_page",1,255);
      if (!/^\/[a-zA-Z0-9_./?=&%\-]*$/.test(page)) invalid("Invalid application path");
      await session.activity(token,page); return response();
    }
    if (route === "GET /auth/me/sessions") return response(await session.sessions(token));
    if (route === "DELETE /auth/me/sessions") { await session.revokeSessions(token); return response(); }
    const family = /^\/auth\/me\/sessions\/([^/]+)$/.exec(path);
    if (family && method === "DELETE") { await session.revokeSessions(token,family[1]); return response(); }
    if (route === "POST /auth/me/password" || route === "PATCH /auth/me/email") {
      const password = path.endsWith("password"), p = await body(request,["current_password",password ? "password" : "email"],8192);
      await account.changeAccount(token,text(p,"current_password",1,128,false),password ? { password: text(p,"password",8,128,false) } : { email: address(p) }); return response();
    }
    if (route === "GET /auth/me/username") return response(await account.usernameAvailable(token,url.searchParams.get("username") ?? ""));
    if (route === "PATCH /auth/me/username") {
      const p = await body(request,["username","expected_username"],8192);
      await account.changeUsername(token,userName(p),text(p,"expected_username",3,64,false)); return response();
    }
    if (route === "GET /users") return response(await admin.users(token,url.searchParams));
    if (route === "GET /users/online/activity") return response(await admin.online(token));
    if (route === "POST /users") {
      const p = await body(request,["username","email","first_name","last_name","mobile","role_names","is_superuser"]);
      const superuser = boolean(p,"is_superuser",false), details = contact(p);
      return response(await admin.createUser(token,{ username:userName(p),email:address(p),first_name:details.first_name!,last_name:details.last_name!,mobile:details.mobile!,role_names:names(p,superuser ? 0 : 1),is_superuser:superuser }),201);
    }
    const target = /^\/users\/([^/]+)(?:\/(password|deactivate|status))?$/.exec(path);
    if (target) {
      const [,id,action] = target;
      if (!action && method === "GET") return response(await admin.user(token,id));
      if (!action && method === "PATCH") {
        const p = await body(request,["username","email","first_name","last_name","mobile","role_names","expected_version"]);
        return response(await admin.updateUser(token,id,{...contact(p,true), expected_version:version(p),
          ...(typeof p.username === "string" && p.username.trim() !== "" ? { username: text(p,"username",3,64) } : {}),
          ...(p.email !== undefined ? { email:address(p) } : {}), ...(p.role_names !== undefined ? { role_names:names(p,1) } : {})}));
      }
      if (action === "password" && method === "POST") {
        const p = await body(request,["reason","password","username","confirmation"]);
        const pwd = text(p,"password",8,128);
        if (pwd !== text(p,"confirmation",8,128)) invalid("New password and confirmation do not match.");
        const uname = typeof p.username === "string" && p.username.trim() !== "" ? text(p,"username",3,64) : undefined;
        const changeReason = p.reason === undefined ? "Password set by Superadmin" : reason(p.reason);
        await admin.resetCredential(token,id,changeReason,pwd,uname);
        return response();
      }
      if (action === "deactivate" && method === "POST") return response(await admin.setActive(token,id,false,"Administrator revoked account access."));
      if (action === "status" && method === "POST") {
        const p = await body(request,["is_active","reason"]); return response(await admin.setActive(token,id,boolean(p,"is_active"),reason(p.reason)));
      }
    }
    if (route === "GET /roles") return response(await admin.roles(token));
    if (route === "GET /roles/permissions") return response(await admin.permissions(token));
    if (route === "POST /roles/finance-presets") return response(await admin.financePresets(token));
    if (route === "POST /roles") {
      const p = await body(request,["name","description","permission_codes"]), name = text(p,"name",3,64);
      if (!/^[a-z0-9._-]+$/.test(name)) invalid("Invalid role name");
      return response(await admin.createRole(token,{name,description:p.description === undefined ? "" : text(p,"description",0,255),permission_codes:list(p,"permission_codes",1,100)}),201);
    }
    const role = /^\/roles\/([^/]+)$/.exec(path);
    if (role && method === "PATCH") {
      const p = await body(request,["description","permission_codes","is_active","expected_version"]);
      return response(await admin.updateRole(token,role[1],{expected_version:version(p),
        ...(p.description !== undefined ? {description:text(p,"description",0,255)} : {}),
        ...(p.permission_codes !== undefined ? {permission_codes:list(p,"permission_codes",0,100)} : {}),
        ...(p.is_active !== undefined ? {is_active:boolean(p,"is_active")} : {})}));
    }
    if (route === "GET /auth/administration-history") return response(await admin.history(token,url.searchParams.get("entity_id")));
    return response({detail:"Not found"},404);
  } catch (error) {
    if (error instanceof AuthError) {
      const result = response({detail:error.message},error.status);
      if (error.status === 401) result.headers.set("WWW-Authenticate","Bearer");
      if (error.status === 429) result.headers.set("Retry-After","900");
      return result;
    }
    // Never log database errors, token values, hashes, request bodies or secrets.
    console.error("Authentication operation unavailable");
    return response({detail:"Service temporarily unavailable"},503);
  }
}
