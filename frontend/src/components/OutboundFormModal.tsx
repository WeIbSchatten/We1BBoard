import { FormEvent, useEffect, useState } from 'react'
import { api, type Outbound } from '../api'
import { useApp } from '../AppContext'
import {
  OUTBOUND_PROTOCOLS,
  OutboundFormState,
  buildOutboundSettings,
  buildOutboundStream,
  emptyOutboundForm,
  parseOutboundToForm,
} from '../lib/outboundForm'
import { FINGERPRINTS, NETWORKS, SECURITIES } from '../lib/inboundForm'
import { randomUUID } from '../lib/random'

type Props = {
  open: boolean
  mode: 'add' | 'edit'
  outbound: Outbound | null
  onClose: () => void
  onSaved: () => void
}

type Tab = 'basic' | 'stream'

export function OutboundFormModal({ open, mode, outbound, onClose, onSaved }: Props) {
  const { tr } = useApp()
  const [tab, setTab] = useState<Tab>('basic')
  const [form, setForm] = useState<OutboundFormState>(emptyOutboundForm())
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setTab('basic')
    setError('')
    if (mode === 'edit' && outbound) {
      setForm(parseOutboundToForm(outbound))
    } else {
      const f = emptyOutboundForm()
      f.uuid = randomUUID()
      setForm(f)
    }
  }, [open, mode, outbound])

  function set<K extends keyof OutboundFormState>(key: K, value: OutboundFormState[K]) {
    setForm((prev) => {
      const next = { ...prev, [key]: value }
      if (key === 'protocol') {
        if (value === 'socks') next.port = 1080
        else if (value === 'http') next.port = 8080
        else if (!['freedom', 'blackhole'].includes(String(value))) next.port = 443
        if (['vless', 'vmess'].includes(String(value)) && !next.uuid) next.uuid = randomUUID()
      }
      return next
    })
  }

  const isProxy = !['freedom', 'blackhole'].includes(form.protocol)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const payload = {
        tag: form.tag,
        protocol: form.protocol,
        enable: form.enable,
        remark: form.remark,
        settings: buildOutboundSettings(form),
        streamSettings: buildOutboundStream(form),
      }
      if (mode === 'edit' && form.id) {
        await api(`/outbounds/${form.id}`, { method: 'PUT', body: JSON.stringify({ ...payload, id: form.id }) })
      } else {
        await api('/outbounds', { method: 'POST', body: JSON.stringify(payload) })
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

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <form className="modal" style={{ width: 'min(720px, 100%)' }} onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <h3>{mode === 'edit' ? tr('edit') : tr('create')} outbound</h3>
        <div className="tabs" style={{ display: 'flex', gap: 6, marginBottom: 12 }}>
          <button type="button" className={`tab ${tab === 'basic' ? 'active' : ''}`} onClick={() => setTab('basic')}>{tr('tabGeneral')}</button>
          {isProxy && (
            <button type="button" className={`tab ${tab === 'stream' ? 'active' : ''}`} onClick={() => setTab('stream')}>{tr('tabNetwork')} / {tr('tabSecurity')}</button>
          )}
        </div>

        {tab === 'basic' && (
          <div className="grid2">
            <div className="field">
              <label className="label">Tag *</label>
              <input className="input" value={form.tag} onChange={(e) => set('tag', e.target.value)} required disabled={mode === 'edit' && ['direct', 'blocked'].includes(form.tag)} />
            </div>
            <div className="field">
              <label className="label">{tr('protocol')}</label>
              <select className="select" value={form.protocol} onChange={(e) => set('protocol', e.target.value)} disabled={mode === 'edit' && ['direct', 'blocked'].includes(form.tag)}>
                {OUTBOUND_PROTOCOLS.map((p) => <option key={p} value={p}>{p}</option>)}
              </select>
            </div>
            <div className="field">
              <label className="label">{tr('remark')}</label>
              <input className="input" value={form.remark} onChange={(e) => set('remark', e.target.value)} />
            </div>
            <div className="field">
              <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <input type="checkbox" checked={form.enable} onChange={(e) => set('enable', e.target.checked)} />
                {tr('enable')}
              </label>
            </div>
            {form.protocol === 'freedom' && (
              <div className="field">
                <label className="label">domainStrategy</label>
                <select className="select" value={form.domainStrategy} onChange={(e) => set('domainStrategy', e.target.value)}>
                  <option value="AsIs">AsIs</option>
                  <option value="UseIP">UseIP</option>
                  <option value="UseIPv4">UseIPv4</option>
                  <option value="UseIPv6">UseIPv6</option>
                </select>
              </div>
            )}
            {isProxy && (
              <>
                <div className="field">
                  <label className="label">Address *</label>
                  <input className="input" value={form.address} onChange={(e) => set('address', e.target.value)} required />
                </div>
                <div className="field">
                  <label className="label">{tr('port')}</label>
                  <input className="input" type="number" value={form.port} onChange={(e) => set('port', Number(e.target.value))} required />
                </div>
                {['vless', 'vmess'].includes(form.protocol) && (
                  <div className="field">
                    <label className="label">UUID</label>
                    <div className="row-actions">
                      <input className="input" value={form.uuid} onChange={(e) => set('uuid', e.target.value)} />
                      <button type="button" className="btn secondary" onClick={() => set('uuid', randomUUID())}>↻</button>
                    </div>
                  </div>
                )}
                {form.protocol === 'vless' && (
                  <div className="field">
                    <label className="label">Flow</label>
                    <select className="select" value={form.flow} onChange={(e) => set('flow', e.target.value)}>
                      <option value="">(none)</option>
                      <option value="xtls-rprx-vision">xtls-rprx-vision</option>
                    </select>
                  </div>
                )}
                {['trojan', 'shadowsocks', 'socks', 'http'].includes(form.protocol) && (
                  <div className="field">
                    <label className="label">{tr('password')}</label>
                    <input className="input" value={form.password} onChange={(e) => set('password', e.target.value)} />
                  </div>
                )}
                {form.protocol === 'shadowsocks' && (
                  <div className="field">
                    <label className="label">Method</label>
                    <input className="input" value={form.method} onChange={(e) => set('method', e.target.value)} />
                  </div>
                )}
                {['socks', 'http'].includes(form.protocol) && (
                  <div className="field">
                    <label className="label">User</label>
                    <input className="input" value={form.email} onChange={(e) => set('email', e.target.value)} />
                  </div>
                )}
              </>
            )}
          </div>
        )}

        {tab === 'stream' && isProxy && (
          <div className="grid2">
            <div className="field">
              <label className="label">{tr('network')}</label>
              <select className="select" value={form.network} onChange={(e) => set('network', e.target.value)}>
                {NETWORKS.map((n) => <option key={n} value={n}>{n}</option>)}
              </select>
            </div>
            <div className="field">
              <label className="label">{tr('security')}</label>
              <select className="select" value={form.security} onChange={(e) => set('security', e.target.value)}>
                {SECURITIES.map((s) => <option key={s} value={s}>{s}</option>)}
              </select>
            </div>
            {form.network === 'ws' && (
              <>
                <div className="field"><label className="label">Path</label><input className="input" value={form.path} onChange={(e) => set('path', e.target.value)} /></div>
                <div className="field"><label className="label">Host</label><input className="input" value={form.host} onChange={(e) => set('host', e.target.value)} /></div>
              </>
            )}
            {form.network === 'grpc' && (
              <div className="field"><label className="label">Service name</label><input className="input" value={form.serviceName} onChange={(e) => set('serviceName', e.target.value)} /></div>
            )}
            {form.security !== 'none' && (
              <div className="field"><label className="label">SNI</label><input className="input" value={form.sni} onChange={(e) => set('sni', e.target.value)} /></div>
            )}
            {form.security === 'reality' && (
              <>
                <div className="field"><label className="label">Public key</label><input className="input" value={form.publicKey} onChange={(e) => set('publicKey', e.target.value)} /></div>
                <div className="field"><label className="label">Short ID</label><input className="input" value={form.shortId} onChange={(e) => set('shortId', e.target.value)} /></div>
              </>
            )}
            {form.security !== 'none' && (
              <div className="field">
                <label className="label">Fingerprint</label>
                <select className="select" value={form.fingerprint} onChange={(e) => set('fingerprint', e.target.value)}>
                  {FINGERPRINTS.map((fp) => <option key={fp} value={fp}>{fp}</option>)}
                </select>
              </div>
            )}
          </div>
        )}

        {error && <p className="error">{error}</p>}
        <div className="row-actions">
          <button className="btn" type="submit" disabled={busy}>{busy ? '…' : tr('save')}</button>
          <button className="btn secondary" type="button" onClick={onClose}>{tr('cancel')}</button>
        </div>
      </form>
      <style>{`
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
        .tab.active { background: var(--accent-soft); color: var(--accent); border-color: transparent; }
      `}</style>
    </div>
  )
}
