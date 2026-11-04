export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs" && process.env.EPCERP_AUTH_OWNER === "nextjs") {
    const { initializeAuth } = await import("./server/auth/bootstrap");
    await initializeAuth();
    const { startCredentialMail } = await import("./server/auth/mail");
    startCredentialMail();
  }
  if (process.env.NEXT_RUNTIME === "nodejs" && process.env.EPCERP_TENDER_INTAKE_OWNER === "nextjs") {
    const { startPublisher } = await import("./server/tenders/intake/publisher");
    startPublisher();
  }
  if (process.env.NEXT_RUNTIME === "nodejs" && process.env.EPCERP_TSC_OWNER === "nextjs") {
    const { startWorker } = await import("./server/tenders/tsc/worker");
    startWorker();
  }
}
