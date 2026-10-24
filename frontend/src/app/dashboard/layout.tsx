import { redirect } from "next/navigation";

import { AuthUserProvider } from "@/components/auth-user-provider";
import { getAuthenticatedUser } from "@/lib/auth/session";

export default async function DashboardLayout({ children }: { children: React.ReactNode }) {
  const user = await getAuthenticatedUser();
  if (!user) {
    redirect("/auth/renew?next=/dashboard");
  }

  if(user.must_change_password) redirect("/change-password");
  return <AuthUserProvider user={user}>{children}</AuthUserProvider>;
}
