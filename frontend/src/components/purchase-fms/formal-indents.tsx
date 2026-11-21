"use client";

import { useSearchParams } from "next/navigation";
import { BOExecution } from "./bo-workspace";
import { TypeIndents } from "./type-indents";

export function IndentExecution({ project, projectName }: { project: string; projectName: string }) {
  const params = useSearchParams();
  if (params.get("indentTab") === "register" && params.has("bo"))
    return <BOExecution key={project} project={project} projectName={projectName} step="PUR-02" />;
  return <TypeIndents key={project} project={project} projectName={projectName} />;
}
