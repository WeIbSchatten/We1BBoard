import { FormEvent, useEffect, useState } from 'react'
import { api, type Host, type Inbound } from '../api'
import { useApp } from '../AppContext'
import { FormRow } from './FormRow'

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
      <form className="modal modal--md" onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <h3>{mode === 'edit' ? tr('edit') : tr('create')} {tr('hosts')}</h3>

        <div className="modal-body-scroll">
          <FormRow label={tr('remark')}>
            <input className="input" value={form.remark} onChange={(e) => set('remark', e.target.value)} />
          </FormRow>
          <FormRow label="Address *">
            <input className="input" value={form.address} onChange={(e) => set('address', e.target.value)} required placeholder="cdn.example.com" />
          </FormRow>
          <FormRow label={tr('port')} hint="0 = use inbound port">
            <input className="input input-number--compact" type="number" min={0} max={65535} value={form.port} onChange={(e) => set('port', Number(e.target.value))} />
          </FormRow>
          <FormRow label="Sort">
            <input className="input input-number--compact" type="number" value={form.sortOrder} onChange={(e) => set('sortOrder', Number(e.target.value))} />
          </FormRow>
          <FormRow label="Inbound" hint="0 = all / use tag">
            <select
              className="select"
              value={form.inboundId}
              onChange={(e) => {
                const id = Number(e.target.value)
                setForm((f) => ({ ...f, inboundId: id, inboundTag: id > 0 ? '' : f.inboundTag }))
              }}
            >
              <option value={0}>{tr('allInbounds')}</option>
              {inbounds.map((i) => (
                <option key={i.id} value={i.id}>#{i.id} {i.remark || i.tag} ({i.protocol}:{i.port})</option>
              ))}
            </select>
          </FormRow>
          {form.inboundId === 0 && (
            <FormRow label="Inbound tag" hint="Optional match">
              <input className="input" value={form.inboundTag} onChange={(e) => set('inboundTag', e.target.value)} placeholder="inbound-tag" list="host-inbound-tags" />
              <datalist id="host-inbound-tags">
                {inbounds.map((i) => i.tag && <option key={i.id} value={i.tag} />)}
              </datalist>
            </FormRow>
          )}

          <div className="form-section">
            <div className="form-section__title">Stream overrides</div>
            <div className="form-section__body">
              <FormRow label="SNI">
                <input className="input" value={form.sni} onChange={(e) => set('sni', e.target.value)} />
              </FormRow>
              <FormRow label="Host header">
                <input className="input" value={form.hostHeader} onChange={(e) => set('hostHeader', e.target.value)} />
              </FormRow>
              <FormRow label="Path">
                <input className="input" value={form.path} onChange={(e) => set('path', e.target.value)} />
              </FormRow>
              <FormRow label="ALPN">
                <input className="input" value={form.alpn} onChange={(e) => set('alpn', e.target.value)} placeholder="h2,http/1.1" />
              </FormRow>
              <FormRow label="Fingerprint">
                <input className="input" value={form.fingerprint} onChange={(e) => set('fingerprint', e.target.value)} placeholder="chrome" />
              </FormRow>
              <FormRow label="allowInsecure">
                <label className="form-switch">
                  <input type="checkbox" checked={form.allowInsecure} onChange={(e) => set('allowInsecure', e.target.checked)} />
                  <span>{form.allowInsecure ? 'On' : 'Off'}</span>
                </label>
              </FormRow>
            </div>
          </div>

          <FormRow label={tr('enable')}>
            <label className="form-switch">
              <input type="checkbox" checked={form.enable} onChange={(e) => set('enable', e.target.checked)} />
              <span>{form.enable ? 'On' : 'Off'}</span>
            </label>
          </FormRow>
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
