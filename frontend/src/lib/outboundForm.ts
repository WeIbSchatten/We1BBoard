/** Build/parse outbound settings like 3x-ui structured forms. */

import { randomUUID } from './random'

export type OutboundFormState = {
  id?: number
  tag: string
  protocol: string
  enable: boolean
  remark: string
  // proxy fields
  address: string
  port: number
  uuid: string
  password: string
  flow: string
  method: string
  email: string
  // stream
  network: string
  security: string
  sni: string
  publicKey: string
  shortId: string
  fingerprint: string
  path: string
  host: string
  serviceName: string
  // freedom/blackhole
  domainStrategy: string
  // dns outbound
  dnsNetwork: string
  dnsBlock: boolean
}

export function emptyOutboundForm(): OutboundFormState {
  return {
    tag: '',
    protocol: 'vless',
    enable: true,
    remark: '',
    address: '',
    port: 443,
    uuid: '',
    password: '',
    flow: '',
    method: '2022-blake3-aes-128-gcm',
    email: '',
    network: 'tcp',
    security: 'none',
    sni: '',
    publicKey: '',
    shortId: '',
    fingerprint: 'chrome',
    path: '/',
    host: '',
    serviceName: '',
    domainStrategy: 'AsIs',
    dnsNetwork: 'udp',
    dnsBlock: false,
  }
}

export function parseOutboundToForm(o: {
  id: number
  tag: string
  protocol: string
  enable: boolean
  remark: string
  settings: string
  streamSettings: string
}): OutboundFormState {
  const f = emptyOutboundForm()
  f.id = o.id
  f.tag = o.tag
  f.protocol = o.protocol
  f.enable = o.enable
  f.remark = o.remark || ''
  try {
    const s = JSON.parse(o.settings || '{}') as Record<string, unknown>
    if (o.protocol === 'freedom') {
      f.domainStrategy = String(s.domainStrategy || 'AsIs')
    } else if (o.protocol === 'dns') {
      f.address = String(s.address || '')
      f.port = Number(s.port || 53)
      f.dnsNetwork = String(s.network || 'udp')
      f.dnsBlock = !!s.block
    } else if (['vless', 'vmess', 'trojan', 'shadowsocks', 'socks', 'http'].includes(o.protocol)) {
      const servers = (s.servers || s.vnext || []) as Record<string, unknown>[]
      const first = servers[0] || {}
      f.address = String(first.address || '')
      f.port = Number(first.port || 443)
      if (o.protocol === 'shadowsocks' || o.protocol === 'trojan' || o.protocol === 'socks' || o.protocol === 'http') {
        f.password = String(first.password || (first.users as Record<string, string>[] | undefined)?.[0]?.pass || '')
        f.method = String(first.method || f.method)
        f.email = String(first.email || (first.users as Record<string, string>[] | undefined)?.[0]?.user || '')
      }
      if (o.protocol === 'vless' || o.protocol === 'vmess') {
        const users = (first.users || []) as Record<string, unknown>[]
        const u = users[0] || {}
        f.uuid = String(u.id || '')
        f.flow = String(u.flow || '')
        f.email = String(u.email || '')
      }
    }
  } catch { /* */ }
  try {
    const stream = JSON.parse(o.streamSettings || '{}') as Record<string, unknown>
    f.network = String(stream.network || 'tcp')
    f.security = String(stream.security || 'none')
    const tls = (stream.tlsSettings || stream.realitySettings || {}) as Record<string, unknown>
    f.sni = String(tls.serverName || (tls.serverNames as string[] | undefined)?.[0] || '')
    f.publicKey = String(tls.publicKey || (tls.settings as Record<string, string> | undefined)?.publicKey || '')
    f.shortId = String((tls.shortIds as string[] | undefined)?.[0] || tls.shortId || '')
    f.fingerprint = String(tls.fingerprint || (tls.settings as Record<string, string> | undefined)?.fingerprint || 'chrome')
    const ws = (stream.wsSettings || {}) as Record<string, unknown>
    f.path = String(ws.path || (stream.xhttpSettings as Record<string, string> | undefined)?.path || '/')
    f.host = String((ws.headers as Record<string, string> | undefined)?.Host || '')
    f.serviceName = String((stream.grpcSettings as Record<string, string> | undefined)?.serviceName || '')
  } catch { /* */ }
  return f
}

