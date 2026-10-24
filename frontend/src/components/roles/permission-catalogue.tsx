import { ValidatedInput, ValidatedLabel } from "@/components/ui/validated-controls";
import { groupPermissions, permissionAction, type Permission } from "./permission-groups";

type Props = {
  permissions: Permission[];
  query?: string;
  selectedCodes?: string[];
  onToggle?: (code: string) => void;
};

export function PermissionCatalogue({ permissions, query = "", selectedCodes = [], onToggle }: Props) {
  const groups = groupPermissions(permissions, query);
  const areas = [...new Set(groups.map((group) => group.area))];
  const selected = new Set(selectedCodes);
  return <div className="space-y-3">
    {areas.map((area) => {
      const capabilities = groups.filter((group) => group.area === area);
      const codes = capabilities.flatMap((group) => group.permissions.map((permission) => permission.code));
      const count = codes.filter((code) => selected.has(code)).length;
      return <details key={`${area}-${Boolean(query.trim())}`} open={Boolean(query.trim()) || area === "Administration"} className="rounded-xl border border-slate-200 bg-white">
        <summary className="min-h-11 cursor-pointer rounded-xl px-4 py-3 text-sm font-bold text-slate-900 focus-visible:outline-2 focus-visible:outline-blue-600">
          {area}<span className="ml-2 text-xs font-normal text-slate-600">{onToggle ? `${count} selected / ` : ""}{codes.length} permissions</span>
        </summary>
        <div className="space-y-4 border-t border-slate-100 p-3 sm:p-4">
          {capabilities.map((group) => <fieldset key={group.key} className="min-w-0">
            <legend className="mb-2 text-sm font-bold text-slate-800">{group.label}</legend>
            <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
              {group.permissions.map((permission) => {
                const content = <span className="min-w-0">
                  <span className="block text-sm font-bold text-slate-900">{permissionAction(permission.code)}</span>
                  <span className="mt-1 block text-xs leading-5 text-slate-600">{permission.description || "Description unavailable for this legacy permission."}</span>
                  <span className="mt-1 block break-all font-mono text-xs text-slate-500">{permission.code}</span>
                </span>;
                return onToggle ? <ValidatedLabel key={permission.code} className={`flex min-h-11 cursor-pointer items-start gap-3 rounded-lg border p-3 hover:border-blue-400 ${selected.has(permission.code) ? "border-blue-400 bg-blue-50" : "border-slate-200"}`}>
                  <ValidatedInput type="checkbox" aria-label={`${group.label}: ${permissionAction(permission.code)}`} checked={selected.has(permission.code)} onChange={() => onToggle(permission.code)} className="mt-1 h-4 w-4 shrink-0 accent-blue-700" />
                  {content}
                </ValidatedLabel> : <div key={permission.code} className="rounded-lg border border-slate-200 p-3">{content}</div>;
              })}
            </div>
          </fieldset>)}
        </div>
      </details>;
    })}
    {!groups.length ? <p role="status" className="p-3 text-sm text-slate-600">No permissions match your search.</p> : null}
  </div>;
}
