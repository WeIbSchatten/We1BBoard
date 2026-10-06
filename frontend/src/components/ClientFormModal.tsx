import { FormEvent, useEffect, useState } from 'react'
import { api, type Client, type Inbound } from '../api'
import { useApp } from '../AppContext'
import { suggestedFlow, parseInboundToForm } from '../lib/inboundForm'

type Props = {
  open: boolean
  mode: 'add' | 'edit'
  inbound: Inbound
  client: Client | null
  onClose: () => void
  onSaved: () => void
}

export function ClientFormModal({ open, mode, inbound, client, onClose, onSaved }: Props) {
  const { tr } = useApp()
  const [email, setEmail] = useState('')
  const [uuid, setUuid] = useState('')
  const [password, setPassword] = useState('')
  const [flow, setFlow] = useState('')
  const [enable, setEnable] = useState(true)
  const [totalGB, setTotalGB] = useState(0)
  const [expiryDays, setExpiryDays] = useState(0)
  const [comment, setComment] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setError('')
    if (mode === 'edit' && client) {
      setEmail(client.email)
      setUuid(client.uuid)
      setPassword(client.password || '')
      setFlow(client.flow || '')
      setEnable(client.enable)
      setTotalGB(client.totalGB || 0)
      setComment(client.comment || '')
      if (client.expiryTime > 0) {
        const days = Math.max(0, Math.ceil((client.expiryTime - Date.now()) / 86400000))
        setExpiryDays(days)
      } else setExpiryDays(0)
    } else {
      const f = parseInboundToForm(inbound)
      setEmail(`${inbound.protocol}-${Date.now().toString(36)}@we1b`)
      setPassword('')
      setFlow(suggestedFlow(f))
      setEnable(true)
      setTotalGB(0)
      setExpiryDays(0)
      setComment('')
      void api<{ uuid: string }>('/tools/uuid').then((r) => setUuid(r.uuid)).catch(() => {})
    }
  }, [open, mode, client, inbound])

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const body = {
        inboundId: inbound.id,
        email,
        uuid: uuid || undefined,
        password: password || undefined,
        flow,
        enable,
        totalGB,
        comment,
        expiryTime: expiryDays > 0 ? Date.now() + expiryDays * 86400000 : 0,
      }
      if (mode === 'edit' && client) {
        await api(`/clients/${client.id}`, { method: 'PUT', body: JSON.stringify({ ...body, id: client.id, subId: client.subId }) })
      } else {
        await api('/clients', { method: 'POST', body: JSON.stringify(body) })
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
      <form className="modal" style={{ width: 'min(560px, 100%)' }} onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <h3>{mode === 'edit' ? tr('edit') : tr('create')} {tr('clients')}</h3>
        <div className="grid2">
          <div className="field">
            <label className="label">Email</label>
            <input className="input" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </div>
          <div className="field">
            <label className="label">UUID</label>
            <div className="row-actions">
              <input className="input" value={uuid} onChange={(e) => setUuid(e.target.value)} />
              <button type="button" className="btn secondary" onClick={() => api<{ uuid: string }>('/tools/uuid').then((r) => setUuid(r.uuid))}>UUID</button>
            </div>
          </div>
          {(inbound.protocol === 'trojan' || inbound.protocol === 'shadowsocks') && (
            <div className="field">
              <label className="label">{tr('password')}</label>
              <input className="input" value={password} onChange={(e) => setPassword(e.target.value)} />
            </div>
          )}
          {inbound.protocol === 'vless' && (
            <div className="field">
              <label className="label">Flow</label>
              <select className="select" value={flow} onChange={(e) => setFlow(e.target.value)}>
                <option value="">(none)</option>
                <option value="xtls-rprx-vision">xtls-rprx-vision</option>
              </select>
            </div>
          )}
          <div className="field">
            <label className="label">Total GB (0 = ∞)</label>
            <input className="input" type="number" min={0} value={totalGB} onChange={(e) => setTotalGB(Number(e.target.value))} />
          </div>
          <div className="field">
            <label className="label">Expiry days (0 = never)</label>
            <input className="input" type="number" min={0} value={expiryDays} onChange={(e) => setExpiryDays(Number(e.target.value))} />
          </div>
          <div className="field">
            <label className="label">Comment</label>
            <input className="input" value={comment} onChange={(e) => setComment(e.target.value)} />
          </div>
          <div className="field">
            <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <input type="checkbox" checked={enable} onChange={(e) => setEnable(e.target.checked)} />
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
