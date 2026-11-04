import "server-only";
import { backendApiUrl } from "./backend-config";

// Module ownership is explicit. Native services run in-process; unmigrated
// resources use Go. Native failures never fall back to another writer.
export async function backendRequest(input: string | URL, init?: RequestInit): Promise<Response> {
  const url = new URL(input);
  if (url.origin === new URL(backendApiUrl).origin) {
    const path=url.pathname.replace(/^\/api\/v1/,"");
    if(process.env.EPCERP_AUTH_OWNER === "nextjs") {
      const { handleAuth, ownsAuthPath } = await import("@/server/auth/http");
      if (ownsAuthPath(path)) return handleAuth(new Request(url,init));
    }
    if(process.env.EPCERP_CLIENTS_OWNER === "nextjs") {
      const {handleMaster,ownsMasterPath}=await import("@/server/master/http");
      if(ownsMasterPath(path)) return handleMaster(new Request(url,init));
    }
  }
  return fetch(input,init);
}
