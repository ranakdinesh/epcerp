import { invalid } from './validation';
export const maximum = BigInt('9999999999999999');
export function money(s: string) {
    if (!/^\d{1,14}(?:\.\d{1,2})?(?![\s\S])/.test(s))
        invalid('amount', 'Budget amounts must be finite, nonnegative and have at most two decimals');
    const [a, b = ''] = s.split('.'), n = BigInt(a) * BigInt(100) + BigInt(b.padEnd(2, '0'));
    if (n > maximum)
        invalid('amount', 'Budget amount exceeds the supported limit');
    return n;
}
export const format = (n: bigint): string => n < BigInt(0) ? '-' + format(-n) : `${n / BigInt(100)}.${String(n % BigInt(100)).padStart(2, '0')}`;
export function qty(s: string) {
    if (!/^\d{1,14}(?:\.\d{1,4})?(?![\s\S])/.test(s))
        invalid('quantity', 'Purchase quantity must be positive and finite');
    const [a, b = ''] = s.split('.'), n = BigInt(a) * BigInt(10000) + BigInt(b.padEnd(4, '0'));
    if (n <= BigInt(0))
        invalid('quantity', 'Purchase quantity must be positive and finite');
    return n;
}
export function lineBudget(q: string, r: string) {
    const v = qty(q) * money(r), n = (v + BigInt(5000)) / BigInt(10000);
    if (n > maximum)
        invalid('amount', 'Item budget exceeds the supported limit');
    return format(n);
}
export function add(a: bigint, b: bigint) {
    if (b > maximum - a)
        invalid('amount', 'Budget totals exceed the supported limit');
    return a + b;
}
