import { NextResponse } from "next/server";
import { clearAuthCookies } from "@/lib/auth/cookies";
import { backendApiUrl } from "@/lib/auth/backend-config";
export async function POST(request:Request){
 try{const payload=await request.json();if(typeof payload.token!=="string"||payload.token.length!==64)return Response.json({detail:"Invalid verification link"},{status:422});const r=await fetch(`${backendApiUrl}/auth/verify-email`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({token:payload.token}),cache:"no-store"});const response=new NextResponse(r.status===204?null:await r.text(),{status:r.status,headers:{"Content-Type":"application/json","Cache-Control":"no-store"}});if(r.status===204)clearAuthCookies(response);return response;}catch{return Response.json({detail:"Verification is temporarily unavailable"},{status:503});}
}
