// Baht <-> satang conversion using string/BigInt arithmetic only — never a
// float, never a lossy numeric parse of the amount (money is always integer
// satang, per pkg/money's Go convention this mirrors). Imports nothing so
// this file compiles standalone.

const BAHT_PATTERN = /^\d{1,6}(\.\d{1,2})?$/;
const MAX_SATANG = BigInt(10_000_000);
const HUNDRED = BigInt(100);

/**
 * Converts a baht amount string (e.g. "150.50") to an integer-satang string
 * (e.g. "15050"). Returns null for anything that isn't a plain non-negative
 * decimal with at most 2 fraction digits, or that exceeds the 10,000,000
 * satang bound (mirrors services/catalog's RoutePrice.Validate).
 */
export function bahtToSatang(input: string): string | null {
  if (!BAHT_PATTERN.test(input)) return null;
  const [intPart, fracPart = ''] = input.split('.');
  const paddedFrac = fracPart.padEnd(2, '0');
  const satang =
    BigInt(intPart) * HUNDRED + BigInt(paddedFrac === '' ? '0' : paddedFrac);
  if (satang > MAX_SATANG) return null;
  return satang.toString();
}

/** Formats an integer-satang string as a ฿-prefixed, thousands-grouped baht display string. */
export function formatSatang(amountSatang: string): string {
  const n = BigInt(amountSatang);
  const baht = n / HUNDRED;
  const frac = n % HUNDRED;
  const bahtStr = baht.toString();
  const grouped = bahtStr.replace(/\B(?=(\d{3})+(?!\d))/g, ',');
  return `฿${grouped}.${frac.toString().padStart(2, '0')}`;
}
