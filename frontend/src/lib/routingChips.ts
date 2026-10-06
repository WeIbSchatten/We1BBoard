/** Common geosite/geoip values for routing rule quick-insert (3x-ui style). */

export const DOMAIN_CHIPS = [
  'geosite:google',
  'geosite:netflix',
  'geosite:telegram',
  'geosite:category-ads-all',
  'geosite:cn',
  'domain:example.com',
] as const

export const IP_CHIPS = [
  'geoip:cn',
  'geoip:private',
  'geoip:cloudflare',
] as const

/** Append a value to a comma-separated field; skips duplicates (case-insensitive). */
export function appendCsv(current: string, value: string): string {
  const parts = current.split(',').map((s) => s.trim()).filter(Boolean)
  const lower = value.toLowerCase()
  if (parts.some((p) => p.toLowerCase() === lower)) return parts.join(', ')
  return parts.length ? `${parts.join(', ')}, ${value}` : value
}

export function csvHas(current: string, value: string): boolean {
  const lower = value.toLowerCase()
  return current.split(',').map((s) => s.trim()).filter(Boolean)
    .some((p) => p.toLowerCase() === lower)
}
