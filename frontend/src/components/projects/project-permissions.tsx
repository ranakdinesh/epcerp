"use client";

import { createContext, useContext, type ReactNode } from "react";
import { useAuthenticatedUser } from "@/components/auth-user-provider";

const ProjectPermissions = createContext<string[] | null>(null);

export function ProjectPermissionProvider({ permissions, children }: {
  permissions: string[];
  children: ReactNode;
}) {
  return <ProjectPermissions.Provider value={permissions}>{children}</ProjectPermissions.Provider>;
}

export function useProjectUser() {
  const user = useAuthenticatedUser();
  const scoped = useContext(ProjectPermissions);
  return { ...user, permissions: scoped ?? user.permissions };
}