export function buildOutboundSettings(f: OutboundFormState): string {
  switch (f.protocol) {
    case 'freedom':
      return JSON.stringify({ domainStrategy: f.domainStrategy || 'AsIs' })
    case 'blackhole':
      return JSON.stringify({})
    case 'dns': {
      const settings: Record<string, unknown> = {
        port: f.port || 53,
        network: f.dnsNetwork || 'udp',
        block: !!f.dnsBlock,
      }
      if (f.address.trim()) settings.address = f.address.trim()
      return JSON.stringify(settings)
    }
    case 'vless':
      return JSON.stringify({
        vnext: [{
          address: f.address,
          port: f.port,
          users: [{ id: f.uuid || randomUUID(), encryption: 'none', flow: f.flow || '', email: f.email || '' }],
        }],
      })
    case 'vmess':
      return JSON.stringify({
        vnext: [{
          address: f.address,
          port: f.port,
          users: [{ id: f.uuid || randomUUID(), alterId: 0, security: 'auto', email: f.email || '' }],
        }],
      })
    case 'trojan':
      return JSON.stringify({
        servers: [{ address: f.address, port: f.port, password: f.password, email: f.email || '' }],
      })
    case 'shadowsocks':
      return JSON.stringify({
        servers: [{ address: f.address, port: f.port, method: f.method, password: f.password, email: f.email || '' }],
      })
    case 'socks':
      return JSON.stringify({
        servers: [{
          address: f.address,
          port: f.port || 1080,
          users: f.email || f.password ? [{ user: f.email, pass: f.password }] : [],
        }],
      })
    case 'http':
      return JSON.stringify({
        servers: [{
          address: f.address,
          port: f.port || 8080,
          users: f.email || f.password ? [{ user: f.email, pass: f.password }] : [],
        }],
      })
    default:
      return '{}'
  }
}

export function buildOutboundStream(f: OutboundFormState): string {
  if (['freedom', 'blackhole', 'dns'].includes(f.protocol)) return '{}'
  const stream: Record<string, unknown> = {
    network: f.network || 'tcp',
    security: f.security || 'none',
  }
  if (f.network === 'ws') {
    stream.wsSettings = { path: f.path || '/', headers: f.host ? { Host: f.host } : {} }
  } else if (f.network === 'grpc') {
    stream.grpcSettings = { serviceName: f.serviceName || '' }
  } else if (f.network === 'xhttp') {
    stream.xhttpSettings = { path: f.path || '/', mode: 'auto' }
  } else {
    stream.tcpSettings = { header: { type: 'none' } }
  }
  if (f.security === 'tls') {
    stream.tlsSettings = {
      serverName: f.sni || '',
      allowInsecure: false,
      fingerprint: f.fingerprint || 'chrome',
    }
  }
  if (f.security === 'reality') {
    stream.realitySettings = {
      serverName: f.sni || '',
      fingerprint: f.fingerprint || 'chrome',
      publicKey: f.publicKey || '',
      shortId: f.shortId || '',
      spiderX: '/',
    }
  }
  return JSON.stringify(stream)
}

export const OUTBOUND_PROTOCOLS = [
  'vless', 'vmess', 'trojan', 'shadowsocks', 'socks', 'http', 'freedom', 'blackhole', 'dns',
]

const SHARE_LINK_RE = /^(vmess|vless|trojan|ss|hysteria2|hy2|wireguard|wg):\/\//i

