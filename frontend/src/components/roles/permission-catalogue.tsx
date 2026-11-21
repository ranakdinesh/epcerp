import type { ReactNode } from "react";
import { DataGrid } from "@/components/ui/data-grid";
import { ValidatedInput } from "@/components/ui/validated-controls";
import { groupPermissions, permissionAction, type Permission } from "./permission-groups";

type PermissionRow = {
  id: string;
  group: string;
  label: string;
  description: string;
  legacy?: boolean;
};

export function PermissionCatalogue({ permissions, selectedCodes, legacyCodes, onToggle, disabled, toolbar }: {
  permissions: Permission[];
  selectedCodes: string[];
  legacyCodes: string[];
  onToggle?: (code: string) => void;
  disabled: boolean;
  toolbar: ReactNode;
}) {
  const selected = new Set(selectedCodes);
  const rows: PermissionRow[] = groupPermissions(permissions).flatMap(group => group.permissions.map(permission => ({
    id: permission.code,
    group: `${group.area} / ${group.label}`,
    label: permissionAction(permission.code),
    description: permission.description,
  })));
  for (const code of legacyCodes) {
    rows.push({ id: code, group: "Other existing assignments", label: code, description: "Existing assignment outside the current permission catalogue.", legacy: true });
  }

  return <DataGrid label="Permissions" rows={rows} toolbar={toolbar} initialSort="group"
    grouping={{ columnKey: "group", key: row => row.group, label: row => row.group }}
    columns={[
      { key: "group", label: "Area / capability", value: row => row.group },
      { key: "permission", label: "Permission", value: row => row.label },
      { key: "description", label: "What it allows", value: row => row.description || "Description unavailable." },
      { key: "grant", label: "Assigned", value: row => selected.has(row.id) || row.legacy ? "Yes" : "No",
        render: row => onToggle && !row.legacy
          ? <ValidatedInput type="checkbox" checked={selected.has(row.id)} disabled={disabled}
              onChange={() => onToggle(row.id)} aria-label={`${row.group}, ${row.label}`} className="size-4 accent-blue-700" />
          : "Yes",
      },
    ]} />;
}
