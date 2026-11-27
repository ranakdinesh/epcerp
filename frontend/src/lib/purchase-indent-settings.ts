export const purchaseIndentReadPermissions = [
  "projects.purchase_workflow.view",
  "projects.purchase_workflow.read",
  "projects.purchase_indent_documents.read",
] as const;
export const purchaseIndentSettingsWrite = "settings.purchase_indent_terms.write";
export const purchaseIndentTermsLimit = 12000;
export const purchaseIndentTermsLineLimit = 1000;

export type PurchaseIndentSettings = {
  terms: string;
  version: number;
  can_write: boolean;
};
