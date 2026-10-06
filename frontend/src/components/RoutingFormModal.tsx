import { FormEvent, useEffect, useState } from 'react'
import { api, type Inbound, type Outbound } from '../api'
import { useApp } from '../AppContext'
import { appendCsv, csvHas, DOMAIN_CHIPS, IP_CHIPS } from '../lib/routingChips'
import { GeoBrowserModal } from './GeoBrowserModal'

export type RoutingRule = {
  id: number
  remark: string
  enable: boolean
  priority: number
  inboundTag: string
  outboundTag: string
  balancerTag: string
  domain: string
  ip: string
  port: string
  network: string
  protocol: string
}

type Balancer = { tag?: string }

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
  balancerTag: '',
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
  const [balancers, setBalancers] = useState<Balancer[]>([])
  const [geoOpen, setGeoOpen] = useState(false)
  const [geoKind, setGeoKind] = useState<'geosite' | 'geoip'>('geosite')

  useEffect(() => {
    if (!open) return
    setError('')
    api<{ routing?: { balancers?: Balancer[] } }>('/xray/template')
      .then((t) => setBalancers(t.routing?.balancers || []))
      .catch(() => setBalancers([]))
    if (mode === 'edit' && rule) {
      setForm({
        remark: rule.remark || '',
        enable: rule.enable,
        priority: rule.priority,
        inboundTag: rule.inboundTag || '',
        outboundTag: rule.outboundTag || 'direct',
        balancerTag: rule.balancerTag || '',
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
    if (!form.balancerTag && !form.outboundTag) {
      setError('outboundTag or balancerTag required')
      return
    }
    setBusy(true)
    try {
      const payload = {
        ...form,
        outboundTag: form.balancerTag ? (form.outboundTag || '') : form.outboundTag,
      }
      if (mode === 'edit' && rule) {
        await api(`/routing/${rule.id}`, { method: 'PUT', body: JSON.stringify({ ...payload, id: rule.id }) })
      } else {
        await api('/routing', { method: 'POST', body: JSON.stringify(payload) })
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
  const balancerTags = Array.from(new Set(balancers.map((b) => b.tag).filter(Boolean) as string[]))

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
            <label className="label">Outbound tag {form.balancerTag ? '' : '*'}</label>
            <select
              className="select"
              value={form.outboundTag}
              onChange={(e) => setForm({ ...form, outboundTag: e.target.value, balancerTag: '' })}
              required={!form.balancerTag}
              disabled={!!form.balancerTag}
            >
              {outboundTags.map((t) => <option key={t} value={t}>{t}</option>)}
            </select>
          </div>
          <div className="field">
            <label className="label">Balancer tag</label>
            <select
              className="select"
              value={form.balancerTag}
              onChange={(e) => setForm({
                ...form,
                balancerTag: e.target.value,
                outboundTag: e.target.value ? '' : (form.outboundTag || 'direct'),
              })}
            >
              <option value="">— (use outbound)</option>
              {balancerTags.map((t) => <option key={t} value={t}>{t}</option>)}
            </select>
            {balancerTags.length === 0 && (
              <p className="page-sub" style={{ margin: '6px 0 0' }}>Add balancers on Xray page first</p>
            )}
          </div>
          <div className="field" style={{ gridColumn: '1 / -1' }}>
            <label className="label" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
              <span>Domain (csv / geosite:)</span>
              <button
                type="button"
                className="btn secondary"
                style={{ padding: '0.25rem 0.65rem', fontSize: '0.8rem' }}
                onClick={() => { setGeoKind('geosite'); setGeoOpen(true) }}
              >
                {tr('geoBrowser')}
              </button>
            </label>
            <input className="input" value={form.domain} onChange={(e) => setForm({ ...form, domain: e.target.value })} placeholder="geosite:google, domain:example.com" />
            <div className="chip-row">
              {DOMAIN_CHIPS.map((v) => (
                <button
                  key={v}
                  type="button"
                  className={`chip${csvHas(form.domain, v) ? ' active' : ''}`}
                  onClick={() => setForm({ ...form, domain: appendCsv(form.domain, v) })}
                >
                  {v}
                </button>
              ))}
            </div>
          </div>
          <div className="field" style={{ gridColumn: '1 / -1' }}>
            <label className="label" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
              <span>IP (csv / geoip:)</span>
              <button
                type="button"
                className="btn secondary"
                style={{ padding: '0.25rem 0.65rem', fontSize: '0.8rem' }}
                onClick={() => { setGeoKind('geoip'); setGeoOpen(true) }}
              >
                {tr('geoBrowser')}
              </button>
            </label>
            <input className="input" value={form.ip} onChange={(e) => setForm({ ...form, ip: e.target.value })} placeholder="geoip:cn, 1.1.1.1/32" />
            <div className="chip-row">
              {IP_CHIPS.map((v) => (
                <button
                  key={v}
                  type="button"
                  className={`chip${csvHas(form.ip, v) ? ' active' : ''}`}
                  onClick={() => setForm({ ...form, ip: appendCsv(form.ip, v) })}
                >
                  {v}
                </button>
              ))}
            </div>
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
      <GeoBrowserModal
        open={geoOpen}
        kind={geoKind}
        onClose={() => setGeoOpen(false)}
        onPick={(value) => {
          if (geoKind === 'geosite') {
            setForm((f) => ({ ...f, domain: appendCsv(f.domain, value) }))
          } else {
            setForm((f) => ({ ...f, ip: appendCsv(f.ip, value) }))
          }
        }}
      />
    </div>
  )
}
