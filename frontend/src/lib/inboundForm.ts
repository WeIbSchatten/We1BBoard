/** Helpers to build/parse inbound stream+settings like 3x-ui forms. */

import { randomInteger, randomLowerAndNum, randomShortIds, randomSpiderX } from './random'

export type Network = 'tcp' | 'ws' | 'grpc' | 'httpupgrade' | 'xhttp' | 'kcp'
export type Security = 'none' | 'tls' | 'reality'

export type InboundFormState = {
  id?: number
  remark: string
  port: number
  listen: string
  protocol: string
  enable: boolean
  network: Network
  security: Security
  // transport
  wsPath: string
  wsHost: string
  grpcService: string
  httpupgradePath: string
  xhttpPath: string
  xhttpMode: string
  kcpSeed: string
  // tls
  tlsSNI: string
  tlsALPN: string
  tlsFingerprint: string
  // reality
  realityDest: string
  realitySNI: string
  realityPrivateKey: string
  realityPublicKey: string
  realityShortIds: string
  realityFingerprint: string
  realitySpiderX: string
  // shadowsocks
  ssMethod: string
  ssPassword: string
  // sniffing
  sniffEnabled: boolean
  sniffDestOverride: string
  sniffRouteOnly: boolean
}

export function emptyInboundForm(): InboundFormState {
  return {
    remark: '',
    port: randomInteger(10000, 60000),
    listen: '',
    protocol: 'vless',
    enable: true,
    network: 'tcp',
    security: 'none',
    wsPath: '/',
    wsHost: '',
    grpcService: '',
    httpupgradePath: '/',
    xhttpPath: '/',
    xhttpMode: 'auto',
    kcpSeed: randomLowerAndNum(8),
    tlsSNI: '',
    tlsALPN: 'h2,http/1.1',
    tlsFingerprint: 'chrome',
    realityDest: '',
    realitySNI: '',
    realityPrivateKey: '',
    realityPublicKey: '',
    realityShortIds: '',
    realityFingerprint: 'chrome',
    realitySpiderX: '/',
    ssMethod: '2022-blake3-aes-256-gcm',
    ssPassword: randomLowerAndNum(32),
    sniffEnabled: false,
    sniffDestOverride: 'http,tls,quic,fakedns',
    sniffRouteOnly: false,
  }
}

export function parseInboundToForm(inb: {
  id: number
  remark: string
  port: number
  listen: string
  protocol: string
  enable: boolean
  settings: string
  streamSettings: string
  sniffing?: string
}): InboundFormState {
  const f = emptyInboundForm()
  f.id = inb.id
  f.remark = inb.remark || ''
  f.port = inb.port
  f.listen = inb.listen === '0.0.0.0' ? '' : (inb.listen || '')
  f.protocol = inb.protocol
  f.enable = inb.enable
  try {
    const stream = JSON.parse(inb.streamSettings || '{}') as Record<string, unknown>
    f.network = (stream.network as Network) || 'tcp'
    f.security = (stream.security as Security) || 'none'
    const ws = (stream.wsSettings || {}) as Record<string, unknown>
    f.wsPath = String(ws.path || '/')
    const headers = (ws.headers || {}) as Record<string, string>
    f.wsHost = headers.Host || String(ws.host || '')
    const grpc = (stream.grpcSettings || {}) as Record<string, unknown>
    f.grpcService = String(grpc.serviceName || '')
    const hu = (stream.httpupgradeSettings || {}) as Record<string, unknown>
    f.httpupgradePath = String(hu.path || '/')
    const xh = (stream.xhttpSettings || {}) as Record<string, unknown>
    f.xhttpPath = String(xh.path || '/')
    f.xhttpMode = String(xh.mode || 'auto')
    const kcp = (stream.kcpSettings || {}) as Record<string, unknown>
    f.kcpSeed = String(kcp.seed || '')
    const tls = (stream.tlsSettings || {}) as Record<string, unknown>
    f.tlsSNI = String(tls.serverName || '')
    const alpn = tls.alpn as string[] | undefined
    if (alpn?.length) f.tlsALPN = alpn.join(',')
    const tlsSettings = (tls.settings || {}) as Record<string, unknown>
    f.tlsFingerprint = String(tlsSettings.fingerprint || tls.fingerprint || 'chrome')
    const rs = (stream.realitySettings || {}) as Record<string, unknown>
    f.realityDest = String(rs.dest || rs.target || '')
    const sns = rs.serverNames as string[] | undefined
    f.realitySNI = sns?.join(',') || ''
    f.realityPrivateKey = String(rs.privateKey || '')
    const sids = rs.shortIds as string[] | undefined
    f.realityShortIds = sids?.join(',') || ''
    f.realityFingerprint = String(rs.fingerprint || 'chrome')
    f.realitySpiderX = String(rs.spiderX || '/')
    f.realityPublicKey = String(rs.publicKey || (rs.settings as Record<string, string> | undefined)?.publicKey || '')
  } catch { /* keep defaults */ }
  try {
    const settings = JSON.parse(inb.settings || '{}') as Record<string, unknown>
    if (typeof settings.method === 'string') f.ssMethod = settings.method
    if (typeof settings.password === 'string') f.ssPassword = settings.password
  } catch { /* */ }
  try {
    const sniff = JSON.parse(inb.sniffing || '{}') as Record<string, unknown>
    f.sniffEnabled = Boolean(sniff.enabled)
    const dest = sniff.destOverride as string[] | undefined
    if (dest?.length) f.sniffDestOverride = dest.join(',')
    f.sniffRouteOnly = Boolean(sniff.routeOnly)
  } catch { /* */ }
  return f
}

