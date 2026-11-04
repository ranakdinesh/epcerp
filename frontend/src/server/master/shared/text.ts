import pairs from "./simple-fold.json";
// Generated from the actual Go helper, not JavaScript's expanding lowercase.
// This keeps every persisted duplicate-name key compatible across runtimes.
const foldMap = new Map<number, number>(pairs.map(([r, canonical]) => [r, canonical]));
const lowerMap = new Map<number, number>(pairs.map(([r, , lower]) => [r, lower]));
const upperMap = new Map<number, number>(pairs.map(([r, , , upper]) => [r, upper]));
export const goSpace = /[\u0009-\u000d\u0020\u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]/u;
export function trim(value: string) {
    return value.replace(/^[\u0009-\u000d\u0020\u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+|[\u0009-\u000d\u0020\u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+$/gu, "");
}
export function fold(value: string) {
    return [...value].map(c => String.fromCodePoint(foldMap.get(c.codePointAt(0)!) ?? c.codePointAt(0)!)).join("");
}
export function lower(value: string) {
    return [...value].map(c => String.fromCodePoint(lowerMap.get(c.codePointAt(0)!) ?? c.codePointAt(0)!)).join("");
}
export function upper(value: string) {
    return [...value].map(c => String.fromCodePoint(upperMap.get(c.codePointAt(0)!) ?? c.codePointAt(0)!)).join("");
}
export function normalizedName(value: string) {
    return fold(spacedName(value));
}
export function spacedName(value: string) {
    return value.split(/[\u0009-\u000d\u0020\u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+/u).filter(Boolean).join(" ");
}
