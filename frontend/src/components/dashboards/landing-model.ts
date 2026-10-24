export function dashboardLanding(permissions: string[]) {
  if (permissions.includes("system:admin")) return "portfolio";
  if (permissions.includes("projects.portfolio.write")) return "manager";
  if (
    permissions.includes("projects.purchase_workflow.view") &&
    ["write", "execute", "resolve"].some((action) =>
      permissions.includes(`projects.purchase_workflow.${action}`),
    )
  )
    return "purchase";
  return "portfolio";
}
