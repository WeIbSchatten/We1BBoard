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
  if (['freedom', 'blackhole'].includes(f.protocol)) return '{}'
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
  'vless', 'vmess', 'trojan', 'shadowsocks', 'socks', 'http', 'freedom', 'blackhole',
]
