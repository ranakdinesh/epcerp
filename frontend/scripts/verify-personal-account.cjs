/* eslint-disable @typescript-eslint/no-require-imports -- opt-in Node browser regression runner */
const path=require("node:path");
const {chromium}=require(process.env.EPCERP_PLAYWRIGHT_MODULE || 'playwright');const fs=require('node:fs');const assert=require('node:assert/strict');const crypto=require('node:crypto');
const env=Object.fromEntries(fs.readFileSync(process.env.EPCERP_TEST_CREDENTIALS_FILE || path.resolve(__dirname,'../../deployment/epcerp/.env.test-users'),'utf8').trim().split('\n').map(s=>{const i=s.indexOf('=');return[s.slice(0,i),s.slice(i+1)]}));
let stage='login';
(async()=>{const browser=await chromium.launch({headless:true});const context=await browser.newContext({viewport:{width:1280,height:900}});const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));let id,email,restored=false,changed=false,emailDirty=false;
const original=env.EPCERP_DIRECTOR_TEST_PASSWORD;const temporary='Qa-A1!'+crypto.randomUUID();
async function login(password){await page.goto('http://localhost:9092/login');await page.locator('#login-identifier').fill(env.EPCERP_DIRECTOR_TEST_USERNAME);await page.locator('#login-password').fill(password);await page.locator('button[type=submit]').click();await page.waitForURL('**/dashboard')}
async function password(current,next){await page.locator('.go-account-menu summary').click();await page.getByRole('button',{name:'Change password',exact:true}).click();const d=page.getByRole('dialog',{name:'Change password'});await d.locator('[name=current_password]').fill(current);await d.locator('[name=password]').fill(next);await d.locator('[name=confirmation]').fill(next);const response = page.waitForResponse(r=>new URL(r.url()).pathname==='/api/auth/me/password'&&r.request().method()==='POST');await d.getByRole('button',{name:'Change password',exact:true}).click();assert.equal((await response).status(),204);await page.waitForURL('**/login?session=password-changed');await page.getByRole('status').filter({hasText:'Sign in with your updated password.'}).waitFor();assert.equal((await context.cookies()).filter(c=>c.name.startsWith('epcerp_')).length,0)}
try{
 await login(original);id=(await (await page.request.get('http://localhost:9092/api/auth/user_profile/me')).json()).user_id;
 stage='email';await page.locator('.go-account-menu summary').click();await page.getByRole('button',{name:'Me · Edit profile'}).click();let d=page.getByRole('dialog',{name:'Me · Personal profile'});email=await d.locator('[name=email]').inputValue();
 async function updateEmail(next){
  emailDirty=true;await d.locator('[name=email]').fill(next);await d.locator('[name=current_password]').fill(original);
  const response=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/auth/me/email'&&r.request().method()==='PATCH');
  await d.getByRole('button',{name:'Update email'}).click();assert.equal((await response).status(),204);await d.getByRole('status').filter({hasText:'Email updated.'}).waitFor();
  await d.getByRole('button',{name:'Close personal account'}).click();await page.reload();
  await page.locator('.go-account-menu summary').click();await page.getByRole('button',{name:'Me · Edit profile'}).click();d=page.getByRole('dialog',{name:'Me · Personal profile'});
  assert.equal(await d.locator('[name=email]').inputValue(),next);
 }
 await updateEmail('director.qa@example.invalid');await updateEmail(email);emailDirty=false;await d.getByRole('button',{name:'Close personal account'}).click();
 stage='password change';changed=true;await password(original,temporary);
 stage='new password login';await login(temporary);
 stage='restore password';await password(temporary,original);restored=true;
 stage='original login';await login(original);stage='restored email verification';await page.locator('.go-account-menu summary').click();await page.getByRole('button',{name:'Me · Edit profile'}).click();assert.equal(await page.getByRole('dialog').locator('[name=email]').inputValue(),email);await page.keyboard.press('Escape');await page.locator('.go-account-menu summary').click();await page.getByRole('button',{name:'Sign out',exact:true}).click();await page.waitForURL(/\/login/);assert.deepEqual(errors,[]);
 console.log('PASS: Director self-service email update/restoration, password change, cleared private cookies, new-password login and original credential restoration; no runtime errors.');
}finally{
 if(id&&((!restored&&changed)||emailDirty)){const admin=await browser.newContext();const request=admin.request;const auth=await request.post('http://localhost:9092/api/auth/login',{data:{identifier:env.EPCERP_SUPERADMIN_USERNAME,password:env.EPCERP_SUPERADMIN_PASSWORD}});assert.equal(auth.status(),200);const reset=await request.post(`http://localhost:9092/api/administration/users/${id}/password`,{data:{password:original}});assert.equal(reset.status(),204);if(email){const restoreEmail=await request.patch(`http://localhost:9092/api/administration/users/${id}`,{data:{email}});assert.equal(restoreEmail.status(),200)}await admin.close()}
 await browser.close();
}
})().catch(()=>{console.error('Account QA failed at '+stage);process.exitCode=1});
