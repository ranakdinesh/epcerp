import { AppError } from "../../errors";
import { type ReferenceKind } from "../shared/references";
import { spacedName, trim, upper } from "../shared/text";
export type ReferenceInput = {
    name: string;
    code: string;
    state_id: string | null;
};
export const referenceKinds = ["client-types", "industries", "states", "cities"] as const;
export function referenceKind(value: string): ReferenceKind {
    if (!referenceKinds.some(k => k === value))
        throw new AppError(404, "Unknown reference master");
    return value as ReferenceKind;
}
export function referencePermission(kind: ReferenceKind, write = false) {
    const area = kind === "client-types" ? "client_types" : kind === "industries" ? "industries" : "locations";
    return `settings.${area}.${write ? "write" : "read"}`;
}
export function prepareReference(kind: ReferenceKind, input: ReferenceInput): ReferenceInput {
    const name = spacedName(input.name);
    if (!name)
        throw new AppError(422, "A name is required");
    // Match Go: trim and uppercase without expansions, replace runs, trim, then cap.
    const code = upper(trim(input.code || name)).replace(/[^A-Z0-9]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 32);
    if (!code)
        throw new AppError(422, "A valid code is required");
    if (kind === "cities" && !input.state_id)
        throw new AppError(422, "A city must be linked to a state or union territory");
    if (kind !== "cities" && input.state_id)
        throw new AppError(422, "Only cities may be linked to a state or union territory");
    return { name, code, state_id: input.state_id };
}
