/** Random helpers mirroring 3x-ui RandomUtil behaviour. */

function cryptoBytes(n: number): Uint8Array {
  const buf = new Uint8Array(n)
  crypto.getRandomValues(buf)
  return buf
}

export function randomInteger(min: number, max: number): number {
  const range = max - min + 1
  const arr = cryptoBytes(4)
  const view = new DataView(arr.buffer)
  return min + (view.getUint32(0) % range)
}

export function randomLowerAndNum(len: number): string {
  const alphabet = 'abcdefghijklmnopqrstuvwxyz0123456789'
  const bytes = cryptoBytes(len)
  let out = ''
  for (let i = 0; i < len; i++) out += alphabet[bytes[i] % alphabet.length]
  return out
}

export function randomSeq(len: number): string {
  const alphabet = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
  const bytes = cryptoBytes(len)
  let out = ''
  for (let i = 0; i < len; i++) out += alphabet[bytes[i] % alphabet.length]
  return out
}

export function randomHex(len: number): string {
  const bytes = cryptoBytes(Math.ceil(len / 2))
  return Array.from(bytes).map((b) => b.toString(16).padStart(2, '0')).join('').slice(0, len)
}

/** 3x-ui style: 8 shortIds of lengths 2..16 (even). */
export function randomShortIds(): string[] {
  const lens = [2, 4, 6, 8, 10, 12, 14, 16]
  for (let i = lens.length - 1; i > 0; i--) {
    const j = randomInteger(0, i)
    ;[lens[i], lens[j]] = [lens[j], lens[i]]
  }
  return lens.map((n) => randomHex(n))
}

export function randomUUID(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  const b = cryptoBytes(16)
  b[6] = (b[6] & 0x0f) | 0x40
  b[8] = (b[8] & 0x3f) | 0x80
  const h = Array.from(b).map((x) => x.toString(16).padStart(2, '0')).join('')
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`
}

export function randomSpiderX(): string {
  return `/${randomSeq(15)}`
}
