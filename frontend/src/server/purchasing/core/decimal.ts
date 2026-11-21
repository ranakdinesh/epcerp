import { invalid } from './validation';
// Decimal quantities and money never pass through binary floating-point arithmetic.
export function fixed(value: string, digits: number, integerDigits: number, field: string, positive = false) {
    if (!(new RegExp(`^[0-9]{1,${integerDigits}}(?:\\.[0-9]{1,${digits}})?$`)).test(value))
        invalid(field, `Use at most ${integerDigits} integer digits and ${digits} decimal places`);
    const [whole, fraction = ''] = value.split('.');
    const result = BigInt(whole + fraction.padEnd(digits, '0'));
    if (positive && result <= BigInt(0))
        invalid(field, 'Value must be positive');
    return result;
}
export function format(value: bigint, digits: number) {
    const sign = value < BigInt(0) ? '-' : '', raw = (value < BigInt(0) ? -value : value).toString().padStart(digits + 1, '0');
    return digits ? sign + raw.slice(0, -digits) + '.' + raw.slice(-digits) : sign + raw;
}
export function rounded(numerator: bigint, denominator: bigint): bigint {
    if (denominator <= BigInt(0))
        throw new Error('Positive decimal denominator required');
    return numerator < BigInt(0) ? -rounded(-numerator, denominator) : (numerator * BigInt(2) + denominator) / (denominator * BigInt(2));
}
export function quantity(value: string) { return fixed(value, 3, 13, 'quantity', true); }
