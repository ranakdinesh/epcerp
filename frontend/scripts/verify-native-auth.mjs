import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const base = process.env.EPCERP_FRONTEND_URL ?? "http://localhost:9092";
assert.ok(["localhost","127.0.0.1"].includes(new URL(base).hostname),"This verifier is restricted to local preview");
const root = fileURLToPath(new URL("../..",import.meta.url));
const testFile = path.join(root,"deployment/epcerp/.env.test-users");
process.loadEnvFile(process.env.EPCERP_TEST_CREDENTIALS_FILE ?? (fs.existsSync(testFile) ? testFile : path.join(root,"deployment/epcerp/.env")));
const password = process.env.EPCERP_SUPERADMIN_PASSWORD ?? process.env.EPCERP_ADMIN_PASSWORD;
assert.ok(password,"Provide the existing local superadmin test credential; never reset it for this check");
const runtime = process.env.EPCERP_PLAYWRIGHT_MODULE;
const { chromium } = runtime ? await import(pathToFileURL(runtime).href) : await import("playwright");
const output = process.env.EPCERP_AUTH_QA_OUTPUT ?? path.join(os.tmpdir(),"epcerp-native-auth-browser");
fs.mkdirSync(output,{recursive:true,mode:0o700});
const stateFile = path.join(output,"precutover-state.json");
const browser = await chromium.launch({headless:true,...(process.env.EPCERP_CHROME_EXECUTABLE ? {executablePath:process.env.EPCERP_CHROME_EXECUTABLE} : {})});
const errors = [];
let stage = "browser startup";
let lastPage;
async function context(state) {
  const c = await browser.newContext({viewport:{width:1440,height:1000},colorScheme:"light",...(state ? {storageState:state} : {})});
  c.setDefaultTimeout(15000);c.setDefaultNavigationTimeout(15000);
  await c.route("**/*",r=>["localhost","127.0.0.1"].includes(new URL(r.request().url()).hostname) ? r.continue() : r.abort());
  const p = await c.newPage();lastPage=p;p.on("pageerror",e=>errors.push(e.message));
  p.on("console",m=>{if(m.type()==="error")errors.push(m.text());});
  return {c,p};
}
async function signIn(p) {
  stage = "existing-password sign-in";
  await p.goto(`${base}/login?session=expired`);
  await p.locator("#login-identifier").fill("superadmin");
  await p.locator("#login-password").fill(password);
  await p.locator('button[type="submit"]').click();
  stage="dashboard navigation";
  await p.waitForURL(url=>url.pathname==="/dashboard");
  stage="page identity";
  await p.locator("#desktop-navigation").waitFor();
  // The existing dashboard sometimes clears title during client navigation.
  // Verify its visible identity as well, without changing UI in an auth migration.
  const title=await p.title();if(title)assert.match(title,/epcERP/);
  assert.match(await p.locator(".go-appbar").innerText(),/epcERP/);
}
async function screen(p,name,heading,endpoint) {
  stage = `screen ${name}`;
  const loaded=p.waitForResponse(r=>r.url().includes(endpoint)&&r.request().method()==="GET"&&r.status()===200);
  await p.goto(`${base}/dashboard?screen=${name}`);await loaded;
  await p.getByRole("heading",{name:heading,exact:true}).waitFor();
  assert.doesNotMatch(await p.locator("body").innerText(),/Unable to load|Internal Server Error|Application error/);
  await p.screenshot({path:path.join(output,`${name}.png`),fullPage:false});
}
try {
  if(process.argv.includes("--capture-before")) {
    const {c,p}=await context();await signIn(p);
    await c.storageState({path:stateFile});fs.chmodSync(stateFile,0o600);
    console.log("PASS: existing password signs in before ownership switch; private session captured.");
  } else {
    assert.ok(fs.existsSync(stateFile),"Capture the existing Go session before switching ownership");
    const {c,p}=await context(stateFile);
    await p.goto(`${base}/dashboard`);await p.waitForURL(url=>url.pathname==="/dashboard");
    await p.locator("#desktop-navigation").waitFor();
    const title=await p.title();if(title)assert.match(title,/epcERP/);
    assert.match(await p.locator(".go-appbar").innerText(),/epcERP/);
    assert.ok((await p.locator("body").innerText()).length>100);
    await screen(p,"users","Users","/api/administration/users?");
    await screen(p,"roles","Roles & permissions","/api/administration/roles/permissions");
    for(const endpoint of ["/api/auth/me/sessions","/api/projects","/api/auth/user_profile/me","/api/administration/organisation/departments"]){
      const r=await c.request.get(`${base}${endpoint}`);assert.equal(r.status(),200,endpoint);
    }
    await p.setViewportSize({width:390,height:844});
    await p.screenshot({path:path.join(output,"roles-mobile.png"),fullPage:false});
    assert.ok(await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
    const out=await p.evaluate(async()=>{const r=await fetch("/api/auth/logout",{method:"POST"});return r.status;});assert.equal(out,200);
    assert.equal((await c.cookies()).filter(v=>v.name.startsWith("epcerp_")).length,0);
    const fresh=await context();await signIn(fresh.p);
    await fresh.p.screenshot({path:path.join(output,"fresh-password-login.png"),fullPage:false});
    assert.deepEqual(errors,[]);
    console.log("PASS: pre-switch session, unchanged-password login, users, roles, sessions, Go projects/profile/organisation, logout, mobile overflow and browser console.");
  }
} catch(error) {
  console.error(`Local auth browser verification failed at ${stage}; no credentials logged.`);
  if(error instanceof assert.AssertionError) console.error(error.message);
  if(lastPage){await lastPage.screenshot({path:path.join(output,"failure.png"),fullPage:false});console.error("Page:",new URL(lastPage.url()).pathname);const alert=await lastPage.locator('[role="alert"]').allTextContents();if(alert.length)console.error("Visible alert:",alert.join(" "));}
  process.exitCode=1;
} finally {await browser.close();}
