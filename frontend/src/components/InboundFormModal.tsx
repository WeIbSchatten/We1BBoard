import { FormEvent, useEffect, useState } from 'react'
import { api, type Inbound } from '../api'
import { useApp } from '../AppContext'
import {
  InboundFormState,
  NETWORKS,
  PROTOCOLS,
  SECURITIES,
  SS_METHODS,
  FINGERPRINTS,
  applyCloudflareRealityDefaults,
  applyRealityDefaults,
  applyTlsDefaults,
  buildInboundSettings,
  buildSniffing,
  buildStreamSettings,
  emptyInboundForm,
  parseInboundToForm,
  protocolNeedsStream,
  randomAwgObfuscation,
  randomSS2022Password,
  suggestedFlow,
} from '../lib/inboundForm'
import { randomInteger, randomLowerAndNum, randomShortIds, randomSpiderX } from '../lib/random'
import { FormRow } from './FormRow'

function ssPasswordForMethod(method: string): string {
  return method.startsWith('2022-') ? randomSS2022Password(method) : randomLowerAndNum(32)
}

/** Protocols with non-empty settings UI (not vless/vmess/trojan clients-only). */
function protocolHasSettingsTab(protocol: string): boolean {
  return [
    'shadowsocks', 'wireguard', 'amneziawg', 'tuic', 'hysteria2',
    'mtproto', 'http', 'socks',
  ].includes(protocol)
}

function applyNetworkPathDefaults(next: InboundFormState, network: string): InboundFormState {
  switch (network) {
    case 'ws':
      return { ...next, wsPath: '/', wsHost: '' }
    case 'grpc':
      return { ...next, grpcService: '' }
    case 'httpupgrade':
      return { ...next, httpupgradePath: '/' }
    case 'xhttp':
      return { ...next, xhttpPath: '/', xhttpMode: 'auto' }
    case 'kcp':
      return { ...next, kcpSeed: randomLowerAndNum(8) }
    default:
      return next
  }
}

type Props = {
  open: boolean
  mode: 'add' | 'edit'
  inbound: Inbound | null
  onClose: () => void
  onSaved: (created?: Inbound) => void
}

type Tab = 'basic' | 'protocol' | 'stream' | 'security' | 'sniffing' | 'advanced'

