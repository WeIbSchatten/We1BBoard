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
  tlsCertFile: string
  tlsKeyFile: string
  tlsCertContent: string
  tlsKeyContent: string
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
  // wireguard / amneziawg
  wgSecretKey: string
  wgAddress: string
  wgMtu: number
  awgJc: number
  awgJmin: number
  awgJmax: number
  awgS1: number
  awgS2: number
  awgH1: string
  awgH2: string
  awgH3: string
  awgH4: string
  // tuic / hysteria2
  tuicCongestion: string
  hy2Password: string
  // mtproto
  mtprotoFakeTlsDomain: string
  // sniffing
  sniffEnabled: boolean
  sniffDestOverride: string
  sniffRouteOnly: boolean
  sniffMetadataOnly: boolean
  sniffDomainsExcluded: string
  sniffIpsExcluded: string
  // advanced
  fallbacksJSON: string
  sockoptJSON: string
}

/** SS2022 password: base64 of 16 (aes-128) or 32 (aes-256 / other) random bytes. */
export function randomSS2022Password(method: string): string {
  const n = /128/.test(method) ? 16 : 32
  const bytes = new Uint8Array(n)
  crypto.getRandomValues(bytes)
  let binary = ''
  for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i])
  return btoa(binary)
}

/** AmneziaWG-style obfuscation defaults (subset: jc/jmin/jmax/s1/s2/h1-h4). */
export function randomAwgObfuscation(): Pick<
  InboundFormState,
  'awgJc' | 'awgJmin' | 'awgJmax' | 'awgS1' | 'awgS2' | 'awgH1' | 'awgH2' | 'awgH3' | 'awgH4'
> {
  const jmin = randomInteger(40, 89)
  const s1 = randomInteger(15, 150)
  let s2 = randomInteger(15, 150)
  while (s1 + 56 === s2) s2 = randomInteger(15, 150)
  const hMax = 2147483647
  const lo = 5
  const band = Math.floor((hMax - lo + 1) / 4)
  const h = (i: number) => String(randomInteger(lo + i * band, lo + i * band + band - 1))
  return {
    awgJc: randomInteger(3, 6),
    awgJmin: jmin,
    awgJmax: jmin + randomInteger(50, 250),
    awgS1: s1,
    awgS2: s2,
    awgH1: h(0),
    awgH2: h(1),
    awgH3: h(2),
    awgH4: h(3),
  }
}

