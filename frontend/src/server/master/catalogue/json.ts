// Node 22+ retains the source token in JSON.parse revivers. Preserve Go's JSON
// number semantics: int32 request fields reject decimal/exponent tokens, while
// arbitrary legacy JSON keeps large integers and decimals without rounding.
export function parseCatalogueJSON(source: string): unknown {
    const parse = JSON.parse as (source: string, reviver: (key: string, value: unknown, context?: {
        source: string;
    }) => unknown) => unknown;
    const raw = (JSON as typeof JSON & {
        rawJSON: (source: string) => unknown;
    }).rawJSON;
    return parse(source, (_key, value, context) => typeof value === "number" && context && (!Number.isSafeInteger(value) || !(/^-?(?:0|[1-9]\d*)$/).test(context.source)) ? raw(context.source) : value);
}
