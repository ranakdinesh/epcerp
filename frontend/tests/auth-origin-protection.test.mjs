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
