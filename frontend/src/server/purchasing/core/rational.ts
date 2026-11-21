import { invalid } from './validation';
import { format, rounded } from './decimal';
const zero = BigInt(0), one = BigInt(1);
function gcd(a: bigint, b: bigint): bigint { a = a < zero ? -a : a; b = b < zero ? -b : b; while (b) {
    const next = a % b;
    a = b;
    b = next;
} return a || one; }
// Immutable rational arithmetic used for pricing, allocations and tax calculations.
export class Rational {
    readonly numerator: bigint;
    readonly denominator: bigint;
    constructor(numerator: bigint, denominator = one) { if (denominator === zero)
        throw new Error('Zero decimal denominator'); if (denominator < zero) {
        numerator = -numerator;
        denominator = -denominator;
    } const divisor = gcd(numerator, denominator); this.numerator = numerator / divisor; this.denominator = denominator / divisor; }
    static integer(value: number | bigint) { return new Rational(BigInt(value)); }
    static cents(value: bigint) { return new Rational(value, BigInt(100)); }
    add(other: Rational) { return new Rational(this.numerator * other.denominator + other.numerator * this.denominator, this.denominator * other.denominator); }
    subtract(other: Rational) { return this.add(new Rational(-other.numerator, other.denominator)); }
    multiply(other: Rational) { return new Rational(this.numerator * other.numerator, this.denominator * other.denominator); }
    divide(other: Rational) { return new Rational(this.numerator * other.denominator, this.denominator * other.numerator); }
    compare(other: Rational) { const difference = this.numerator * other.denominator - other.numerator * this.denominator; return difference < zero ? -1 : difference > zero ? 1 : 0; }
    round(places: number) { return rounded(this.numerator * BigInt(10) ** BigInt(places), this.denominator); }
    fixed(places = 2) { return format(this.round(places), places); }
}
export function amount(raw: string, integers: number, places: number) {
    if (!/^[0-9]+(?:\.[0-9]+)?$/.test(raw) || raw.length > integers + places + 2)
        invalid('amount', 'Prices and charges must be finite nonnegative decimal amounts');
    const [whole, fraction = ''] = raw.split('.');
    if (whole.replace(/^0+/, '').length > integers || fraction.length > places)
        invalid('amount', 'Amount exceeds the supported digits or decimal places');
    return new Rational(BigInt(whole + fraction), BigInt(10) ** BigInt(fraction.length));
}
