import assert from "node:assert/strict";
import test from "node:test";
import {readFileSync} from "node:fs";
import ts from "typescript";
import {NextRequest,NextResponse} from "next/server.js";
function load(path,deps){const source=readFileSync(new URL(path,import.meta.url),"utf8");const {outputText}=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}});const exports={};new Function("require","exports",outputText)(n=>deps[n],exports);return exports;}
const origin=load("../src/lib/auth/request-origin.ts",{});
const {proxy}=load("../src/proxy.ts",{"next/server":{NextResponse},"@/lib/auth/request-origin":origin,"@/lib/auth/session":{AUTH_COOKIE_NAMES:{accessToken:"access"}}});
test("all browser API mutations require the real same origin, including login and logout",()=>{
 for(const path of ["/api/auth/login","/api/auth/logout","/api/auth/me/email","/api/administration/users/a/status","/api/administration/roles/a"]){for(const supplied of [undefined,"https://attacker.invalid","https://sibling.example.invalid"]){const headers={host:"erp.example.invalid"};if(supplied)headers.origin=supplied;const r=proxy(new NextRequest(`https://erp.example.invalid${path}`,{method:"POST",headers}));assert.equal(r.status,403,path);}
 const r=proxy(new NextRequest(`https://erp.example.invalid${path}`,{method:"PATCH",headers:{host:"erp.example.invalid",origin:"https://erp.example.invalid"}}));assert.equal(r.status,200);}
});
test("read-only API requests do not require an origin header",()=>{assert.equal(proxy(new NextRequest("https://erp.example.invalid/api/auth/me/sessions")).status,200);});

test("password recovery accepts the browser-facing origin when Next uses its bind address", async () => {
  const { recoveryRequest } = load("../src/lib/auth/recovery-proxy.ts", {
    "next/server": { NextResponse },
    "./request-origin": origin,
    "./backend-config": { backendApiUrl: "http://backend.invalid" },
    "@/lib/auth/backend-request": { backendRequest: async () => Response.json({ message: "Accepted" }, { status: 202 }) },
  });
  for (const [action, body, status] of [
    ["forgot-password", { email: "user@example.invalid" }, 202],
    ["reset-password", { token: "fixture", password: "Fixture-Password1!" }, 202],
  ]) {
    for (const [suppliedOrigin, expected] of [["http://127.0.0.1:9092", status], ["http://other.invalid", 403], [null, 403]]) {
      const headers = { host: "127.0.0.1:9092" };
      if (suppliedOrigin) headers.origin = suppliedOrigin;
      const response = await recoveryRequest(new Request(`http://0.0.0.0:3000/api/auth/${action}`, {
        method: "POST", headers, body: JSON.stringify(body),
      }), action);
      assert.equal(response.status, expected);
    }
  }
});
