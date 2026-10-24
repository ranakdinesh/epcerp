export function permitted(permissions: readonly string[], code: string): boolean {
  return permissions.includes("system:admin") || permissions.includes(code);
}

export function canOpen(permissions: readonly string[], resource: string): boolean {
  return permitted(permissions, `${resource}.view`) && permitted(permissions, `${resource}.read`);
}

export const indentTabPermissions = {
  assignment: "projects.purchase_indent_groups.view",
  vendors: "projects.purchase_group_vendors.view",
  report: "projects.purchase_item_vendor_report.view",
  preview: "projects.purchase_indent_register.view",
  document: "projects.purchase_indent_documents.view",
} as const;

export function indentTabs(permissions: readonly string[]) {
  if (!permitted(permissions, "projects.purchase_indent_groups.read")) return [];
  return (Object.keys(indentTabPermissions) as (keyof typeof indentTabPermissions)[])
    .filter(tab => permitted(permissions, indentTabPermissions[tab]) &&
      (!(tab === "preview" || tab === "document") ||
        permitted(permissions, "projects.purchase_indent_documents.read")));
}

export function purchaseStepAllowed(permissions: readonly string[], step: number): boolean {
  if (!Number.isInteger(step) || step < 1 || step > 20) return false;
  if (step === 1) return canOpen(permissions, "projects.purchase_bo");
  if (step === 2) return canOpen(permissions,"projects.purchase_indents") || indentTabs(permissions).length > 0;
  if (step === 3) return canOpen(permissions, "projects.group_quotations");
  if (step === 4) return canOpen(permissions, "projects.group_comparisons");
  if (step === 5) return permitted(permissions, "projects.group_awards.view");
  if (step <= 9) return permitted(permissions, "projects.purchase_sourcing.read") &&
    permitted(permissions, "projects.purchase_workflow.view");
  if (step <= 14) return canOpen(permissions, "projects.purchase_inspection");
  return canOpen(permissions, "projects.purchase_delivery");
}

const projectTabResources: Record<string, string> = {
  overview: "dashboard", control: "delivery", processes: "processes",
  items: "items", budget: "budgets", planning: "planning", schedule: "schedule",
  delivery: "o2te", team: "team", contractor_work: "contractor_work",
};

export function projectTabAllowed(permissions: readonly string[], tab: string): boolean {
  if (tab === "purchasing") return [
    "purchase_workflow", "purchase_indents", "purchase_holds", "purchase_bo",
    "group_quotations", "group_comparisons",
  ].some(area => canOpen(permissions, "projects." + area)) || indentTabs(permissions).length > 0;
  if (tab === "templates") return canOpen(permissions, "settings.task_templates");
  return !!projectTabResources[tab] && canOpen(permissions, "projects." + projectTabResources[tab]);
}
