const API_BASE = (() => {
  const path = window.location.pathname
  const idx = path.indexOf('/we1b')
  if (idx >= 0) return path.slice(0, idx) + '/we1b/api'
  return '/we1b/api'
})()

export { API_BASE }

export async function api<T = unknown>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(API_BASE + path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(init?.headers || {}) },
    ...init,
  })
  const data = await res.json()
  if (!data.success) throw new Error(data.error || res.statusText)
  return data.data as T
}

export type Inbound = {
  id: number
  remark: string
  enable: boolean
  listen: string
  port: number
  protocol: string
  settings: string
  streamSettings: string
  sniffing: string
  tag: string
  up: number
  down: number
  clients?: Client[]
}

export type Client = {
  id: number
  inboundId: number
  inboundIds?: string
  email: string
  enable: boolean
  uuid: string
  password: string
  flow: string
  subId: string
  group?: string
  limitIp?: number
  limitHwid?: number
  totalGB: number
  expiryTime: number
  trafficReset?: string
  extraLinks?: string
  up: number
  down: number
  comment: string
  tgId?: number
}

export type OnlineClients = {
  emails: string[]
  map: Record<string, number>
}

export type ClientIPRow = {
  ip: string
  lastSeen: number
}

export type ClientHWIDRow = {
  id: number
  email: string
  hwid: string
  createdAt: string
}

export type GroupSummary = {
  name: string
  count: number
  up: number
  down: number
}

export type Host = {
  id: number
  enable: boolean
  remark: string
  inboundId: number
  inboundTag: string
  address: string
  port: number
  sni: string
  hostHeader: string
  path: string
  alpn: string
  fingerprint: string
  allowInsecure: boolean
  sortOrder: number
}

export type Bridge = {
  id: number
  name: string
  enable: boolean
  dialerProtocol: string
  dialerAddress: string
  dialerPort: number
  dialerUUID: string
  dialerPassword?: string
  dialerEmail?: string
  dialerMethod?: string
  dialerFlow: string
  dialerSecurity: string
  dialerNetwork?: string
  dialerSNI: string
  dialerPublicKey: string
  dialerShortId: string
  dialerFingerprint: string
  dialerPath?: string
  dialerHost?: string
  dialerServiceName?: string
  dialerSettings?: string
  dialerStreamSettings?: string
  outboundTag: string
  routingInbound: string
  remark: string
}

export type Node = {
  id: number
  name: string
  url: string
  token: string
  tlsMode: string
  region: string
  enable: boolean
  online: boolean
  lastSeen: number
}

export type TgProxy = {
  id: number
  name: string
  enable: boolean
  hostname: string
  listen: string
  mtproxyAddr: string
  secret: string
  carrierMode: string
  publicSiteDir: string
  publicMode: string
  remark: string
}

export type Outbound = {
  id: number
  tag: string
  protocol: string
  settings: string
  streamSettings: string
  enable: boolean
  remark: string
}

export type OutboundSubscription = {
  id: number
  remark: string
  url: string
  enable: boolean
  prefix: string
  intervalMin: number
  lastFetch: number
  lastError: string
}

export type ServerHistory = {
  cpu: number[]
  mem: number[]
  tcp: number[]
  udp: number[]
}

export type GeodataFileStatus = {
  exists: boolean
  path?: string
  size?: number
  mtime?: number
}

export type GeodataStatus = {
  geosite: GeodataFileStatus
  geoip: GeodataFileStatus
  geositeURL: string
  geoipURL: string
  dir: string
}
