import { recoveryRequest } from "@/lib/auth/recovery-proxy";
export async function POST(request: Request) { return recoveryRequest(request, "reset-password"); }
