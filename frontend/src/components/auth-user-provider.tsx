"use client";

import { createContext, useContext, useEffect } from "react";
import { useRouter } from "next/navigation";

import { PersonalProfileProvider } from "@/components/personal-profile";
import type { AuthenticatedUser } from "@/lib/auth/session";

import { renewSession, withSessionLock } from "@/lib/auth/renew-session";

const AuthUserContext = createContext<AuthenticatedUser | null>(null);

export function AuthUserProvider({
  user,
  children,
}: {
  user: AuthenticatedUser;
  children: React.ReactNode;
}) {
  const router = useRouter();
  useEffect(() => {
    const originalFetch = window.fetch;
    window.fetch = async (...args) => {
      const url = new URL(args[0] instanceof Request ? args[0].url : String(args[0]), window.location.href);
      if (url.origin === window.location.origin && url.pathname === "/api/auth/logout") {
        return withSessionLock(() => originalFetch(...args));
      }
      // Clone Request bodies before the first attempt so a protected write can be retried.
      const retryArgs: Parameters<typeof fetch> = [args[0] instanceof Request ? args[0].clone() : args[0], args[1]];
      const securityChange = url.origin === window.location.origin && url.pathname === "/api/auth/me/password";
      const send = (values: Parameters<typeof fetch>) => securityChange ? withSessionLock(() => originalFetch(...values)) : originalFetch(...values);
      let response = await send(args);
      if (response.headers.get("X-VTPL-Session-Expired") === "1") {
        const renewed = await renewSession(originalFetch);
        if (renewed.ok) response = await send(retryArgs);
        else if (renewed.status !== 401) return renewed;
        if (!renewed.ok || response.headers.get("X-VTPL-Session-Expired") === "1") {
          router.replace("/login?session=expired&next=/dashboard");
          router.refresh();
        }
      }
      return response;
    };
    return () => { window.fetch = originalFetch; };
  }, [router]);
  return <AuthUserContext value={user}><PersonalProfileProvider>{children}</PersonalProfileProvider></AuthUserContext>;
}

export function useAuthenticatedUser(): AuthenticatedUser {
  const user = useContext(AuthUserContext);
  if (!user) {
    throw new Error("useAuthenticatedUser must be used inside AuthUserProvider");
  }
  return user;
}