export function emptyInboundForm(): InboundFormState {
  const ssMethod = '2022-blake3-aes-256-gcm'
  const awg = randomAwgObfuscation()
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
    tlsCertFile: '',
    tlsKeyFile: '',
    tlsCertContent: '',
    tlsKeyContent: '',
    realityDest: '',
    realitySNI: '',
    realityPrivateKey: '',
    realityPublicKey: '',
    realityShortIds: '',
    realityFingerprint: 'chrome',
    realitySpiderX: '/',
    ssMethod,
    ssPassword: randomSS2022Password(ssMethod),
    wgSecretKey: '',
    wgAddress: '10.0.0.1/24',
    wgMtu: 1420,
    ...awg,
    tuicCongestion: 'bbr',
    hy2Password: randomLowerAndNum(16),
    mtprotoFakeTlsDomain: 'www.cloudflare.com',
    sniffEnabled: false,
    sniffDestOverride: 'http,tls,quic,fakedns',
    sniffRouteOnly: false,
    sniffMetadataOnly: false,
    sniffDomainsExcluded: '',
    sniffIpsExcluded: '',
    fallbacksJSON: '[]',
    sockoptJSON: '',
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
    const certs = (tls.certificates || []) as Record<string, unknown>[]
    if (certs[0]) {
      f.tlsCertFile = String(certs[0].certificateFile || '')
      f.tlsKeyFile = String(certs[0].keyFile || '')
      const certContent = certs[0].certificate
      const keyContent = certs[0].key
      if (Array.isArray(certContent)) f.tlsCertContent = certContent.map(String).join('\n')
      else if (typeof certContent === 'string') f.tlsCertContent = certContent
      if (Array.isArray(keyContent)) f.tlsKeyContent = keyContent.map(String).join('\n')
      else if (typeof keyContent === 'string') f.tlsKeyContent = keyContent
    }
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
    if (stream.sockopt !== undefined) {
      f.sockoptJSON = JSON.stringify(stream.sockopt, null, 2)
    }
  } catch { /* keep defaults */ }
  try {
    const settings = JSON.parse(inb.settings || '{}') as Record<string, unknown>
    if (typeof settings.method === 'string') f.ssMethod = settings.method
    if (typeof settings.password === 'string') {
      f.ssPassword = settings.password
      f.hy2Password = settings.password
    }
    if (settings.fallbacks !== undefined) {
      f.fallbacksJSON = JSON.stringify(settings.fallbacks, null, 2)
    }
    if (typeof settings.secretKey === 'string') f.wgSecretKey = settings.secretKey
    if (typeof settings.mtu === 'number') f.wgMtu = settings.mtu
    const addr = settings.address
    if (Array.isArray(addr) && addr.length) f.wgAddress = String(addr[0])
    else if (typeof addr === 'string') f.wgAddress = addr
    if (typeof settings.jc === 'number') f.awgJc = settings.jc
    if (typeof settings.jmin === 'number') f.awgJmin = settings.jmin
    if (typeof settings.jmax === 'number') f.awgJmax = settings.jmax
    if (typeof settings.s1 === 'number') f.awgS1 = settings.s1
    if (typeof settings.s2 === 'number') f.awgS2 = settings.s2
    if (settings.h1 !== undefined) f.awgH1 = String(settings.h1)
    if (settings.h2 !== undefined) f.awgH2 = String(settings.h2)
    if (settings.h3 !== undefined) f.awgH3 = String(settings.h3)
    if (settings.h4 !== undefined) f.awgH4 = String(settings.h4)
    if (typeof settings.congestion_control === 'string') f.tuicCongestion = settings.congestion_control
    if (typeof settings.fakeTlsDomain === 'string') f.mtprotoFakeTlsDomain = settings.fakeTlsDomain
  } catch { /* */ }
  try {
    const sniff = JSON.parse(inb.sniffing || '{}') as Record<string, unknown>
    f.sniffEnabled = Boolean(sniff.enabled)
    const dest = sniff.destOverride as string[] | undefined
    if (dest?.length) f.sniffDestOverride = dest.join(',')
    f.sniffRouteOnly = Boolean(sniff.routeOnly)
    f.sniffMetadataOnly = Boolean(sniff.metadataOnly)
    const domainsEx = sniff.domainsExcluded as string[] | undefined
    if (domainsEx?.length) f.sniffDomainsExcluded = domainsEx.join(',')
    const ipsEx = sniff.ipsExcluded as string[] | undefined
    if (ipsEx?.length) f.sniffIpsExcluded = ipsEx.join(',')
  } catch { /* */ }
  return f
}

function pemToLines(pem: string): string[] {
  return pem.replace(/\r\n/g, '\n').split('\n').filter((l) => l.length > 0)
}

function csvList(s: string): string[] {
  return s.split(',').map((x) => x.trim()).filter(Boolean)
}

/** Protocols that do not use Xray streamSettings. */
export function protocolNeedsStream(protocol: string): boolean {
  return !['tun', 'tunnel', 'mtproto', 'tuic', 'hysteria2', 'wireguard', 'amneziawg'].includes(protocol)
}

