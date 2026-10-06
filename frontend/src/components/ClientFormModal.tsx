import { FormEvent, useEffect, useState } from 'react'
import { api, type Client, type Inbound } from '../api'
import { useApp } from '../AppContext'
import { parseInboundToForm, suggestedFlow } from '../lib/inboundForm'
import { randomLowerAndNum, randomUUID } from '../lib/random'

type Props = {
  open: boolean
  mode: 'add' | 'edit'
  inbound: Inbound | null
  inbounds?: Inbound[]
  client: Client | null
  onClose: () => void
  onSaved: () => void
}

type Tab = 'basic' | 'config'

export function ClientFormModal({ open, mode, inbound, inbounds, client, onClose, onSaved }: Props) {
  const { tr } = useApp()
  const [tab, setTab] = useState<Tab>('basic')
  const [inboundId, setInboundId] = useState(0)
  const [email, setEmail] = useState('')
  const [uuid, setUuid] = useState('')
  const [password, setPassword] = useState('')
  const [subId, setSubId] = useState('')
  const [flow, setFlow] = useState('')
  const [enable, setEnable] = useState(true)
  const [totalGB, setTotalGB] = useState(0)
  const [expiryDays, setExpiryDays] = useState(0)
  const [tgId, setTgId] = useState(0)
  const [comment, setComment] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const selectedInbound = inbound || inbounds?.find((i) => i.id === inboundId) || null
  const protocol = selectedInbound?.protocol || ''

  useEffect(() => {
    if (!open) return
    setTab('basic')
    setError('')
    if (mode === 'edit' && client) {
      setInboundId(client.inboundId)
      setEmail(client.email)
      setUuid(client.uuid)
      setPassword(client.password || '')
      setSubId(client.subId || '')
      setFlow(client.flow || '')
      setEnable(client.enable)
      setTotalGB(client.totalGB || 0)
      setComment(client.comment || '')
      setTgId(client.tgId || 0)
      if (client.expiryTime > 0) {
        setExpiryDays(Math.max(0, Math.ceil((client.expiryTime - Date.now()) / 86400000)))
      } else setExpiryDays(0)
    } else {
      const ib = inbound || inbounds?.[0] || null
      setInboundId(ib?.id || 0)
      setEmail(randomLowerAndNum(10))
      setUuid(randomUUID())
      setPassword(randomLowerAndNum(16))
      setSubId(randomLowerAndNum(16))
      setEnable(true)
      setTotalGB(0)
      setExpiryDays(0)
      setComment('')
      setTgId(0)
      if (ib) {
        const f = parseInboundToForm(ib)
        setFlow(suggestedFlow(f))
      } else {
        setFlow('')
      }
    }
  }, [open, mode, client, inbound, inbounds])

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    const id = inbound?.id || inboundId
    if (!id) {
      setError('Select inbound')
      return
    }
    setBusy(true)
    try {
      const body = {
        inboundId: id,
        email,
        uuid: uuid || undefined,
        password: password || undefined,
        subId: subId || undefined,
        flow,
        enable,
        totalGB,
        comment,
        tgId,
        expiryTime: expiryDays > 0 ? Date.now() + expiryDays * 86400000 : 0,
      }
      if (mode === 'edit' && client) {
        await api(`/clients/${client.id}`, { method: 'PUT', body: JSON.stringify({ ...body, id: client.id }) })
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
      <form className="modal" style={{ width: 'min(640px, 100%)' }} onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <h3>{mode === 'edit' ? tr('edit') : tr('create')} {tr('clients')}</h3>
        <div className="tabs" style={{ display: 'flex', gap: 6, marginBottom: 12 }}>
          <button type="button" className={`tab ${tab === 'basic' ? 'active' : ''}`} onClick={() => setTab('basic')}>{tr('tabGeneral')}</button>
          <button type="button" className={`tab ${tab === 'config' ? 'active' : ''}`} onClick={() => setTab('config')}>{tr('tabConfig')}</button>
        </div>

        {tab === 'basic' && (
          <div className="grid2">
            {!inbound && (
              <div className="field" style={{ gridColumn: '1 / -1' }}>
                <label className="label">Inbound</label>
                <select className="select" value={inboundId} onChange={(e) => {
                  const id = Number(e.target.value)
                  setInboundId(id)
                  const ib = inbounds?.find((i) => i.id === id)
                  if (ib) setFlow(suggestedFlow(parseInboundToForm(ib)))
                }} required>
                  <option value={0}>—</option>
                  {(inbounds || []).map((i) => (
                    <option key={i.id} value={i.id}>#{i.id} {i.remark || i.tag} ({i.protocol}:{i.port})</option>
                  ))}
                </select>
              </div>
            )}
            <div className="field">
              <label className="label">Email</label>
              <div className="row-actions">
                <input className="input" value={email} onChange={(e) => setEmail(e.target.value)} required />
                <button type="button" className="btn secondary" onClick={() => setEmail(randomLowerAndNum(10))}>↻</button>
              </div>
            </div>
            <div className="field">
              <label className="label">Total GB (0 = ∞)</label>
              <input className="input" type="number" min={0} value={totalGB} onChange={(e) => setTotalGB(Number(e.target.value))} />
            </div>
            <div className="field">
              <label className="label">Expiry days (0 = never)</label>
              <input className="input" type="number" min={0} value={expiryDays} onChange={(e) => setExpiryDays(Number(e.target.value))} />
            </div>
            <div className="field">
              <label className="label">Telegram ID</label>
              <input className="input" type="number" value={tgId} onChange={(e) => setTgId(Number(e.target.value))} />
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
        )}

        {tab === 'config' && (
          <div className="grid2">
            <div className="field">
              <label className="label">UUID</label>
              <div className="row-actions">
                <input className="input" value={uuid} onChange={(e) => setUuid(e.target.value)} />
                <button type="button" className="btn secondary" onClick={() => setUuid(randomUUID())}>↻</button>
              </div>
            </div>
            <div className="field">
              <label className="label">Sub ID</label>
              <div className="row-actions">
                <input className="input" value={subId} onChange={(e) => setSubId(e.target.value)} />
                <button type="button" className="btn secondary" onClick={() => setSubId(randomLowerAndNum(16))}>↻</button>
              </div>
            </div>
            {(protocol === 'trojan' || protocol === 'shadowsocks' || !protocol) && (
              <div className="field">
                <label className="label">{tr('password')}</label>
                <div className="row-actions">
                  <input className="input" value={password} onChange={(e) => setPassword(e.target.value)} />
                  <button type="button" className="btn secondary" onClick={() => setPassword(randomLowerAndNum(16))}>↻</button>
                </div>
              </div>
            )}
            {(protocol === 'vless' || !protocol) && (
              <div className="field">
                <label className="label">Flow</label>
                <select className="select" value={flow} onChange={(e) => setFlow(e.target.value)}>
                  <option value="">(none)</option>
                  <option value="xtls-rprx-vision">xtls-rprx-vision</option>
                  <option value="xtls-rprx-vision-udp443">xtls-rprx-vision-udp443</option>
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
        .tab.active {
          background: var(--accent-soft);
          color: var(--accent);
          border-color: transparent;
        }
      `}</style>
    </div>
  )
}
