import { FormEvent, useState } from 'react'
import { api, type Inbound } from '../api'
import { useApp } from '../AppContext'
import { parseInboundToForm, suggestedFlow } from '../lib/inboundForm'
import { randomLowerAndNum, randomUUID } from '../lib/random'

type Props = {
  open: boolean
  inbounds: Inbound[]
  onClose: () => void
  onSaved: () => void
}

export function ClientBulkAddModal({ open, inbounds, onClose, onSaved }: Props) {
  const { tr } = useApp()
  const [inboundId, setInboundId] = useState(0)
  const [quantity, setQuantity] = useState(5)
  const [totalGB, setTotalGB] = useState(0)
  const [expiryDays, setExpiryDays] = useState(0)
  const [sharedSub, setSharedSub] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  if (!open) return null

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    const ib = inbounds.find((i) => i.id === inboundId)
    if (!ib) {
      setError('Select inbound')
      return
    }
    const n = Math.min(100, Math.max(1, quantity))
    setBusy(true)
    try {
      const flow = suggestedFlow(parseInboundToForm(ib))
      const shared = sharedSub ? randomLowerAndNum(16) : ''
      for (let i = 0; i < n; i++) {
        await api('/clients', {
          method: 'POST',
          body: JSON.stringify({
            inboundId: ib.id,
            email: randomLowerAndNum(10),
            uuid: randomUUID(),
            password: randomLowerAndNum(16),
            subId: shared || randomLowerAndNum(16),
            flow,
            enable: true,
            totalGB,
            expiryTime: expiryDays > 0 ? Date.now() + expiryDays * 86400000 : 0,
          }),
        })
      }
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <form className="modal" style={{ width: 'min(520px, 100%)' }} onClick={(ev) => ev.stopPropagation()} onSubmit={submit}>
        <h3>{tr('bulkAdd')}</h3>
        <div className="field">
          <label className="label">Inbound</label>
          <select className="select" value={inboundId} onChange={(e) => setInboundId(Number(e.target.value))} required>
            <option value={0}>—</option>
            {inbounds.map((i) => (
              <option key={i.id} value={i.id}>#{i.id} {i.remark || i.tag} ({i.protocol}:{i.port})</option>
            ))}
          </select>
        </div>
        <div className="grid2">
          <div className="field">
            <label className="label">Quantity (1–100)</label>
            <input className="input" type="number" min={1} max={100} value={quantity} onChange={(e) => setQuantity(Number(e.target.value))} />
          </div>
          <div className="field">
            <label className="label">Total GB</label>
            <input className="input" type="number" min={0} value={totalGB} onChange={(e) => setTotalGB(Number(e.target.value))} />
          </div>
          <div className="field">
            <label className="label">Expiry days</label>
            <input className="input" type="number" min={0} value={expiryDays} onChange={(e) => setExpiryDays(Number(e.target.value))} />
          </div>
          <div className="field">
            <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <input type="checkbox" checked={sharedSub} onChange={(e) => setSharedSub(e.target.checked)} />
              Shared subId
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
