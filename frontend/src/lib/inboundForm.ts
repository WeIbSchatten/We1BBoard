/** Helpers to build/parse inbound stream+settings like 3x-ui forms. */

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
  // reality
  realityDest: string
  realitySNI: string
  realityPrivateKey: string
  realityPublicKey: string
  realityShortId: string
  realityFingerprint: string
  realitySpiderX: string
  // shadowsocks
  ssMethod: string
  // client (create)
  clientEmail: string
  clientUUID: string
  clientPassword: string
  clientFlow: string
  clientTotalGB: number
  clientExpiryDays: number
}

export function emptyInboundForm(): InboundFormState {
  const port = 10000 + Math.floor(Math.random() * 50000)
  return {
    remark: '',
    port,
    listen: '0.0.0.0',
    protocol: 'vless',
    enable: true,
    network: 'tcp',
    security: 'none',
    wsPath: '/',
    wsHost: '',
    grpcService: 'grpc',
    httpupgradePath: '/',
    xhttpPath: '/',
    xhttpMode: 'auto',
    kcpSeed: '',
    tlsSNI: '',
    realityDest: 'www.cloudflare.com:443',
    realitySNI: 'www.cloudflare.com',
    realityPrivateKey: '',
    realityPublicKey: '',
    realityShortId: '',
    realityFingerprint: 'chrome',
    realitySpiderX: '/',
    ssMethod: 'aes-128-gcm',
    clientEmail: '',
    clientUUID: '',
    clientPassword: '',
    clientFlow: '',
    clientTotalGB: 0,
    clientExpiryDays: 0,
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
}): InboundFormState {
  const f = emptyInboundForm()
  f.id = inb.id
  f.remark = inb.remark || ''
  f.port = inb.port
  f.listen = inb.listen || '0.0.0.0'
  f.protocol = inb.protocol
  f.enable = inb.enable
  try {
    const stream = JSON.parse(inb.streamSettings || '{}') as Record<string, unknown>
    f.network = (stream.network as Network) || 'tcp'
    f.security = (stream.security as Security) || 'none'
    const ws = (stream.wsSettings || {}) as Record<string, unknown>
    f.wsPath = String(ws.path || '/')
    const headers = (ws.headers || {}) as Record<string, string>
    f.wsHost = headers.Host || ''
    const grpc = (stream.grpcSettings || {}) as Record<string, unknown>
    f.grpcService = String(grpc.serviceName || 'grpc')
    const hu = (stream.httpupgradeSettings || {}) as Record<string, unknown>
    f.httpupgradePath = String(hu.path || '/')
    const xh = (stream.xhttpSettings || {}) as Record<string, unknown>
    f.xhttpPath = String(xh.path || '/')
    f.xhttpMode = String(xh.mode || 'auto')
    const kcp = (stream.kcpSettings || {}) as Record<string, unknown>
    f.kcpSeed = String(kcp.seed || '')
    const tls = (stream.tlsSettings || {}) as Record<string, unknown>
    f.tlsSNI = String(tls.serverName || '')
    const rs = (stream.realitySettings || {}) as Record<string, unknown>
    f.realityDest = String(rs.dest || 'www.cloudflare.com:443')
    const sns = rs.serverNames as string[] | undefined
    f.realitySNI = sns?.[0] || 'www.cloudflare.com'
    f.realityPrivateKey = String(rs.privateKey || '')
    const sids = rs.shortIds as string[] | undefined
    f.realityShortId = sids?.[0] || ''
    f.realityFingerprint = String(rs.fingerprint || 'chrome')
    f.realitySpiderX = String(rs.spiderX || '/')
    // public key may be stored for convenience
    f.realityPublicKey = String(rs.publicKey || '')
  } catch { /* keep defaults */ }
  try {
    const settings = JSON.parse(inb.settings || '{}') as Record<string, unknown>
    if (typeof settings.method === 'string') f.ssMethod = settings.method
  } catch { /* */ }
  return f
}

export function buildStreamSettings(f: InboundFormState): string {
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
      stream.grpcSettings = { serviceName: f.grpcService || 'grpc' }
      break
    case 'httpupgrade':
      stream.httpupgradeSettings = { path: f.httpupgradePath || '/' }
      break
    case 'xhttp':
      stream.xhttpSettings = { path: f.xhttpPath || '/', mode: f.xhttpMode || 'auto' }
      break
    case 'kcp':
      stream.kcpSettings = { mtu: 1350, seed: f.kcpSeed || '' }
      break
    default:
      stream.tcpSettings = { header: { type: 'none' } }
  }
  if (f.security === 'tls') {
    stream.tlsSettings = {
      serverName: f.tlsSNI || '',
      allowInsecure: false,
    }
  }
  if (f.security === 'reality') {
    stream.realitySettings = {
      show: false,
      dest: f.realityDest || 'www.cloudflare.com:443',
      xver: 0,
      serverNames: [f.realitySNI || 'www.cloudflare.com'],
      privateKey: f.realityPrivateKey,
      shortIds: [f.realityShortId || ''],
      fingerprint: f.realityFingerprint || 'chrome',
      spiderX: f.realitySpiderX || '/',
      publicKey: f.realityPublicKey || '',
    }
  }
  return JSON.stringify(stream)
}

export function buildInboundSettings(f: InboundFormState): string {
  switch (f.protocol) {
    case 'shadowsocks':
      return JSON.stringify({ method: f.ssMethod || 'aes-128-gcm', network: 'tcp,udp' })
    case 'vless':
      return JSON.stringify({ decryption: 'none', clients: [] })
    case 'vmess':
      return JSON.stringify({ clients: [] })
    case 'trojan':
      return JSON.stringify({ clients: [] })
    default:
      return '{}'
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
export const SS_METHODS = ['aes-128-gcm', 'aes-256-gcm', 'chacha20-poly1305', '2022-blake3-aes-128-gcm']
export const FINGERPRINTS = ['chrome', 'firefox', 'safari', 'ios', 'android', 'edge', 'random']
