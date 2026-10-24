export function activationValidationMessage({ currentStatus, targetStatus, loiNumber, poNumber, evidenceReady }: {
  currentStatus: string; targetStatus: string; loiNumber: string; poNumber: string; evidenceReady: boolean | null;
}): string {
  if (targetStatus !== "active" || currentStatus === "active") return "";
  return [
    !loiNumber.trim() && !poNumber.trim() ? "Enter an LOI or PO reference." : "",
    evidenceReady === false ? "Upload qualifying contract evidence using Add evidence." : "",
  ].filter(Boolean).join(" ");
}
