import { FormEvent, useEffect, useState } from 'react'
import { api, type Client, type Inbound } from '../api'
import { useApp } from '../AppContext'

type Props = {
  open: boolean
  clients: Client[]
  inbounds: Inbound[]
  onClose: () => void
  onSaved: () => void
}

export function ClientBulkAttachModal({ open, clients, inbounds, onClose, onSaved }: Props) {
  const { tr } = useApp()
  const [selectedClients, setSelectedClients] = useState<number[]>([])
  const [selectedInbounds, setSelectedInbounds] = useState<number[]>([])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setSelectedClients(clients.map((c) => c.id))
    setSelectedInbounds([])
    setError('')
  }, [open, clients])

  function toggle(list: number[], id: number, set: (v: number[]) => void) {
    set(list.includes(id) ? list.filter((x) => x !== id) : [...list, id])
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (!selectedClients.length) {
      setError('Select at least one client')
      return
    }
    if (!selectedInbounds.length) {
      setError('Select at least one inbound')
      return
    }
    setBusy(true)
    try {
      await api('/clients/bulk-attach', {
        method: 'POST',
        body: JSON.stringify({ ids: selectedClients, inboundIds: selectedInbounds }),
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
      <form className="modal" style={{ width: 'min(640px, 100%)' }} onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <h3>{tr('bulkAttach')}</h3>
        <p className="page-sub">Attach selected clients to one or more inbounds (updates inboundIds).</p>
        <div className="grid2">
          <div className="field">
            <label className="label">Clients ({selectedClients.length}/{clients.length})</label>
            <div style={{ maxHeight: 200, overflow: 'auto', border: '1px solid var(--border)', borderRadius: 8, padding: 8 }}>
              <label style={{ display: 'flex', gap: 8, marginBottom: 6 }}>
                <input
                  type="checkbox"
                  checked={selectedClients.length === clients.length && clients.length > 0}
                  onChange={(e) => setSelectedClients(e.target.checked ? clients.map((c) => c.id) : [])}
                />
                Select all
              </label>
              {clients.map((c) => (
                <label key={c.id} style={{ display: 'flex', gap: 8, cursor: 'pointer' }}>
                  <input type="checkbox" checked={selectedClients.includes(c.id)} onChange={() => toggle(selectedClients, c.id, setSelectedClients)} />
                  {c.email}
                </label>
              ))}
            </div>
          </div>
          <div className="field">
            <label className="label">Inbounds ({selectedInbounds.length})</label>
            <div style={{ maxHeight: 200, overflow: 'auto', border: '1px solid var(--border)', borderRadius: 8, padding: 8 }}>
              {inbounds.map((i) => (
                <label key={i.id} style={{ display: 'flex', gap: 8, cursor: 'pointer' }}>
                  <input type="checkbox" checked={selectedInbounds.includes(i.id)} onChange={() => toggle(selectedInbounds, i.id, setSelectedInbounds)} />
                  #{i.id} {i.remark || i.tag} ({i.protocol}:{i.port})
                </label>
              ))}
            </div>
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
