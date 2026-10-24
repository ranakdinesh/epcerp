import type { ReactNode } from "react";
import type { PurchaseProject } from "./data";

/** Project identity belongs to the process shell, not individual stages. */
export function PurchaseProjectHeader({project, status}: {project: PurchaseProject; status: string}) {
  return <header className="purchase-project-header">
    <div className="purchase-project-identity">
      <h1><span>{project.project_number}</span><span> · </span>{project.name}</h1>
      <span className="purchase-project-state">{project.is_active ? project.lifecycle_status.replaceAll("_", " ") : "Inactive"}</span>
    </div>
    <div className="purchase-progress-strip" role="status">
      <strong>Purchase progress: {project.progress_percent === null ? "Not recorded" : `${project.progress_percent}%`}</strong>
      <span>{status}</span>
    </div>
  </header>;
}

/** Consistent hierarchy and right-aligned actions for stage and package screens. */
export function PurchaseSectionHeader({title, description, children}: {title: string; description?: ReactNode; children?: ReactNode}) {
  return <header className="purchase-section-header">
    <div className="purchase-section-heading"><h2>{title}</h2>{description && <p>{description}</p>}</div>
    {children && <div className="purchase-section-actions">{children}</div>}
  </header>;
}