function looksLikeBase64Blob(text: string): boolean {
  const compact = text.replace(/\s+/g, '')
  if (compact.length < 16) return false
  if (/:\/\//.test(text)) return false
  return /^[A-Za-z0-9+/_-]+={0,2}$/.test(compact)
}

function decodeBase64Blob(text: string): string {
  const compact = text.replace(/\s+/g, '')
  const normalized = compact.replace(/-/g, '+').replace(/_/g, '/')
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=')
  try {
    if (typeof atob === 'function') {
      return new TextDecoder().decode(Uint8Array.from(atob(padded), (c) => c.charCodeAt(0)))
    }
  } catch { /* fall through */ }
  return text
}

/** Extract shareable proxy links from pasted text, a subscription URL, or a base64 blob. */
export function extractShareLinks(text: string): string[] {
  const trimmed = text.trim()
  if (!trimmed) return []
  if (/^https?:\/\//i.test(trimmed)) return [trimmed]

  let body = trimmed
  if (looksLikeBase64Blob(trimmed)) {
    body = decodeBase64Blob(trimmed)
  }

  return body
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => SHARE_LINK_RE.test(line))
}

function asRecord(v: unknown): Record<string, unknown> {
  return v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : {}
}

function asArray(v: unknown): Record<string, unknown>[] {
  return Array.isArray(v) ? (v as Record<string, unknown>[]) : []
}

/** Map parseOutboundLink result into OutboundFormState. */
export function applyParsedOutboundLink(
  parsed: Record<string, unknown>,
  keepTag?: string,
): OutboundFormState {
  const f = emptyOutboundForm()
  const protocol = String(parsed.protocol || 'vless')
  f.protocol = protocol === 'hysteria' ? 'hysteria' : protocol
  const tag = String(parsed.tag || '').trim()
  f.tag = tag || keepTag || ''
  f.remark = tag || f.remark

  const settings = asRecord(parsed.settings)
  // vless flat: {address, port, id, flow}
  if (settings.address != null || settings.id != null) {
    f.address = String(settings.address || '')
    f.port = Number(settings.port || 443)
    f.uuid = String(settings.id || '')
    f.flow = String(settings.flow || '')
    if (settings.password != null) f.password = String(settings.password)
    if (settings.method != null) f.method = String(settings.method)
  }

  // vmess/trojan/ss: vnext / servers
  const servers = asArray(settings.vnext).length
    ? asArray(settings.vnext)
    : asArray(settings.servers)
  if (servers.length > 0) {
    const first = servers[0] || {}
    f.address = String(first.address || f.address)
    f.port = Number(first.port || f.port || 443)
    if (first.password != null) f.password = String(first.password)
    if (first.method != null) f.method = String(first.method)
    if (first.email != null) f.email = String(first.email)
    const users = asArray(first.users)
    if (users.length > 0) {
      const u = users[0] || {}
      if (u.id != null) f.uuid = String(u.id)
      if (u.flow != null) f.flow = String(u.flow)
      if (u.email != null) f.email = String(u.email)
      if (u.pass != null) f.password = String(u.pass)
      if (u.user != null) f.email = String(u.user)
    }
  }

  const stream = asRecord(parsed.streamSettings)
  if (Object.keys(stream).length > 0) {
    f.network = String(stream.network || 'tcp')
    f.security = String(stream.security || 'none')

    const tls = asRecord(stream.tlsSettings)
    const reality = asRecord(stream.realitySettings)
    const sec = Object.keys(reality).length > 0 ? reality : tls
    f.sni = String(
      sec.serverName
      || (Array.isArray(sec.serverNames) ? sec.serverNames[0] : '')
      || '',
    )
    const nested = asRecord(sec.settings)
    f.publicKey = String(sec.publicKey || nested.publicKey || '')
    f.shortId = String(
      (Array.isArray(sec.shortIds) ? sec.shortIds[0] : '')
      || sec.shortId
      || '',
    )
    f.fingerprint = String(sec.fingerprint || nested.fingerprint || 'chrome') || 'chrome'

    const ws = asRecord(stream.wsSettings)
    const xhttp = asRecord(stream.xhttpSettings)
    const headers = asRecord(ws.headers)
    f.path = String(ws.path || xhttp.path || f.path || '/')
    f.host = String(ws.host || headers.Host || xhttp.host || '')

    const grpc = asRecord(stream.grpcSettings)
    f.serviceName = String(grpc.serviceName || '')
  }

  return f
}
