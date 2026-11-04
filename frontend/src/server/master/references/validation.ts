import { canonicalID, MasterValidationError, strictObject, stringField } from "../shared/validation";
import { lower } from "../shared/text";
import type { ReferenceInput } from "./domain";
export function parseReference(value: unknown): ReferenceInput {
    const input = strictObject(value, ["name", "code", "state_id"]);
    return { name: stringField(input, "name", 2, 128), code: stringField(input, "code", 0, 32, ""),
        state_id: input.state_id == null ? null : canonicalID(input.state_id, "state_id") };
}
export function parseReferenceQuery(query: URLSearchParams) {
    const state = query.has("state_id") ? canonicalID(query.get("state_id"), "state_id", "query") : null;
    let include = false;
    if (query.has("include_inactive")) {
        const value = lower(query.get("include_inactive")!);
        if (["1", "true", "yes", "y", "on", "t"].includes(value))
            include = true;
        else if (!["0", "false", "no", "n", "off", "f"].includes(value))
            throw new MasterValidationError("include_inactive", "query");
    }
    return { state, activeOnly: !include };
}
