#!/usr/bin/env node
import {access,readFile,mkdir,cp,rm,symlink,realpath,lstat,readdir} from 'node:fs/promises';
import {parseArgs} from 'node:util';import {resolve,join,dirname,relative} from 'node:path';import {fileURLToPath,pathToFileURL} from 'node:url';import {realpathSync} from 'node:fs';
export const skillSource=fileURLToPath(new URL('../',import.meta.url));
async function exists(path){try{await lstat(path);return true;}catch(error){if(error.code==='ENOENT'||error.code==='ENOTDIR')return false;throw error;}}
/** One canonical skill; Claude points to the same files. Gemini supports the .agents alias. */
export async function installSkill({cwd=process.cwd(),agent='all',dryRun=false,source=skillSource}={}){
 if(!['all','codex','claude','gemini'].includes(agent))throw new Error('agent must be all, codex, claude or gemini');
 const root=await realpath(cwd), canonical=join(root,'.agents/skills/citual-ui');
 const aliases=['all','claude'].includes(agent)?[join(root,'.claude/skills/citual-ui')]:[];
 await access(join(source,'SKILL.md'));
 const version=JSON.parse(await readFile(join(source,'version.json'),'utf8')).version;
 const created=[];let adopted;
 if(!(await exists(canonical)))for(const native of [join(root,'.claude/skills/citual-ui'),join(root,'.gemini/skills/citual-ui')])if(await exists(native)){
  let nativeVersion;try{nativeVersion=JSON.parse(await readFile(join(native,'version.json'),'utf8')).version;}catch{}
  if(nativeVersion===version){adopted=await realpath(native);break;}
 }

 if(await exists(canonical)){
  let installed;try{installed=JSON.parse(await readFile(join(canonical,'version.json'),'utf8')).version;}catch{throw new Error('A Citual UI skill already exists without version metadata. Preserve or remove it explicitly before installation.');}
  if(installed!==version)throw new Error(`Existing Citual UI skill ${installed} differs from ${version}. Update the skill explicitly; it was not overwritten.`);
 }else created.push(canonical);
 for(const alias of aliases)if(await exists(alias)){
  if(await realpath(alias).catch(()=>undefined)!==(adopted??await realpath(canonical).catch(()=>undefined)))throw new Error(`Existing skill destination ${relative(root,alias)} is not the shared skill. It was not overwritten.`);
 }else created.push(alias);
 if(!dryRun){const completed=[];try{
  if(created.includes(canonical)){await mkdir(dirname(canonical),{recursive:true});if(adopted){await symlink(process.platform==='win32'?adopted:relative(dirname(canonical),adopted),canonical,process.platform==='win32'?'junction':'dir');completed.push(canonical);}else{await mkdir(canonical);completed.push(canonical);for(const entry of await readdir(source))await cp(join(source,entry),join(canonical,entry),{recursive:true,errorOnExist:true,force:false});}}
  for(const alias of aliases)if(created.includes(alias)){await mkdir(dirname(alias),{recursive:true});await symlink(process.platform==='win32'?canonical:relative(dirname(alias),canonical),alias,process.platform==='win32'?'junction':'dir');completed.push(alias);}
 }catch(error){for(const file of completed.reverse())await rm(file,{recursive:true,force:true});throw error;}}
 return {status:dryRun?'planned':created.length?'installed':'already-installed',version,canonical:relative(root,canonical),agents:agent==='all'?['codex','claude','gemini']:[agent],aliases:aliases.map(path=>relative(root,path)),created};
}
export async function main(args=process.argv.slice(2)){const {values}=parseArgs({args,options:{cwd:{type:'string'},agent:{type:'string',default:'all'},'dry-run':{type:'boolean',default:false}}});console.log(JSON.stringify(await installSkill({cwd:values.cwd,agent:values.agent,dryRun:values['dry-run']}),null,2));}
if(process.argv[1]&&import.meta.url===pathToFileURL(realpathSync(process.argv[1])).href)main().catch(error=>{console.error(error.message);process.exitCode=1;});
