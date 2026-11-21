export type Permission = { code: string; description: string; group?: string };
export type PermissionGroup = { key: string; area: string; label: string; permissions: Permission[] };

const areas: Record<string, string> = {
  settings: "Settings", projects: "Projects", purchasing: "Purchase Workflow",
  tenders: "Tenders", accounts: "Accounts / Finance", administration: "Administration",
  documents: "Documents", system: "System administration", other: "Other permissions",
};
const areaOrder = Object.values(areas);
const resources: Record<string, string> = {
  purchase_bo: "Items to purchase", purchase_indent_groups: "Purchase item grouping",
  purchase_group_vendors: "Group vendor selection", purchase_indent_register: "Vendor indent register",
  group_comparisons: "Group price comparison", group_awards: "Group award approval (reserved)",
  o2te: "O2TE contractual readiness", contractor_work: "Contractor work orders",
  master: "Project master", dashboard: "Project workspace", portfolio: "Portfolio access",
  items: "Items & materials", material_catalogue: "Material catalogue", freeze: "Project freeze controls",
};
export function permissionLabel(value: string): string {
  return resources[value] ?? value.replaceAll("_", " ").replace(/\b\w/g, (letter) => letter.toUpperCase());
}
export function permissionAction(code: string): string {
  if (code === "projects.purchase_bo.complete") return "Finalise and reopen purchase item lists";
  if (code === "projects.purchase_bo.write") return "Prepare purchase item lists";
  if (code === "projects.purchase_bo.read") return "View purchase item lists";
  return code === "system:admin" ? "Full administration" : permissionLabel(code.split(".").at(-1) ?? code);
}
export function groupPermissions(permissions: Permission[], query = ""): PermissionGroup[] {
  const groups = new Map<string, PermissionGroup>();
  const search = query.toLowerCase().split(/\s+/).filter(Boolean);
  for (const permission of permissions) {
    const parts = permission.code.split(".");
    const fallback = permission.code === "system:admin" ? "system" : parts.length >= 3 ? parts[0] : "other";
    const areaCode = permission.group ?? (fallback === "admin" ? "administration" : fallback);
    const area = areas[areaCode] ?? permissionLabel(areaCode);
    const resource = parts.length >= 3 ? parts.slice(1, -1).join(".") : "general";
    const label = resource === "general" ? "General permissions" : permissionLabel(resource);
    const searchable = `${area} ${label} ${permissionAction(permission.code)} ${permission.code} ${permission.description}`.toLowerCase();
    if (!search.every(term => searchable.includes(term))) continue;
    const key = `${areaCode}.${resource}`;
    const group = groups.get(key) ?? { key, area, label, permissions: [] };
    group.permissions.push(permission);
    groups.set(key, group);
  }
  return [...groups.values()].sort((a, b) => areaOrder.indexOf(a.area) - areaOrder.indexOf(b.area) || a.label.localeCompare(b.label))
    .map((group) => ({ ...group, permissions: group.permissions.sort((a, b) => {
      const order = ["view", "read", "write", "execute", "review", "approve", "release", "complete"];
      const rank = (code: string) => { const index = order.indexOf(code.split(".").at(-1) ?? ""); return index < 0 ? order.length : index; };
      return rank(a.code) - rank(b.code) || a.code.localeCompare(b.code);
    }) }));
}
