const API_BASE = (() => {
  const path = window.location.pathname
  const idx = path.indexOf('/we1b')
  if (idx >= 0) return path.slice(0, idx) + '/we1b/api'
  return '/we1b/api'
})()

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
  email: string
  enable: boolean
  uuid: string
  password: string
  flow: string
  subId: string
  totalGB: number
  expiryTime: number
  up: number
  down: number
  comment: string
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