export function buildStreamSettings(f: InboundFormState): string {
  if (!protocolNeedsStream(f.protocol)) {
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
    const useContent = Boolean(f.tlsCertContent.trim() || f.tlsKeyContent.trim())
    const cert: Record<string, unknown> = {
      usage: 'encipherment',
      ocspStapling: 0,
    }
    if (useContent) {
      cert.certificate = pemToLines(f.tlsCertContent)
      cert.key = pemToLines(f.tlsKeyContent)
    } else {
      cert.certificateFile = f.tlsCertFile || ''
      cert.keyFile = f.tlsKeyFile || ''
    }
    stream.tlsSettings = {
      serverName: f.tlsSNI || '',
      minVersion: '1.2',
      maxVersion: '1.3',
      cipherSuites: '',
      rejectUnknownSni: false,
      allowInsecure: false,
      alpn: f.tlsALPN ? csvList(f.tlsALPN) : ['h2', 'http/1.1'],
      certificates: [cert],
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
  const sockRaw = (f.sockoptJSON || '').trim()
  if (sockRaw) {
    const parsed = JSON.parse(sockRaw) as unknown
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      stream.sockopt = parsed
    } else {
      throw new Error('sockopt must be a JSON object')
    }
  }
  return JSON.stringify(stream)
}

function parseFallbacks(raw: string): unknown[] {
  const t = (raw || '').trim()
  if (!t) return []
  const parsed = JSON.parse(t) as unknown
  if (!Array.isArray(parsed)) throw new Error('fallbacks must be a JSON array')
  return parsed
}

export function buildInboundSettings(f: InboundFormState): string {
  let fallbacks: unknown[] = []
  if (f.protocol === 'vless' || f.protocol === 'trojan') {
    try {
      fallbacks = parseFallbacks(f.fallbacksJSON)
    } catch (e) {
      throw e instanceof Error ? e : new Error('invalid fallbacks JSON')
    }
  }
  switch (f.protocol) {
    case 'shadowsocks':
      return JSON.stringify({
        method: f.ssMethod || '2022-blake3-aes-256-gcm',
        password: f.ssPassword || '',
        network: 'tcp,udp',
        clients: [],
      })
    case 'vless':
      return JSON.stringify({ clients: [], decryption: 'none', encryption: 'none', fallbacks })
    case 'vmess':
      return JSON.stringify({ clients: [] })
    case 'trojan':
      return JSON.stringify({ clients: [], fallbacks })
    case 'http':
    case 'socks':
      return JSON.stringify({ auth: 'password', accounts: [], udp: f.protocol === 'socks', ip: '127.0.0.1' })
    case 'wireguard':
      return JSON.stringify({
        secretKey: f.wgSecretKey || '',
        address: [f.wgAddress || '10.0.0.1/24'],
        peers: [],
        mtu: f.wgMtu || 1420,
      })
    case 'amneziawg':
      return JSON.stringify({
        secretKey: f.wgSecretKey || '',
        address: [f.wgAddress || '10.0.0.1/24'],
        peers: [],
        mtu: f.wgMtu || 1420,
        jc: f.awgJc,
        jmin: f.awgJmin,
        jmax: f.awgJmax,
        s1: f.awgS1,
        s2: f.awgS2,
        h1: f.awgH1,
        h2: f.awgH2,
        h3: f.awgH3,
        h4: f.awgH4,
      })
    case 'tuic':
      return JSON.stringify({
        users: [],
        congestion_control: f.tuicCongestion || 'bbr',
      })
    case 'hysteria2':
      return JSON.stringify({
        users: [],
        password: f.hy2Password || '',
      })
    case 'mtproto':
      return JSON.stringify({
        fakeTlsDomain: f.mtprotoFakeTlsDomain || 'www.cloudflare.com',
      })
    default:
      return '{}'
  }
}

export function buildSniffing(f: InboundFormState): string {
  return JSON.stringify({
    enabled: f.sniffEnabled,
    destOverride: f.sniffDestOverride
      ? csvList(f.sniffDestOverride)
      : ['http', 'tls', 'quic', 'fakedns'],
    metadataOnly: f.sniffMetadataOnly,
    routeOnly: f.sniffRouteOnly,
    domainsExcluded: csvList(f.sniffDomainsExcluded),
    ipsExcluded: csvList(f.sniffIpsExcluded),
  })
}

/** Quick Cloudflare dest/SNI defaults for REALITY. */
export function applyCloudflareRealityDefaults(f: InboundFormState): InboundFormState {
  return {
    ...f,
    realityDest: 'www.cloudflare.com:443',
    realitySNI: 'www.cloudflare.com',
  }
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

const NO_CLIENT_PROTOCOLS = new Set([
  'wireguard', 'amneziawg', 'wg', 'tun', 'tunnel', 'mtproto', 'tuic', 'hysteria2',
])

/** Inbounds that can hold panel clients (xray DB-backed user protocols). */
export function inboundSupportsClients(protocol: string): boolean {
  return !NO_CLIENT_PROTOCOLS.has(protocol)
}