export function InboundFormModal({ open, mode, inbound, onClose, onSaved }: Props) {
  const { tr } = useApp()
  const [tab, setTab] = useState<Tab>('basic')
  const [form, setForm] = useState<InboundFormState>(emptyInboundForm())
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setTab('basic')
    setError('')
    if (mode === 'edit' && inbound) {
      setForm(parseInboundToForm(inbound))
    } else {
      setForm(emptyInboundForm())
    }
  }, [open, mode, inbound])

  useEffect(() => {
    if (!open) return
    const needs = protocolNeedsStream(form.protocol)
    const hasProto = protocolHasSettingsTab(form.protocol)
    const showAdv = form.protocol === 'vless' || form.protocol === 'trojan' || needs
    if (tab === 'protocol' && !hasProto) setTab('basic')
    else if ((tab === 'stream' || tab === 'security' || tab === 'sniffing') && !needs) setTab('basic')
    else if (tab === 'advanced' && !showAdv) setTab('basic')
  }, [open, form.protocol, tab])

  function set<K extends keyof InboundFormState>(key: K, value: InboundFormState[K]) {
    setForm((prev) => {
      let next = { ...prev, [key]: value }
      if (key === 'protocol' && mode === 'add') {
        if (value === 'shadowsocks') {
          next.ssMethod = '2022-blake3-aes-256-gcm'
          next.ssPassword = randomSS2022Password(next.ssMethod)
        }
        if (['tun', 'tunnel', 'mtproto', 'tuic', 'hysteria2', 'wireguard', 'amneziawg'].includes(String(value))) {
          next.network = 'tcp'
          next.security = 'none'
        }
        // vless/vmess/trojan: keep security as-is (none stays none)
        if (value === 'amneziawg' || value === 'wireguard') {
          Object.assign(next, randomAwgObfuscation())
          next.wgAddress = '10.0.0.1/24'
          next.wgMtu = 1420
          queueMicrotask(() => { void genWgKeys() })
        }
        if (value === 'mtproto') {
          next.mtprotoFakeTlsDomain = 'www.cloudflare.com'
        }
        if (value === 'tuic') {
          next.tuicCongestion = 'bbr'
        }
        if (value === 'hysteria2') {
          next.hy2Password = randomLowerAndNum(16)
        }
      }
      if (key === 'ssMethod') {
        next.ssPassword = ssPasswordForMethod(String(value))
      }
      if (key === 'network' && String(value) !== prev.network) {
        next = applyNetworkPathDefaults(next, String(value))
      }
      if (key === 'security') {
        if (value === 'reality') {
          next = applyRealityDefaults(next)
          queueMicrotask(() => { void refreshKeys() })
        } else if (value === 'tls') {
          next = applyTlsDefaults(next)
        }
      }
      return next
    })
  }

  async function refreshKeys() {
    try {
      const keys = await api<{ privateKey: string; publicKey: string; shortId: string }>('/tools/reality-keys')
      setForm((prev) => ({
        ...prev,
        realityPrivateKey: keys.privateKey,
        realityPublicKey: keys.publicKey,
        realityShortIds: prev.realityShortIds || [keys.shortId, ...randomShortIds().slice(0, 7)].join(','),
        realitySpiderX: prev.realitySpiderX && prev.realitySpiderX !== '/' ? prev.realitySpiderX : randomSpiderX(),
      }))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'keys error')
    }
  }

  async function genWgKeys() {
    try {
      const keys = await api<{ secretKey: string; privateKey: string; publicKey: string }>('/tools/wireguard-keys')
      setForm((prev) => ({
        ...prev,
        wgSecretKey: keys.secretKey || keys.privateKey,
      }))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'wg keys error')
    }
  }

  async function scanRealityTarget() {
    const target = form.realityDest.trim() || 'www.example.com:443'
    setError('')
    setBusy(true)
    try {
      const res = await api<{ dest: string; serverNames: string[]; ok: boolean; error?: string }>('/tools/reality-scan', {
        method: 'POST',
        body: JSON.stringify({ target }),
      })
      if (!res.ok) {
        throw new Error(res.error || 'scan failed')
      }
      const names = (res.serverNames || []).slice(0, 5)
      setForm((prev) => ({
        ...prev,
        realityDest: res.dest || target,
        realitySNI: names.length ? names.join(',') : prev.realitySNI,
      }))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'scan error')
    } finally {
      setBusy(false)
    }
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      if (form.security === 'tls' && protocolNeedsStream(form.protocol)) {
        const hasFiles = form.tlsCertFile.trim() && form.tlsKeyFile.trim()
        const hasContent = form.tlsCertContent.trim() && form.tlsKeyContent.trim()
        if (!hasFiles && !hasContent) {
          throw new Error('TLS: provide cert/key file paths or paste PEM content')
        }
      }
      if (form.security === 'reality' && !form.realityPrivateKey) {
        throw new Error('Generate REALITY keys first')
      }
      if (form.security === 'reality' && !form.realityDest) {
        throw new Error('REALITY dest/target is required (e.g. www.example.com:443)')
      }
      if (form.security === 'reality' && !form.realitySNI) {
        throw new Error('REALITY serverNames (SNI) is required')
      }
      if ((form.protocol === 'wireguard' || form.protocol === 'amneziawg') && !form.wgSecretKey.trim()) {
        throw new Error('Generate WireGuard secretKey first')
      }
      const remark = form.remark.trim() || `${form.protocol}-${form.port}`
      const payload = {
        remark,
        port: form.port,
        listen: form.listen || '0.0.0.0',
        protocol: form.protocol,
        enable: form.enable,
        settings: buildInboundSettings(form),
        streamSettings: buildStreamSettings(form),
        sniffing: buildSniffing(form),
      }
      let saved: Inbound
      if (mode === 'edit' && form.id) {
        saved = await api<Inbound>(`/inbounds/${form.id}`, { method: 'PUT', body: JSON.stringify({ ...payload, id: form.id }) })
      } else {
        saved = await api<Inbound>('/inbounds', { method: 'POST', body: JSON.stringify(payload) })
      }
      onSaved(saved)
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  if (!open) return null

  const needsStream = protocolNeedsStream(form.protocol)
  const showProtocol = protocolHasSettingsTab(form.protocol)
  const showFallbacks = form.protocol === 'vless' || form.protocol === 'trojan'
  const showAdvanced = showFallbacks || needsStream
  const isWg = form.protocol === 'wireguard' || form.protocol === 'amneziawg'
  const visionTip = suggestedFlow(form)
  const tabs: { id: Tab; label: string; show?: boolean }[] = [
    { id: 'basic', label: tr('tabGeneral') },
    { id: 'protocol', label: tr('tabProtocol'), show: showProtocol },
    { id: 'stream', label: tr('tabStream'), show: needsStream },
    { id: 'security', label: tr('tabSecurity'), show: needsStream },
    { id: 'sniffing', label: tr('tabSniffing'), show: needsStream },
    { id: 'advanced', label: tr('tabAdvanced'), show: showAdvanced },
  ]

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <form className="modal modal--lg" onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <h3>{mode === 'edit' ? tr('editInbound') : tr('addInbound')}</h3>

        <div className="tabs">
          {tabs.filter((t) => t.show !== false).map((t) => (
            <button key={t.id} type="button" className={`tab ${tab === t.id ? 'active' : ''}`} onClick={() => setTab(t.id)}>
              {t.label}
            </button>
          ))}
        </div>

        <div className="modal-body-scroll">
          {tab === 'basic' && (
            <div>
              <FormRow label={tr('remark')}>
                <input className="input" value={form.remark} onChange={(e) => set('remark', e.target.value)} placeholder="" />
              </FormRow>
              <FormRow label={tr('port')}>
                <div className="input-compact">
                  <input
                    className="input input-number--compact"
                    type="number"
                    min={1}
                    max={65535}
                    value={form.port}
                    onChange={(e) => set('port', Number(e.target.value))}
                    required
                  />
                  <button
                    type="button"
                    className="btn secondary btn-sm"
                    title={tr('randomPort')}
                    onClick={() => set('port', randomInteger(10000, 60000))}
                  >
                    ↻
                  </button>
                </div>
              </FormRow>
              <FormRow label={tr('protocol')}>
                <select className="select" value={form.protocol} onChange={(e) => set('protocol', e.target.value)} disabled={mode === 'edit'}>
                  {PROTOCOLS.map((p) => <option key={p} value={p}>{p}</option>)}
                </select>
              </FormRow>
              <FormRow label={tr('listen')} hint={tr('listenHint')}>
                <input className="input" value={form.listen} onChange={(e) => set('listen', e.target.value)} placeholder="" />
              </FormRow>
              <FormRow label={tr('enable')}>
                <label className="form-switch">
                  <input type="checkbox" checked={form.enable} onChange={(e) => set('enable', e.target.checked)} />
                  <span>{form.enable ? 'On' : 'Off'}</span>
                </label>
              </FormRow>
            </div>
          )}

          {tab === 'protocol' && showProtocol && (
            <div>
              {form.protocol === 'shadowsocks' && (
                <div className="form-section">
                  <div className="form-section__title">Shadowsocks</div>
                  <div className="form-section__body">
                    <FormRow label="Method">
                      <select className="select" value={form.ssMethod} onChange={(e) => set('ssMethod', e.target.value)}>
                        {SS_METHODS.map((m) => <option key={m} value={m}>{m}</option>)}
                      </select>
                    </FormRow>
                    <FormRow label={tr('password')}>
                      <div className="input-compact">
                        <input className="input" value={form.ssPassword} onChange={(e) => set('ssPassword', e.target.value)} />
                        <button type="button" className="btn secondary btn-sm" onClick={() => set('ssPassword', ssPasswordForMethod(form.ssMethod))}>↻</button>
                      </div>
                    </FormRow>
                  </div>
                </div>
              )}

              {isWg && (
                <div className="form-section">
                  <div className="form-section__title">{form.protocol === 'amneziawg' ? 'AmneziaWG' : 'WireGuard'}</div>
                  <div className="form-section__subtitle">Peers are empty — panel clients can be mapped as peers later.</div>
                  <div className="form-section__body">
                    <FormRow label="Secret key">
                      <div className="input-compact">
                        <input className="input" value={form.wgSecretKey} onChange={(e) => set('wgSecretKey', e.target.value)} />
                        <button type="button" className="btn secondary btn-sm" onClick={() => { void genWgKeys() }}>↻</button>
                      </div>
                    </FormRow>
                    <FormRow label="Address">
                      <input className="input" value={form.wgAddress} onChange={(e) => set('wgAddress', e.target.value)} placeholder="10.0.0.1/24" />
                    </FormRow>
                    <FormRow label="MTU">
                      <input
                        className="input input-number--compact"
                        type="number"
                        min={576}
                        max={65535}
                        value={form.wgMtu}
                        onChange={(e) => set('wgMtu', Number(e.target.value))}
                      />
                    </FormRow>
                  </div>
                </div>
              )}

              {form.protocol === 'amneziawg' && (
                <div className="form-section">
                  <div className="form-section__title">Obfuscation</div>
                  <div className="form-section__body">
                    <div className="form-actions" style={{ marginTop: 0, paddingTop: 0, borderTop: 0, marginBottom: '0.75rem' }}>
                      <button type="button" className="btn secondary btn-sm" onClick={() => setForm((prev) => ({ ...prev, ...randomAwgObfuscation() }))}>
                        Regenerate obfuscation
                      </button>
                    </div>
                    <FormRow label="Jc">
                      <input className="input input-number--compact" type="number" value={form.awgJc} onChange={(e) => set('awgJc', Number(e.target.value))} />
                    </FormRow>
                    <FormRow label="Jmin">
                      <input className="input input-number--compact" type="number" value={form.awgJmin} onChange={(e) => set('awgJmin', Number(e.target.value))} />
                    </FormRow>
                    <FormRow label="Jmax">
                      <input className="input input-number--compact" type="number" value={form.awgJmax} onChange={(e) => set('awgJmax', Number(e.target.value))} />
                    </FormRow>
                    <FormRow label="S1">
                      <input className="input input-number--compact" type="number" value={form.awgS1} onChange={(e) => set('awgS1', Number(e.target.value))} />
                    </FormRow>
                    <FormRow label="S2">
                      <input className="input input-number--compact" type="number" value={form.awgS2} onChange={(e) => set('awgS2', Number(e.target.value))} />
                    </FormRow>
                    <FormRow label="H1">
                      <input className="input" value={form.awgH1} onChange={(e) => set('awgH1', e.target.value)} />
                    </FormRow>
                    <FormRow label="H2">
                      <input className="input" value={form.awgH2} onChange={(e) => set('awgH2', e.target.value)} />
                    </FormRow>
                    <FormRow label="H3">
                      <input className="input" value={form.awgH3} onChange={(e) => set('awgH3', e.target.value)} />
                    </FormRow>
                    <FormRow label="H4">
                      <input className="input" value={form.awgH4} onChange={(e) => set('awgH4', e.target.value)} />
                    </FormRow>
                  </div>
                </div>
              )}

              {form.protocol === 'tuic' && (
                <div className="form-section">
                  <div className="form-section__title">TUIC</div>
                  <div className="form-section__subtitle">Users list starts empty — panel clients may not map to TUIC users yet.</div>
                  <div className="form-section__body">
                    <FormRow label="Congestion">
                      <select className="select" value={form.tuicCongestion} onChange={(e) => set('tuicCongestion', e.target.value)}>
                        <option value="bbr">bbr</option>
                        <option value="cubic">cubic</option>
                        <option value="new_reno">new_reno</option>
                      </select>
                    </FormRow>
                  </div>
                </div>
              )}

              {form.protocol === 'hysteria2' && (
                <div className="form-section">
                  <div className="form-section__title">Hysteria2</div>
                  <div className="form-section__subtitle">Users list starts empty — panel clients may not map to Hysteria2 users yet.</div>
                  <div className="form-section__body">
                    <FormRow label={tr('password')}>
                      <div className="input-compact">
                        <input className="input" value={form.hy2Password} onChange={(e) => set('hy2Password', e.target.value)} />
                        <button type="button" className="btn secondary btn-sm" onClick={() => set('hy2Password', randomLowerAndNum(16))}>↻</button>
                      </div>
                    </FormRow>
                  </div>
                </div>
              )}

              {form.protocol === 'mtproto' && (
                <div className="form-section">
                  <div className="form-section__title">MTProto</div>
                  <div className="form-section__body">
                    <FormRow label="Fake TLS domain">
                      <input className="input" value={form.mtprotoFakeTlsDomain} onChange={(e) => set('mtprotoFakeTlsDomain', e.target.value)} placeholder="www.cloudflare.com" />
                    </FormRow>
                  </div>
                </div>
              )}

              {(form.protocol === 'http' || form.protocol === 'socks') && (
                <div className="form-section">
                  <div className="form-section__title">{form.protocol.toUpperCase()}</div>
                  <div className="form-section__subtitle">
                    Clients on this inbound become {form.protocol.toUpperCase()} accounts (user/pass).
                  </div>
                </div>
              )}
            </div>
          )}

          {tab === 'stream' && needsStream && (
            <div>
              <FormRow label={tr('network')}>
                <div className="seg seg--wrap">
                  {NETWORKS.map((n) => (
                    <button
                      key={n}
                      type="button"
                      className={form.network === n ? 'active' : ''}
                      onClick={() => set('network', n)}
                    >
                      {n === 'tcp' ? 'tcp' : n}
                    </button>
                  ))}
                </div>
              </FormRow>

              {form.network === 'ws' && (
                <div className="form-section">
                  <div className="form-section__title">WebSocket</div>
                  <div className="form-section__body">
                    <FormRow label="Path">
                      <input className="input" value={form.wsPath} onChange={(e) => set('wsPath', e.target.value)} />
                    </FormRow>
                    <FormRow label="Host">
                      <input className="input" value={form.wsHost} onChange={(e) => set('wsHost', e.target.value)} />
                    </FormRow>
                  </div>
                </div>
              )}

              {form.network === 'grpc' && (
                <div className="form-section">
                  <div className="form-section__title">gRPC</div>
                  <div className="form-section__body">
                    <FormRow label="Service name">
                      <input className="input" value={form.grpcService} onChange={(e) => set('grpcService', e.target.value)} />
                    </FormRow>
                  </div>
                </div>
              )}

              {form.network === 'httpupgrade' && (
                <div className="form-section">
                  <div className="form-section__title">HTTPUpgrade</div>
                  <div className="form-section__body">
                    <FormRow label="Path">
                      <input className="input" value={form.httpupgradePath} onChange={(e) => set('httpupgradePath', e.target.value)} />
                    </FormRow>
                  </div>
                </div>
              )}

              {form.network === 'xhttp' && (
                <div className="form-section">
                  <div className="form-section__title">XHTTP</div>
                  <div className="form-section__body">
                    <FormRow label="Path">
                      <input className="input" value={form.xhttpPath} onChange={(e) => set('xhttpPath', e.target.value)} />
                    </FormRow>
                    <FormRow label="Mode">
                      <select className="select" value={form.xhttpMode} onChange={(e) => set('xhttpMode', e.target.value)}>
                        <option value="auto">auto</option>
                        <option value="packet-up">packet-up</option>
                        <option value="stream-up">stream-up</option>
                        <option value="stream-one">stream-one</option>
                      </select>
                    </FormRow>
                  </div>
                </div>
              )}

              {form.network === 'kcp' && (
                <div className="form-section">
                  <div className="form-section__title">mKCP</div>
                  <div className="form-section__body">
                    <FormRow label="Seed">
                      <div className="input-compact">
                        <input className="input" value={form.kcpSeed} onChange={(e) => set('kcpSeed', e.target.value)} />
                        <button type="button" className="btn secondary btn-sm" onClick={() => set('kcpSeed', randomLowerAndNum(8))}>↻</button>
                      </div>
                    </FormRow>
                  </div>
                </div>
              )}

              {form.network === 'tcp' && (
                <div className="form-section">
                  <div className="form-section__title">TCP</div>
                  <div className="form-section__subtitle">No extra transport settings for raw TCP.</div>
                </div>
              )}
            </div>
          )}

          {tab === 'security' && needsStream && (
            <div>
              <FormRow label={tr('security')}>
                <div className="seg">
                  {SECURITIES.map((s) => (
                    <button
                      key={s}
                      type="button"
                      className={form.security === s ? 'active' : ''}
                      onClick={() => set('security', s)}
                    >
                      {s}
                    </button>
                  ))}
                </div>
              </FormRow>

              {form.security === 'tls' && (
                <div className="form-section">
                  <div className="form-section__title">TLS</div>
                  <div className="form-section__body">
                    <FormRow label="SNI / serverName">
                      <input className="input" value={form.tlsSNI} onChange={(e) => set('tlsSNI', e.target.value)} />
                    </FormRow>
                    <FormRow label="ALPN">
                      <input className="input" value={form.tlsALPN} onChange={(e) => set('tlsALPN', e.target.value)} placeholder="h2,http/1.1" />
                    </FormRow>
                    <FormRow label="Cert file">
                      <input className="input" value={form.tlsCertFile} onChange={(e) => set('tlsCertFile', e.target.value)} placeholder="/path/to/fullchain.pem" />
                    </FormRow>
                    <FormRow label="Key file">
                      <input className="input" value={form.tlsKeyFile} onChange={(e) => set('tlsKeyFile', e.target.value)} placeholder="/path/to/privkey.pem" />
                    </FormRow>
                    <FormRow label="Cert PEM" hint="Optional alternative to file path">
                      <textarea className="textarea" rows={4} value={form.tlsCertContent} onChange={(e) => set('tlsCertContent', e.target.value)} placeholder="-----BEGIN CERTIFICATE-----" />
                    </FormRow>
                    <FormRow label="Key PEM" hint="Optional alternative to file path">
                      <textarea className="textarea" rows={4} value={form.tlsKeyContent} onChange={(e) => set('tlsKeyContent', e.target.value)} placeholder="-----BEGIN PRIVATE KEY-----" />
                    </FormRow>
                    <FormRow label="Fingerprint">
                      <select className="select" value={form.tlsFingerprint} onChange={(e) => set('tlsFingerprint', e.target.value)}>
                        {FINGERPRINTS.map((fp) => <option key={fp} value={fp}>{fp}</option>)}
                      </select>
                    </FormRow>
                  </div>
                </div>
              )}

              {form.security === 'reality' && (
                <div className="form-section">
                  <div className="form-section__title">REALITY</div>
                  <div className="form-section__body">
                    <FormRow label="Dest / target *">
                      <div className="input-compact">
                        <input
                          className="input"
                          value={form.realityDest}
                          onChange={(e) => set('realityDest', e.target.value)}
                          placeholder="www.example.com:443"
                          required={form.security === 'reality'}
                        />
                        <button type="button" className="btn secondary btn-sm" disabled={busy} onClick={() => { void scanRealityTarget() }}>{tr('scan')}</button>
                      </div>
                    </FormRow>
                    <FormRow label="ServerNames *">
                      <input className="input" value={form.realitySNI} onChange={(e) => set('realitySNI', e.target.value)} placeholder="www.example.com" />
                    </FormRow>
                    <FormRow label="Private key">
                      <div className="input-compact">
                        <input className="input" value={form.realityPrivateKey} onChange={(e) => set('realityPrivateKey', e.target.value)} />
                        <button type="button" className="btn secondary btn-sm" title={tr('genKeys')} onClick={() => { void refreshKeys() }}>↻</button>
                      </div>
                    </FormRow>
                    <FormRow label="Public key">
                      <div className="input-compact">
                        <input className="input" value={form.realityPublicKey} onChange={(e) => set('realityPublicKey', e.target.value)} readOnly />
                        <button type="button" className="btn secondary btn-sm" onClick={() => { void refreshKeys() }}>{tr('genKeys')}</button>
                      </div>
                    </FormRow>
                    <FormRow label="Short IDs" hint="Comma-separated">
                      <div className="input-compact">
                        <input className="input" value={form.realityShortIds} onChange={(e) => set('realityShortIds', e.target.value)} />
                        <button type="button" className="btn secondary btn-sm" onClick={() => set('realityShortIds', randomShortIds().join(','))}>↻</button>
                      </div>
                    </FormRow>
                    <FormRow label="SpiderX">
                      <div className="input-compact">
                        <input className="input" value={form.realitySpiderX} onChange={(e) => set('realitySpiderX', e.target.value)} />
                        <button type="button" className="btn secondary btn-sm" onClick={() => set('realitySpiderX', randomSpiderX())}>↻</button>
                      </div>
                    </FormRow>
                    <FormRow label="Fingerprint">
                      <select className="select" value={form.realityFingerprint} onChange={(e) => set('realityFingerprint', e.target.value)}>
                        {FINGERPRINTS.map((fp) => <option key={fp} value={fp}>{fp}</option>)}
                      </select>
                    </FormRow>

                    {visionTip && (
                      <div className="alert info">
                        {tr('realityVisionTip')} <code>{visionTip}</code>
                      </div>
                    )}

                    <div className="row-actions" style={{ marginTop: '0.85rem' }}>
                      <button
                        type="button"
                        className="btn secondary btn-sm"
                        onClick={() => setForm((prev) => applyCloudflareRealityDefaults(prev))}
                      >
                        {tr('cloudflareDefaults')}
                      </button>
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}

          {tab === 'sniffing' && needsStream && (
            <div>
              <FormRow label="Sniffing">
                <label className="form-switch">
                  <input type="checkbox" checked={form.sniffEnabled} onChange={(e) => set('sniffEnabled', e.target.checked)} />
                  <span>{form.sniffEnabled ? 'On' : 'Off'}</span>
                </label>
              </FormRow>
              <FormRow label="routeOnly">
                <label className="form-switch">
                  <input type="checkbox" checked={form.sniffRouteOnly} onChange={(e) => set('sniffRouteOnly', e.target.checked)} />
                  <span>{form.sniffRouteOnly ? 'On' : 'Off'}</span>
                </label>
              </FormRow>
              <FormRow label="metadataOnly">
                <label className="form-switch">
                  <input type="checkbox" checked={form.sniffMetadataOnly} onChange={(e) => set('sniffMetadataOnly', e.target.checked)} />
                  <span>{form.sniffMetadataOnly ? 'On' : 'Off'}</span>
                </label>
              </FormRow>
              <FormRow label="destOverride" hint="Comma-separated">
                <input className="input" value={form.sniffDestOverride} onChange={(e) => set('sniffDestOverride', e.target.value)} />
              </FormRow>
              <FormRow label="domainsExcluded" hint="Comma-separated">
                <input className="input" value={form.sniffDomainsExcluded} onChange={(e) => set('sniffDomainsExcluded', e.target.value)} />
              </FormRow>
              <FormRow label="ipsExcluded" hint="Comma-separated">
                <input className="input" value={form.sniffIpsExcluded} onChange={(e) => set('sniffIpsExcluded', e.target.value)} />
              </FormRow>
            </div>
          )}

          {tab === 'advanced' && showAdvanced && (
            <div>
              {showFallbacks && (
                <div className="form-section">
                  <div className="form-section__title">Fallbacks</div>
                  <div className="form-section__subtitle">JSON array</div>
                  <div className="form-section__body">
                    <FormRow stack>
                      <textarea
                        className="textarea"
                        rows={8}
                        value={form.fallbacksJSON}
                        onChange={(e) => set('fallbacksJSON', e.target.value)}
                        placeholder='[{"dest":"80","xver":0}]'
                      />
                    </FormRow>
                  </div>
                </div>
              )}
              {needsStream && (
                <div className="form-section">
                  <div className="form-section__title">Sockopt</div>
                  <div className="form-section__subtitle">JSON object, optional</div>
                  <div className="form-section__body">
                    <FormRow stack>
                      <textarea
                        className="textarea"
                        rows={6}
                        value={form.sockoptJSON}
                        onChange={(e) => set('sockoptJSON', e.target.value)}
                        placeholder='{"tcpFastOpen":true}'
                      />
                    </FormRow>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        {error && <p className="error">{error}</p>}
        <div className="form-actions">
          <div className="form-actions__end">
            <button className="btn secondary" type="button" onClick={onClose}>{tr('cancel')}</button>
            <button className="btn" type="submit" disabled={busy}>{busy ? '…' : tr('save')}</button>
          </div>
        </div>
      </form>
    </div>
  )
}
