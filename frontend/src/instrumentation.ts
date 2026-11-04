export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs" && process.env.EPCERP_AUTH_OWNER === "nextjs") {
    const { initializeAuth } = await import("./server/auth/bootstrap");
    await initializeAuth();
    const { startCredentialMail } = await import("./server/auth/mail");
    startCredentialMail();
  }
}
