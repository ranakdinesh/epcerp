export function projectStatus(project: { is_active: boolean }) {
  return project.is_active ? "Active" : "Inactive";
}
