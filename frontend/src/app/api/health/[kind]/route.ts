import {health} from '@/server/platform/health';
export const runtime='nodejs';
export async function GET(_:Request,context:{params:Promise<{kind:string}>}){const{kind}=await context.params;return ['live','ready'].includes(kind)?health(kind==='ready'):new Response(null,{status:404});}
