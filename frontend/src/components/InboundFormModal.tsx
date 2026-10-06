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
  buildInboundSettings,
  buildStreamSettings,
  emptyInboundForm,
  parseInboundToForm,
  suggestedFlow,
} from '../lib/inboundForm'

type Props = {
  open: boolean
  mode: 'add' | 'edit'
  inbound: Inbound | null
  onClose: () => void
  onSaved: () => void
}

type Tab = 'general' | 'network' | 'security' | 'client'

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
      const f = emptyInboundForm()
      f.clientEmail = `user-${Date.now().toString(36)}@we1b`
      setForm(f)
      void refreshUUID()
    }
  }, [open, mode, inbound])

  function set<K extends keyof InboundFormState>(key: K, value: InboundFormState[K]) {
    setForm((prev) => {
      const next = { ...prev, [key]: value }
      if (key === 'security' || key === 'network' || key === 'protocol') {
        const flow = suggestedFlow(next)
        if (flow) next.clientFlow = flow
        if (key === 'security' && value === 'reality' && !next.realityPrivateKey) {
          queueMicrotask(() => { void refreshKeys() })
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
        realityShortId: keys.shortId,
      }))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'keys error')
    }
  }

  async function refreshUUID() {
    try {
      const r = await api<{ uuid: string }>('/tools/uuid')
      setForm((prev) => ({ ...prev, clientUUID: r.uuid }))
    } catch { /* ignore */ }
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      if (form.security === 'reality' && !form.realityPrivateKey) {
        throw new Error('Generate REALITY keys first')
      }
      const payload = {
        remark: form.remark,
        port: form.port,
        listen: form.listen || '0.0.0.0',
        protocol: form.protocol,
        enable: form.enable,
        settings: buildInboundSettings(form),
        streamSettings: buildStreamSettings(form),
      }
      let created: Inbound
      if (mode === 'edit' && form.id) {
        created = await api<Inbound>(`/inbounds/${form.id}`, { method: 'PUT', body: JSON.stringify({ ...payload, id: form.id }) })
      } else {
        created = await api<Inbound>('/inbounds', { method: 'POST', body: JSON.stringify(payload) })
        if (!['tun', 'tunnel'].includes(form.protocol)) {
          const flow = form.clientFlow || suggestedFlow(form)
          await api('/clients', {
            method: 'POST',
            body: JSON.stringify({
              inboundId: created.id,
              email: form.clientEmail || `${form.protocol}-${form.port}@we1b`,
              enable: true,
              uuid: form.clientUUID || undefined,
              password: form.clientPassword || undefined,
              flow,
              totalGB: form.clientTotalGB || 0,
              expiryTime: form.clientExpiryDays > 0
                ? Date.now() + form.clientExpiryDays * 86400000
                : 0,
            }),
          })
        }
      }
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  if (!open) return null

  const needsStream = !['tun', 'tunnel', 'mtproto', 'tuic', 'hysteria2'].includes(form.protocol)
  const tabs: { id: Tab; label: string; show?: boolean }[] = [
    { id: 'general', label: tr('tabGeneral') },
    { id: 'network', label: tr('tabNetwork'), show: needsStream },
    { id: 'security', label: tr('tabSecurity'), show: needsStream },
    { id: 'client', label: tr('tabClient'), show: mode === 'add' && !['tun', 'tunnel'].includes(form.protocol) },
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
              <label className="label">Listen</label>
              <input className="input" value={form.listen} onChange={(e) => set('listen', e.target.value)} />
            </div>
            {form.protocol === 'shadowsocks' && (
              <div className="field">
                <label className="label">Method</label>
                <select className="select" value={form.ssMethod} onChange={(e) => set('ssMethod', e.target.value)}>
                  {SS_METHODS.map((m) => <option key={m} value={m}>{m}</option>)}
                </select>
              </div>
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
                {NETWORKS.map((n) => <option key={n} value={n}>{n}</option>)}
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
                <input className="input" value={form.kcpSeed} onChange={(e) => set('kcpSeed', e.target.value)} />
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
              <div className="field">
                <label className="label">SNI / serverName</label>
                <input className="input" value={form.tlsSNI} onChange={(e) => set('tlsSNI', e.target.value)} />
              </div>
            )}
            {form.security === 'reality' && (
              <>
                <div className="row-actions" style={{ marginBottom: 10 }}>
                  <button type="button" className="btn secondary" onClick={() => refreshKeys()}>{tr('genKeys')}</button>
                </div>
                <div className="grid2">
                  <div className="field">
                    <label className="label">Dest (target)</label>
                    <input className="input" value={form.realityDest} onChange={(e) => set('realityDest', e.target.value)} />
                  </div>
                  <div className="field">
                    <label className="label">SNI / serverNames</label>
                    <input className="input" value={form.realitySNI} onChange={(e) => set('realitySNI', e.target.value)} />
                  </div>
                  <div className="field">
                    <label className="label">Private key</label>
                    <input className="input" value={form.realityPrivateKey} onChange={(e) => set('realityPrivateKey', e.target.value)} />
                  </div>
                  <div className="field">
                    <label className="label">Public key</label>
                    <input className="input" value={form.realityPublicKey} onChange={(e) => set('realityPublicKey', e.target.value)} readOnly />
                  </div>
                  <div className="field">
                    <label className="label">Short ID</label>
                    <input className="input" value={form.realityShortId} onChange={(e) => set('realityShortId', e.target.value)} />
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

        {tab === 'client' && mode === 'add' && (
          <div className="grid2">
            <div className="field">
              <label className="label">Email</label>
              <input className="input" value={form.clientEmail} onChange={(e) => set('clientEmail', e.target.value)} />
            </div>
            <div className="field">
              <label className="label">UUID</label>
              <div className="row-actions">
                <input className="input" value={form.clientUUID} onChange={(e) => set('clientUUID', e.target.value)} />
                <button type="button" className="btn secondary" onClick={() => refreshUUID()}>UUID</button>
              </div>
            </div>
            {(form.protocol === 'trojan' || form.protocol === 'shadowsocks') && (
              <div className="field">
                <label className="label">{tr('password')}</label>
                <input className="input" value={form.clientPassword} onChange={(e) => set('clientPassword', e.target.value)} />
              </div>
            )}
            {form.protocol === 'vless' && (
              <div className="field">
                <label className="label">Flow</label>
                <select className="select" value={form.clientFlow} onChange={(e) => set('clientFlow', e.target.value)}>
                  <option value="">(none)</option>
                  <option value="xtls-rprx-vision">xtls-rprx-vision</option>
                </select>
              </div>
            )}
            <div className="field">
              <label className="label">Total GB (0 = ∞)</label>
              <input className="input" type="number" min={0} value={form.clientTotalGB} onChange={(e) => set('clientTotalGB', Number(e.target.value))} />
            </div>
            <div className="field">
              <label className="label">Expiry days (0 = never)</label>
              <input className="input" type="number" min={0} value={form.clientExpiryDays} onChange={(e) => set('clientExpiryDays', Number(e.target.value))} />
            </div>
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
        .tabs { display: flex; flex-wrap: wrap; gap: 0.35rem; margin: 0 0 1rem; }
        .tab {
          border: 1px solid var(--border);
          background: transparent;
          color: var(--text-muted);
          border-radius: 999px;
          padding: 0.4rem 0.85rem;
          cursor: pointer;
          font-weight: 600;
          font-size: 0.85rem;
        }
        .tab.active {
          background: var(--accent-soft);
          color: var(--accent);
          border-color: transparent;
        }
      `}</style>
    </div>
  )
}
