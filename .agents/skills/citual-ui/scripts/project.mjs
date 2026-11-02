#!/usr/bin/env node
import {readFile,access} from 'node:fs/promises';import {createRequire} from 'node:module';import {resolve,join,dirname} from 'node:path';import {pathToFileURL,fileURLToPath} from 'node:url';import {realpathSync} from 'node:fs';import {spawnSync} from 'node:child_process';import {parseArgs} from 'node:util';
const snapshot=JSON.parse(await readFile(new URL('../references/library.json',import.meta.url),'utf8'));
export async function inspectProject(cwd=process.cwd(),library){
 const root=resolve(cwd);const pkg=JSON.parse(await readFile(join(root,'package.json'),'utf8'));const deps={...pkg.devDependencies,...pkg.dependencies};
 let libraryRoot=library?resolve(library):undefined;
 if(!libraryRoot){try{const require=createRequire(join(root,'package.json'));libraryRoot=dirname(dirname(dirname(require.resolve('citual-ui/workflows'))));}catch{}}
 let version,catalog,setup=false;
 if(libraryRoot){const installed=JSON.parse(await readFile(join(libraryRoot,'package.json'),'utf8'));if(installed.name!=='citual-ui')throw new Error('--library must point to a Citual UI package');version=installed.version;catalog=join(libraryRoot,'registry/agent-catalog.json');await access(catalog);}
 try{await access(join(root,'.citual-ui-setup.json'));setup=true;}catch{}
 return {root,projectName:pkg.name,framework:deps.next?'next':deps.vite?'vite':undefined,react:deps.react,tailwind:deps.tailwindcss,libraryRoot,libraryVersion:version,skillVersion:snapshot.version,versionMatches:version===snapshot.version,catalog,setup,gitDependency:snapshot.gitDependency};
}
export async function connectProject({cwd=process.cwd(),library,dryRun=false,agent='all'}={}){
 if(!['all','codex','claude','gemini'].includes(agent))throw new Error('agent must be all, codex, claude or gemini');
 let state=await inspectProject(cwd,library);
 if(state.projectName==='citual-ui')throw new Error('Connect a consuming application, not the Citual UI library itself.');
 if(!state.react || !['3','4'].includes(String(state.tailwind??'').match(/\d+/)?.[0]))throw new Error('Configure React and Tailwind 3 or 4 before connecting the library.');
 if(!state.framework)throw new Error('Create a React Next.js or Vite application first, then connect Citual UI.');
 if(state.libraryVersion&&!state.versionMatches)throw new Error(`Installed library ${state.libraryVersion} and skill ${state.skillVersion} differ. Review an explicit version update before continuing.`);
 if(dryRun)return {...state,status:'planned',willInstall:!state.libraryRoot || (Boolean(library)&&!state.setup),willConfigure:!state.setup};
 if(!state.libraryRoot){
  const install=spawnSync('npm',['install',snapshot.gitDependency],{cwd:state.root,stdio:'inherit',timeout:120000});
  if(install.status!==0)throw new Error('Private Citual UI installation failed. Check existing GitHub SSH access, or provide --library /path/to/authenticated/checkout. No credentials are requested or stored by this skill.');
  state=await inspectProject(state.root);
  if(!state.libraryRoot||!state.versionMatches)throw new Error('Installed package does not match this skill snapshot. Stop and inspect the dependency.');
 }
 const {initProject}=await import(pathToFileURL(join(state.libraryRoot,'scripts/init-project.mjs')).href);
 const result=await initProject({cwd:state.root,...(library?{source:state.libraryRoot}:{}),agent});
 return {...await inspectProject(state.root,library),status:result.status};
}
export async function main(args=process.argv.slice(2)){
 const {values,positionals}=parseArgs({args,allowPositionals:true,options:{cwd:{type:'string'},library:{type:'string'},agent:{type:'string',default:'all'},'dry-run':{type:'boolean',default:false}}});
 const command=positionals[0]??'status';if(!['status','connect'].includes(command))throw new Error('Use project.mjs status or connect [--dry-run]');
 const options={cwd:values.cwd,library:values.library,agent:values.agent,dryRun:values['dry-run']};console.log(JSON.stringify(command==='status'?await inspectProject(options.cwd,options.library):await connectProject(options),null,2));
}
if(process.argv[1]&&import.meta.url===pathToFileURL(realpathSync(process.argv[1])).href)main().catch(error=>{console.error(error.message);process.exitCode=1;});
