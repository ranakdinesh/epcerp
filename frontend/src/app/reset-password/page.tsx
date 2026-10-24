import type { Metadata } from "next";
import RecoveryForm from "@/components/auth/recovery-form";
export const metadata: Metadata = { title: "Reset password | epcERP", robots: { index: false, follow: false }, referrer: "no-referrer" };
export default function ResetPasswordPage() { return <RecoveryForm reset />; }
