export function prefixOptions<T extends { name: string }>(options: readonly T[], query: string): T[] {
  const prefix = query.trim().toLocaleLowerCase("en-IN");
  return options.filter(option => option.name.trim().toLocaleLowerCase("en-IN").startsWith(prefix))
    .sort((a, b) => a.name.localeCompare(b.name, "en-IN", { sensitivity: "base" }) || a.name.localeCompare(b.name));
}
