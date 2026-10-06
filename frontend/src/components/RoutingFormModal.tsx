import { FormEvent, useEffect, useState } from 'react'
import { api, type Inbound, type Outbound } from '../api'
import { useApp } from '../AppContext'

export type RoutingRule = {
  id: number
  remark: string
  enable: boolean
  priority: number
  inboundTag: string
  outboundTag: string
  domain: string
  ip: string
  port: string
  network: string
  protocol: string
}

type Props = {
  open: boolean
  mode: 'add' | 'edit'
  rule: RoutingRule | null
  inbounds: Inbound[]
  outbounds: Outbound[]
  onClose: () => void
  onSaved: () => void
}

const empty = (): Omit<RoutingRule, 'id'> => ({
  remark: '',
  enable: true,
  priority: 100,
  inboundTag: '',
  outboundTag: 'direct',
  domain: '',
  ip: '',
  port: '',
  network: '',
  protocol: '',
})

export function RoutingFormModal({ open, mode, rule, inbounds, outbounds, onClose, onSaved }: Props) {
  const { tr } = useApp()
  const [form, setForm] = useState(empty())
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setError('')
    if (mode === 'edit' && rule) {
      setForm({
        remark: rule.remark || '',
        enable: rule.enable,
        priority: rule.priority,
        inboundTag: rule.inboundTag || '',
        outboundTag: rule.outboundTag || 'direct',
        domain: rule.domain || '',
        ip: rule.ip || '',
        port: rule.port || '',
        network: rule.network || '',
        protocol: rule.protocol || '',
      })
    } else {
      setForm(empty())
    }
  }, [open, mode, rule])

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      if (mode === 'edit' && rule) {
        await api(`/routing/${rule.id}`, { method: 'PUT', body: JSON.stringify({ ...form, id: rule.id }) })
      } else {
        await api('/routing', { method: 'POST', body: JSON.stringify(form) })
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

  const inboundTags = Array.from(new Set(inbounds.map((i) => i.tag).filter(Boolean)))
  const outboundTags = Array.from(new Set([
    ...outbounds.map((o) => o.tag),
    'direct', 'blocked', 'api',
  ].filter(Boolean)))

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <form className="modal" style={{ width: 'min(680px, 100%)' }} onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <h3>{mode === 'edit' ? tr('edit') : tr('create')} {tr('routing')}</h3>
        <div className="grid2">
          <div className="field">
            <label className="label">{tr('remark')}</label>
            <input className="input" value={form.remark} onChange={(e) => setForm({ ...form, remark: e.target.value })} />
          </div>
          <div className="field">
            <label className="label">Priority</label>
            <input className="input" type="number" value={form.priority} onChange={(e) => setForm({ ...form, priority: Number(e.target.value) })} />
          </div>
          <div className="field">
            <label className="label">Inbound tag</label>
            <select className="select" value={form.inboundTag} onChange={(e) => setForm({ ...form, inboundTag: e.target.value })}>
              <option value="">* (any)</option>
              {inboundTags.map((t) => <option key={t} value={t}>{t}</option>)}
            </select>
            <input className="input" style={{ marginTop: 6 }} placeholder="or type custom / csv" value={form.inboundTag} onChange={(e) => setForm({ ...form, inboundTag: e.target.value })} />
          </div>
          <div className="field">
            <label className="label">Outbound tag *</label>
            <select className="select" value={form.outboundTag} onChange={(e) => setForm({ ...form, outboundTag: e.target.value })} required>
              {outboundTags.map((t) => <option key={t} value={t}>{t}</option>)}
            </select>
          </div>
          <div className="field">
            <label className="label">Domain (csv / geosite:)</label>
            <input className="input" value={form.domain} onChange={(e) => setForm({ ...form, domain: e.target.value })} placeholder="geosite:google, domain:example.com" />
          </div>
          <div className="field">
            <label className="label">IP (csv / geoip:)</label>
            <input className="input" value={form.ip} onChange={(e) => setForm({ ...form, ip: e.target.value })} placeholder="geoip:cn, 1.1.1.1/32" />
          </div>
          <div className="field">
            <label className="label">Port</label>
            <input className="input" value={form.port} onChange={(e) => setForm({ ...form, port: e.target.value })} placeholder="80,443,1000-2000" />
          </div>
          <div className="field">
            <label className="label">Network</label>
            <select className="select" value={form.network} onChange={(e) => setForm({ ...form, network: e.target.value })}>
              <option value="">any</option>
              <option value="tcp">tcp</option>
              <option value="udp">udp</option>
              <option value="tcp,udp">tcp,udp</option>
            </select>
          </div>
          <div className="field">
            <label className="label">L7 protocol (csv)</label>
            <input className="input" value={form.protocol} onChange={(e) => setForm({ ...form, protocol: e.target.value })} placeholder="http,tls,bittorrent,quic" />
          </div>
          <div className="field">
            <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <input type="checkbox" checked={form.enable} onChange={(e) => setForm({ ...form, enable: e.target.checked })} />
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