export function buildStreamSettings(f: InboundFormState): string {
  if (['tun', 'tunnel', 'mtproto', 'tuic', 'hysteria2'].includes(f.protocol)) {
    return '{}'
  }
  const stream: Record<string, unknown> = {
    network: f.network,
    security: f.security,
  }
  switch (f.network) {
    case 'ws':
      stream.wsSettings = {
        path: f.wsPath || '/',
        headers: f.wsHost ? { Host: f.wsHost } : {},
      }
      break
    case 'grpc':
      stream.grpcSettings = { serviceName: f.grpcService || '' }
      break
    case 'httpupgrade':
      stream.httpupgradeSettings = { path: f.httpupgradePath || '/' }
      break
    case 'xhttp':
      stream.xhttpSettings = { path: f.xhttpPath || '/', mode: f.xhttpMode || 'auto' }
      break
    case 'kcp':
      stream.kcpSettings = {
        mtu: 1350,
        tti: 20,
        uplinkCapacity: 5,
        downlinkCapacity: 20,
        congestion: false,
        readBufferSize: 2,
        writeBufferSize: 2,
        header: { type: 'none' },
        seed: f.kcpSeed || '',
      }
      break
    default:
      stream.tcpSettings = { acceptProxyProtocol: false, header: { type: 'none' } }
  }
  if (f.security === 'tls') {
    stream.tlsSettings = {
      serverName: f.tlsSNI || '',
      minVersion: '1.2',
      maxVersion: '1.3',
      cipherSuites: '',
      rejectUnknownSni: false,
      allowInsecure: false,
      alpn: f.tlsALPN ? f.tlsALPN.split(',').map((s) => s.trim()).filter(Boolean) : ['h2', 'http/1.1'],
      certificates: [{ certificateFile: '', keyFile: '', usage: 'encipherment', ocspStapling: 0 }],
      settings: { fingerprint: f.tlsFingerprint || 'chrome', allowInsecure: false },
    }
  }
  if (f.security === 'reality') {
    const shortIds = f.realityShortIds
      ? f.realityShortIds.split(',').map((s) => s.trim()).filter(Boolean)
      : ['']
    const serverNames = f.realitySNI
      ? f.realitySNI.split(',').map((s) => s.trim()).filter(Boolean)
      : []
    stream.realitySettings = {
      show: false,
      dest: f.realityDest || '',
      target: f.realityDest || '',
      xver: 0,
      serverNames,
      privateKey: f.realityPrivateKey,
      shortIds,
      fingerprint: f.realityFingerprint || 'chrome',
      spiderX: f.realitySpiderX || '/',
      publicKey: f.realityPublicKey || '',
      settings: {
        publicKey: f.realityPublicKey || '',
        fingerprint: f.realityFingerprint || 'chrome',
        serverName: serverNames[0] || '',
        spiderX: f.realitySpiderX || '/',
      },
    }
  }
  return JSON.stringify(stream)
}

export function buildInboundSettings(f: InboundFormState): string {
  switch (f.protocol) {
    case 'shadowsocks':
      return JSON.stringify({
        method: f.ssMethod || '2022-blake3-aes-256-gcm',
        password: f.ssPassword || '',
        network: 'tcp,udp',
        clients: [],
      })
    case 'vless':
      return JSON.stringify({ clients: [], decryption: 'none', encryption: 'none', fallbacks: [] })
    case 'vmess':
      return JSON.stringify({ clients: [] })
    case 'trojan':
      return JSON.stringify({ clients: [], fallbacks: [] })
    case 'http':
    case 'socks':
      return JSON.stringify({ auth: 'password', accounts: [], udp: false, ip: '127.0.0.1' })
    default:
      return '{}'
  }
}

export function buildSniffing(f: InboundFormState): string {
  return JSON.stringify({
    enabled: f.sniffEnabled,
    destOverride: f.sniffDestOverride
      ? f.sniffDestOverride.split(',').map((s) => s.trim()).filter(Boolean)
      : ['http', 'tls', 'quic', 'fakedns'],
    metadataOnly: false,
    routeOnly: f.sniffRouteOnly,
  })
}

/** Apply 3x-ui Reality bootstrap when security switches to reality. */
export function applyRealityDefaults(f: InboundFormState): InboundFormState {
  return {
    ...f,
    security: 'reality',
    realityDest: '',
    realitySNI: '',
    realityShortIds: randomShortIds().join(','),
    realitySpiderX: randomSpiderX(),
    realityFingerprint: 'chrome',
  }
}

export function applyTlsDefaults(f: InboundFormState): InboundFormState {
  return {
    ...f,
    security: 'tls',
    tlsFingerprint: 'chrome',
    tlsALPN: 'h2,http/1.1',
  }
}

export function suggestedFlow(f: InboundFormState): string {
  if (f.protocol === 'vless' && f.security === 'reality' && f.network === 'tcp') {
    return 'xtls-rprx-vision'
  }
  return ''
}

export const NETWORKS: Network[] = ['tcp', 'ws', 'grpc', 'httpupgrade', 'xhttp', 'kcp']
export const SECURITIES: Security[] = ['none', 'tls', 'reality']
export const PROTOCOLS = [
  'vless', 'vmess', 'trojan', 'shadowsocks', 'wireguard', 'amneziawg',
  'tuic', 'hysteria2', 'mtproto', 'http', 'socks', 'tunnel', 'tun',
]
export const SS_METHODS = [
  'aes-128-gcm', 'aes-256-gcm', 'chacha20-poly1305',
  '2022-blake3-aes-128-gcm', '2022-blake3-aes-256-gcm',
]
export const FINGERPRINTS = ['chrome', 'firefox', 'safari', 'ios', 'android', 'edge', 'random', 'qq', 'randomized']
