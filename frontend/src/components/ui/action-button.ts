export type ActionVariant = "primary" | "secondary" | "danger" | "success" | "warning";

/** Shared action hierarchy for native buttons and links. */
export function actionButtonClass(variant: ActionVariant = "secondary", iconOnly = false) {
  return `ui-action ui-action--${variant}${iconOnly ? " ui-action--icon" : ""}`;
}
