import { FormEvent, useEffect, useState } from 'react'
import { api, type Host, type Inbound } from '../api'
import { useApp } from '../AppContext'

type Props = {
  open: boolean
  mode: 'add' | 'edit'
  host: Host | null
  inbounds: Inbound[]
  onClose: () => void
  onSaved: () => void
}

export function emptyHost(): Host {
  return {
    id: 0,
    enable: true,
    remark: '',
    inboundId: 0,
    inboundTag: '',
    address: '',
    port: 0,
    sni: '',
    hostHeader: '',
    path: '',
    alpn: '',
    fingerprint: '',
    allowInsecure: false,
    sortOrder: 0,
  }
}

export function HostFormModal({ open, mode, host, inbounds, onClose, onSaved }: Props) {
  const { tr } = useApp()
  const [form, setForm] = useState<Host>(emptyHost())
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setError('')
    setForm(host ? { ...host } : emptyHost())
  }, [open, host])

  function set<K extends keyof Host>(key: K, value: Host[K]) {
    setForm((f) => ({ ...f, [key]: value }))
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (!form.address.trim()) {
      setError('Address required')
      return
    }
    setBusy(true)
    try {
      const body = {
        ...form,
        address: form.address.trim(),
        inboundId: Number(form.inboundId) || 0,
        port: Number(form.port) || 0,
        sortOrder: Number(form.sortOrder) || 0,
      }
      if (mode === 'edit' && host) {
        await api(`/hosts/${host.id}`, { method: 'PUT', body: JSON.stringify(body) })
      } else {
        await api('/hosts', { method: 'POST', body: JSON.stringify(body) })
      }
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed')
    } finally {
      setBusy(false)
    }
  }

  if (!open) return null

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <form className="modal" style={{ width: 'min(640px, 100%)' }} onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <h3>{mode === 'edit' ? tr('edit') : tr('create')} {tr('hosts')}</h3>
        <div className="grid2">
          <div className="field">
            <label className="label">{tr('remark')}</label>
            <input className="input" value={form.remark} onChange={(e) => set('remark', e.target.value)} />
          </div>
          <div className="field">
            <label className="label">Address *</label>
            <input className="input" value={form.address} onChange={(e) => set('address', e.target.value)} required placeholder="cdn.example.com" />
          </div>
          <div className="field">
            <label className="label">{tr('port')} (0 = inbound)</label>
            <input className="input" type="number" min={0} max={65535} value={form.port} onChange={(e) => set('port', Number(e.target.value))} />
          </div>
          <div className="field">
            <label className="label">Sort</label>
            <input className="input" type="number" value={form.sortOrder} onChange={(e) => set('sortOrder', Number(e.target.value))} />
          </div>
          <div className="field" style={{ gridColumn: '1 / -1' }}>
            <label className="label">Inbound (0 = all / use tag)</label>
            <select
              className="select"
              value={form.inboundId}
              onChange={(e) => {
                const id = Number(e.target.value)
                const ib = inbounds.find((i) => i.id === id)
                setForm((f) => ({ ...f, inboundId: id, inboundTag: id > 0 ? '' : f.inboundTag, ...(ib ? {} : {}) }))
              }}
            >
              <option value={0}>{tr('allInbounds')}</option>
              {inbounds.map((i) => (
                <option key={i.id} value={i.id}>#{i.id} {i.remark || i.tag} ({i.protocol}:{i.port})</option>
              ))}
            </select>
          </div>
          {form.inboundId === 0 && (
            <div className="field" style={{ gridColumn: '1 / -1' }}>
              <label className="label">Inbound tag (optional match)</label>
              <input className="input" value={form.inboundTag} onChange={(e) => set('inboundTag', e.target.value)} placeholder="inbound-tag" list="host-inbound-tags" />
              <datalist id="host-inbound-tags">
                {inbounds.map((i) => i.tag && <option key={i.id} value={i.tag} />)}
              </datalist>
            </div>
          )}
          <div className="field">
            <label className="label">SNI</label>
            <input className="input" value={form.sni} onChange={(e) => set('sni', e.target.value)} />
          </div>
          <div className="field">
            <label className="label">Host header</label>
            <input className="input" value={form.hostHeader} onChange={(e) => set('hostHeader', e.target.value)} />
          </div>
          <div className="field">
            <label className="label">Path</label>
            <input className="input" value={form.path} onChange={(e) => set('path', e.target.value)} />
          </div>
          <div className="field">
            <label className="label">ALPN</label>
            <input className="input" value={form.alpn} onChange={(e) => set('alpn', e.target.value)} placeholder="h2,http/1.1" />
          </div>
          <div className="field">
            <label className="label">Fingerprint</label>
            <input className="input" value={form.fingerprint} onChange={(e) => set('fingerprint', e.target.value)} placeholder="chrome" />
          </div>
          <div className="field">
            <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <input type="checkbox" checked={form.allowInsecure} onChange={(e) => set('allowInsecure', e.target.checked)} />
              allowInsecure
            </label>
          </div>
          <div className="field">
            <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <input type="checkbox" checked={form.enable} onChange={(e) => set('enable', e.target.checked)} />
              {tr('enable')}
            </label>
          </div>
        </div>
        {error && <p className="error">{error}</p>}
        <div className="row-actions">
          <button className="btn" type="submit" disabled={busy}>{busy ? '…' : tr('save')}</button>
          <button className="btn secondary" type="button" onClick={onClose}>{tr('cancel')}</button>
        </div>
      </form>
    </div>
  )
}
