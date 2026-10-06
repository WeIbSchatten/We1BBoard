import { FormEvent, useEffect, useState } from 'react'
import { api, type Client } from '../api'
import { useApp } from '../AppContext'

type Props = {
  open: boolean
  clients: Client[]
  onClose: () => void
  onSaved: () => void
}

export function ClientBulkAdjustModal({ open, clients, onClose, onSaved }: Props) {
  const { tr } = useApp()
  const [selected, setSelected] = useState<number[]>([])
  const [addDays, setAddDays] = useState(0)
  const [addGB, setAddGB] = useState(0)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setSelected(clients.map((c) => c.id))
    setAddDays(0)
    setAddGB(0)
    setError('')
  }, [open, clients])

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (!selected.length) {
      setError('Select at least one client')
      return
    }
    if (!addDays && !addGB) {
      setError('Set addDays and/or addGB')
      return
    }
    setBusy(true)
    try {
      await api('/clients/bulk-adjust', {
        method: 'POST',
        body: JSON.stringify({ ids: selected, addDays, addGB }),
      })
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
      <form className="modal" style={{ width: 'min(520px, 100%)' }} onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <h3>{tr('bulkAdjust')}</h3>
        <p className="page-sub">Apply to filtered / selected clients: add days to expiry, add GB to total.</p>
        <div className="field">
          <label className="label">Clients ({selected.length}/{clients.length})</label>
          <div style={{ maxHeight: 180, overflow: 'auto', border: '1px solid var(--border)', borderRadius: 8, padding: 8 }}>
            <label style={{ display: 'flex', gap: 8, marginBottom: 6 }}>
              <input
                type="checkbox"
                checked={selected.length === clients.length && clients.length > 0}
                onChange={(e) => setSelected(e.target.checked ? clients.map((c) => c.id) : [])}
              />
              Select all filtered
            </label>
            {clients.map((c) => (
              <label key={c.id} style={{ display: 'flex', gap: 8, cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={selected.includes(c.id)}
                  onChange={() => setSelected((prev) => prev.includes(c.id) ? prev.filter((x) => x !== c.id) : [...prev, c.id])}
                />
                {c.email}
              </label>
            ))}
          </div>
        </div>
        <div className="grid2">
          <div className="field">
            <label className="label">Add days</label>
            <input className="input" type="number" value={addDays} onChange={(e) => setAddDays(Number(e.target.value))} />
          </div>
          <div className="field">
            <label className="label">Add GB</label>
            <input className="input" type="number" value={addGB} onChange={(e) => setAddGB(Number(e.target.value))} />
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
