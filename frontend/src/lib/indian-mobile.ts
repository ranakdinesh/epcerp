export const indianMobileLength = 10;
export const indianMobilePattern = `[0-9]{${indianMobileLength}}`;
export const indianMobileMessage = "Enter a 10-digit mobile number without +91.";

const mobile = new RegExp(`^${indianMobilePattern}$`);
export function isIndianMobile(value: unknown): value is string {
  return typeof value === "string" && value.length === indianMobileLength && mobile.test(value);
}
