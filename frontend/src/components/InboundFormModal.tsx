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
} from '../lib/inboundForm'
import { randomLowerAndNum, randomShortIds, randomSpiderX } from '../lib/random'

function ssPasswordForMethod(method: string): string {
  return method.startsWith('2022-') ? randomSS2022Password(method) : randomLowerAndNum(32)
}

type Props = {
  open: boolean
  mode: 'add' | 'edit'
  inbound: Inbound | null
  onClose: () => void
  onSaved: (created?: Inbound) => void
}

type Tab = 'general' | 'network' | 'security' | 'sniffing' | 'advanced'

export function InboundFormModal({ open, mode, inbound, onClose, onSaved }: Props) {
  const { tr } = useApp()
  const [tab, setTab] = useState<Tab>('general')
  const [form, setForm] = useState<InboundFormState>(emptyInboundForm())
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setTab('general')
    setError('')
    if (mode === 'edit' && inbound) {
      setForm(parseInboundToForm(inbound))
    } else {
      setForm(emptyInboundForm())
    }
  }, [open, mode, inbound])

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
    const target = form.realityDest.trim() || 'www.cloudflare.com:443'
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
        throw new Error('REALITY dest/target is required (e.g. www.cloudflare.com:443)')
      }
      if (form.security === 'reality' && !form.realitySNI) {
        throw new Error('REALITY serverNames (SNI) is required')
      }
      if ((form.protocol === 'wireguard' || form.protocol === 'amneziawg') && !form.wgSecretKey.trim()) {
        throw new Error('Generate WireGuard secretKey first')
      }
      const payload = {
        remark: form.remark,
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
  const showFallbacks = form.protocol === 'vless' || form.protocol === 'trojan'
  const showAdvanced = showFallbacks || needsStream
  const isWg = form.protocol === 'wireguard' || form.protocol === 'amneziawg'
  const tabs: { id: Tab; label: string; show?: boolean }[] = [
    { id: 'general', label: tr('tabGeneral') },
    { id: 'network', label: tr('tabNetwork'), show: needsStream },
    { id: 'security', label: tr('tabSecurity'), show: needsStream },
    { id: 'sniffing', label: tr('tabSniffing'), show: needsStream },
    { id: 'advanced', label: 'Advanced', show: showAdvanced },
  ]

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <form className="modal inbound-modal" onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <h3>{mode === 'edit' ? tr('edit') : tr('create')} inbound</h3>

        <div className="tabs">
          {tabs.filter((t) => t.show !== false).map((t) => (
            <button key={t.id} type="button" className={`tab ${tab === t.id ? 'active' : ''}`} onClick={() => setTab(t.id)}>
              {t.label}
            </button>
          ))}
        </div>

        {tab === 'general' && (
          <div className="grid2">
            <div className="field">
              <label className="label">{tr('remark')}</label>
              <input className="input" value={form.remark} onChange={(e) => set('remark', e.target.value)} placeholder="My VLESS" />
            </div>
            <div className="field">
              <label className="label">{tr('port')}</label>
              <input className="input" type="number" min={1} max={65535} value={form.port} onChange={(e) => set('port', Number(e.target.value))} required />
            </div>
            <div className="field">
              <label className="label">{tr('protocol')}</label>
              <select className="select" value={form.protocol} onChange={(e) => set('protocol', e.target.value)} disabled={mode === 'edit'}>
                {PROTOCOLS.map((p) => <option key={p} value={p}>{p}</option>)}
              </select>
            </div>
            <div className="field">
              <label className="label">Listen <span className="hint">(empty = 0.0.0.0)</span></label>
              <input className="input" value={form.listen} onChange={(e) => set('listen', e.target.value)} placeholder="0.0.0.0" />
            </div>
            {form.protocol === 'shadowsocks' && (
              <>
                <div className="field">
                  <label className="label">Method</label>
                  <select className="select" value={form.ssMethod} onChange={(e) => set('ssMethod', e.target.value)}>
                    {SS_METHODS.map((m) => <option key={m} value={m}>{m}</option>)}
                  </select>
                </div>
                <div className="field">
                  <label className="label">{tr('password')}</label>
                  <div className="row-actions">
                    <input className="input" value={form.ssPassword} onChange={(e) => set('ssPassword', e.target.value)} />
                    <button type="button" className="btn secondary" onClick={() => set('ssPassword', ssPasswordForMethod(form.ssMethod))}>↻</button>
                  </div>
                </div>
              </>
            )}
            {isWg && (
              <>
                <div className="field" style={{ gridColumn: '1 / -1' }}>
                  <label className="label">Secret key</label>
                  <div className="row-actions">
                    <input className="input" value={form.wgSecretKey} onChange={(e) => set('wgSecretKey', e.target.value)} />
                    <button type="button" className="btn secondary" onClick={() => { void genWgKeys() }}>↻</button>
                  </div>
                </div>
                <div className="field">
                  <label className="label">Address</label>
                  <input className="input" value={form.wgAddress} onChange={(e) => set('wgAddress', e.target.value)} placeholder="10.0.0.1/24" />
                </div>
                <div className="field">
                  <label className="label">MTU</label>
                  <input className="input" type="number" min={576} max={65535} value={form.wgMtu} onChange={(e) => set('wgMtu', Number(e.target.value))} />
                </div>
                <p className="page-sub" style={{ gridColumn: '1 / -1', margin: 0 }}>Peers are empty — panel clients can be mapped as peers later.</p>
              </>
            )}
            {form.protocol === 'amneziawg' && (
              <>
                <div className="row-actions" style={{ gridColumn: '1 / -1' }}>
                  <button type="button" className="btn secondary" onClick={() => setForm((prev) => ({ ...prev, ...randomAwgObfuscation() }))}>
                    Regenerate obfuscation
                  </button>
                </div>
                <div className="field"><label className="label">Jc</label><input className="input" type="number" value={form.awgJc} onChange={(e) => set('awgJc', Number(e.target.value))} /></div>
                <div className="field"><label className="label">Jmin</label><input className="input" type="number" value={form.awgJmin} onChange={(e) => set('awgJmin', Number(e.target.value))} /></div>
                <div className="field"><label className="label">Jmax</label><input className="input" type="number" value={form.awgJmax} onChange={(e) => set('awgJmax', Number(e.target.value))} /></div>
                <div className="field"><label className="label">S1</label><input className="input" type="number" value={form.awgS1} onChange={(e) => set('awgS1', Number(e.target.value))} /></div>
                <div className="field"><label className="label">S2</label><input className="input" type="number" value={form.awgS2} onChange={(e) => set('awgS2', Number(e.target.value))} /></div>
                <div className="field"><label className="label">H1</label><input className="input" value={form.awgH1} onChange={(e) => set('awgH1', e.target.value)} /></div>
                <div className="field"><label className="label">H2</label><input className="input" value={form.awgH2} onChange={(e) => set('awgH2', e.target.value)} /></div>
                <div className="field"><label className="label">H3</label><input className="input" value={form.awgH3} onChange={(e) => set('awgH3', e.target.value)} /></div>
                <div className="field"><label className="label">H4</label><input className="input" value={form.awgH4} onChange={(e) => set('awgH4', e.target.value)} /></div>
              </>
            )}
            {form.protocol === 'tuic' && (
              <>
                <div className="field">
                  <label className="label">Congestion control</label>
                  <select className="select" value={form.tuicCongestion} onChange={(e) => set('tuicCongestion', e.target.value)}>
                    <option value="bbr">bbr</option>
                    <option value="cubic">cubic</option>
                    <option value="new_reno">new_reno</option>
                  </select>
                </div>
                <p className="page-sub" style={{ gridColumn: '1 / -1', margin: 0 }}>Users list starts empty — panel clients may not map to TUIC users yet.</p>
              </>
            )}
            {form.protocol === 'hysteria2' && (
              <>
                <div className="field">
                  <label className="label">{tr('password')}</label>
                  <div className="row-actions">
                    <input className="input" value={form.hy2Password} onChange={(e) => set('hy2Password', e.target.value)} />
                    <button type="button" className="btn secondary" onClick={() => set('hy2Password', randomLowerAndNum(16))}>↻</button>
                  </div>
                </div>
                <p className="page-sub" style={{ gridColumn: '1 / -1', margin: 0 }}>Users list starts empty — panel clients may not map to Hysteria2 users yet.</p>
              </>
            )}
            {form.protocol === 'mtproto' && (
              <div className="field" style={{ gridColumn: '1 / -1' }}>
                <label className="label">Fake TLS domain</label>
                <input className="input" value={form.mtprotoFakeTlsDomain} onChange={(e) => set('mtprotoFakeTlsDomain', e.target.value)} placeholder="www.cloudflare.com" />
              </div>
            )}
            {(form.protocol === 'http' || form.protocol === 'socks') && (
              <p className="page-sub" style={{ gridColumn: '1 / -1', margin: 0 }}>
                Clients on this inbound become {form.protocol.toUpperCase()} accounts (user/pass).
              </p>
            )}
            <div className="field">
              <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <input type="checkbox" checked={form.enable} onChange={(e) => set('enable', e.target.checked)} />
                {tr('enable')}
              </label>
            </div>
          </div>
        )}

        {tab === 'network' && needsStream && (
          <div>
            <div className="field">
              <label className="label">{tr('network')}</label>
              <select className="select" value={form.network} onChange={(e) => set('network', e.target.value as InboundFormState['network'])}>
                {NETWORKS.map((n) => <option key={n} value={n}>{n === 'tcp' ? 'tcp (raw)' : n}</option>)}
              </select>
            </div>
            {form.network === 'ws' && (
              <div className="grid2">
                <div className="field">
                  <label className="label">Path</label>
                  <input className="input" value={form.wsPath} onChange={(e) => set('wsPath', e.target.value)} />
                </div>
                <div className="field">
                  <label className="label">Host</label>
                  <input className="input" value={form.wsHost} onChange={(e) => set('wsHost', e.target.value)} />
                </div>
              </div>
            )}
            {form.network === 'grpc' && (
              <div className="field">
                <label className="label">Service name</label>
                <input className="input" value={form.grpcService} onChange={(e) => set('grpcService', e.target.value)} />
              </div>
            )}
            {form.network === 'httpupgrade' && (
              <div className="field">
                <label className="label">Path</label>
                <input className="input" value={form.httpupgradePath} onChange={(e) => set('httpupgradePath', e.target.value)} />
              </div>
            )}
            {form.network === 'xhttp' && (
              <div className="grid2">
                <div className="field">
                  <label className="label">Path</label>
                  <input className="input" value={form.xhttpPath} onChange={(e) => set('xhttpPath', e.target.value)} />
                </div>
                <div className="field">
                  <label className="label">Mode</label>
                  <select className="select" value={form.xhttpMode} onChange={(e) => set('xhttpMode', e.target.value)}>
                    <option value="auto">auto</option>
                    <option value="packet-up">packet-up</option>
                    <option value="stream-up">stream-up</option>
                    <option value="stream-one">stream-one</option>
                  </select>
                </div>
              </div>
            )}
            {form.network === 'kcp' && (
              <div className="field">
                <label className="label">Seed</label>
                <div className="row-actions">
                  <input className="input" value={form.kcpSeed} onChange={(e) => set('kcpSeed', e.target.value)} />
                  <button type="button" className="btn secondary" onClick={() => set('kcpSeed', randomLowerAndNum(8))}>↻</button>
                </div>
              </div>
            )}
          </div>
        )}

        {tab === 'security' && needsStream && (
          <div>
            <div className="field">
              <label className="label">{tr('security')}</label>
              <select className="select" value={form.security} onChange={(e) => set('security', e.target.value as InboundFormState['security'])}>
                {SECURITIES.map((s) => <option key={s} value={s}>{s}</option>)}
              </select>
            </div>
            {form.security === 'tls' && (
              <div className="grid2">
                <div className="field">
                  <label className="label">SNI / serverName</label>
                  <input className="input" value={form.tlsSNI} onChange={(e) => set('tlsSNI', e.target.value)} />
                </div>
                <div className="field">
                  <label className="label">ALPN</label>
                  <input className="input" value={form.tlsALPN} onChange={(e) => set('tlsALPN', e.target.value)} placeholder="h2,http/1.1" />
                </div>
                <div className="field">
                  <label className="label">Cert file path</label>
                  <input className="input" value={form.tlsCertFile} onChange={(e) => set('tlsCertFile', e.target.value)} placeholder="/path/to/fullchain.pem" />
                </div>
                <div className="field">
                  <label className="label">Key file path</label>
                  <input className="input" value={form.tlsKeyFile} onChange={(e) => set('tlsKeyFile', e.target.value)} placeholder="/path/to/privkey.pem" />
                </div>
                <div className="field" style={{ gridColumn: '1 / -1' }}>
                  <label className="label">Cert PEM content <span className="hint">(optional alternative to file)</span></label>
                  <textarea className="input" rows={4} value={form.tlsCertContent} onChange={(e) => set('tlsCertContent', e.target.value)} placeholder="-----BEGIN CERTIFICATE-----" />
                </div>
                <div className="field" style={{ gridColumn: '1 / -1' }}>
                  <label className="label">Key PEM content <span className="hint">(optional alternative to file)</span></label>
                  <textarea className="input" rows={4} value={form.tlsKeyContent} onChange={(e) => set('tlsKeyContent', e.target.value)} placeholder="-----BEGIN PRIVATE KEY-----" />
                </div>
                <div className="field">
                  <label className="label">Fingerprint</label>
                  <select className="select" value={form.tlsFingerprint} onChange={(e) => set('tlsFingerprint', e.target.value)}>
                    {FINGERPRINTS.map((fp) => <option key={fp} value={fp}>{fp}</option>)}
                  </select>
                </div>
              </div>
            )}
            {form.security === 'reality' && (
              <>
                <div className="row-actions" style={{ marginBottom: 10 }}>
                  <button type="button" className="btn secondary" onClick={() => refreshKeys()}>{tr('genKeys')}</button>
                  <button type="button" className="btn secondary" onClick={() => setForm((prev) => applyCloudflareRealityDefaults(prev))}>Fill Cloudflare defaults</button>
                  <button type="button" className="btn secondary" onClick={() => set('realityShortIds', randomShortIds().join(','))}>ShortIds</button>
                  <button type="button" className="btn secondary" onClick={() => set('realitySpiderX', randomSpiderX())}>SpiderX</button>
                </div>
                <div className="grid2">
                  <div className="field">
                    <label className="label">Dest / target *</label>
                    <div className="row-actions">
                      <input className="input" value={form.realityDest} onChange={(e) => set('realityDest', e.target.value)} placeholder="www.cloudflare.com:443" required={form.security === 'reality'} />
                      <button type="button" className="btn secondary" disabled={busy} onClick={() => { void scanRealityTarget() }}>{tr('scan')}</button>
                    </div>
                  </div>
                  <div className="field">
                    <label className="label">ServerNames (SNI) *</label>
                    <input className="input" value={form.realitySNI} onChange={(e) => set('realitySNI', e.target.value)} placeholder="www.cloudflare.com" />
                  </div>
                  <div className="field">
                    <label className="label">Private key</label>
                    <input className="input" value={form.realityPrivateKey} onChange={(e) => set('realityPrivateKey', e.target.value)} />
                  </div>
                  <div className="field">
                    <label className="label">Public key</label>
                    <input className="input" value={form.realityPublicKey} onChange={(e) => set('realityPublicKey', e.target.value)} readOnly />
                  </div>
                  <div className="field" style={{ gridColumn: '1 / -1' }}>
                    <label className="label">Short IDs (csv)</label>
                    <input className="input" value={form.realityShortIds} onChange={(e) => set('realityShortIds', e.target.value)} />
                  </div>
                  <div className="field">
                    <label className="label">Fingerprint</label>
                    <select className="select" value={form.realityFingerprint} onChange={(e) => set('realityFingerprint', e.target.value)}>
                      {FINGERPRINTS.map((fp) => <option key={fp} value={fp}>{fp}</option>)}
                    </select>
                  </div>
                  <div className="field">
                    <label className="label">SpiderX</label>
                    <input className="input" value={form.realitySpiderX} onChange={(e) => set('realitySpiderX', e.target.value)} />
                  </div>
                </div>
              </>
            )}
          </div>
        )}

        {tab === 'sniffing' && needsStream && (
          <div className="grid2">
            <div className="field">
              <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <input type="checkbox" checked={form.sniffEnabled} onChange={(e) => set('sniffEnabled', e.target.checked)} />
                {tr('enable')} sniffing
              </label>
            </div>
            <div className="field">
              <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <input type="checkbox" checked={form.sniffRouteOnly} onChange={(e) => set('sniffRouteOnly', e.target.checked)} />
                routeOnly
              </label>
            </div>
            <div className="field">
              <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <input type="checkbox" checked={form.sniffMetadataOnly} onChange={(e) => set('sniffMetadataOnly', e.target.checked)} />
                metadataOnly
              </label>
            </div>
            <div className="field" style={{ gridColumn: '1 / -1' }}>
              <label className="label">destOverride (csv)</label>
              <input className="input" value={form.sniffDestOverride} onChange={(e) => set('sniffDestOverride', e.target.value)} />
            </div>
            <div className="field">
              <label className="label">domainsExcluded (csv)</label>
              <input className="input" value={form.sniffDomainsExcluded} onChange={(e) => set('sniffDomainsExcluded', e.target.value)} />
            </div>
            <div className="field">
              <label className="label">ipsExcluded (csv)</label>
              <input className="input" value={form.sniffIpsExcluded} onChange={(e) => set('sniffIpsExcluded', e.target.value)} />
            </div>
          </div>
        )}

        {tab === 'advanced' && showAdvanced && (
          <div className="grid2">
            {showFallbacks && (
              <div className="field" style={{ gridColumn: '1 / -1' }}>
                <label className="label">Fallbacks (JSON array)</label>
                <textarea
                  className="input"
                  rows={8}
                  value={form.fallbacksJSON}
                  onChange={(e) => set('fallbacksJSON', e.target.value)}
                  placeholder='[{"dest":"80","xver":0}]'
                  style={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace', fontSize: '0.85rem' }}
                />
              </div>
            )}
            {needsStream && (
              <div className="field" style={{ gridColumn: '1 / -1' }}>
                <label className="label">Sockopt (JSON object, optional)</label>
                <textarea
                  className="input"
                  rows={6}
                  value={form.sockoptJSON}
                  onChange={(e) => set('sockoptJSON', e.target.value)}
                  placeholder='{"tcpFastOpen":true}'
                  style={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace', fontSize: '0.85rem' }}
                />
              </div>
            )}
          </div>
        )}

        {error && <p className="error">{error}</p>}
        <div className="row-actions" style={{ marginTop: 12 }}>
          <button className="btn" type="submit" disabled={busy}>{busy ? '…' : tr('save')}</button>
          <button className="btn secondary" type="button" onClick={onClose}>{tr('cancel')}</button>
        </div>
      </form>
      <style>{`
        .inbound-modal { width: min(820px, 100%); }
        .hint { color: var(--text-muted); font-weight: 400; font-size: 0.8em; }
      `}</style>
    </div>
  )
}
