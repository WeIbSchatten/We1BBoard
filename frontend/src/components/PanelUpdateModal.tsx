import { useEffect, useState } from 'react'
import { api } from '../api'
import { useApp } from '../AppContext'

type Props = {
  open: boolean
  onClose: () => void
}

type UpdateInfo = {
  current: string
  latest: string
  tag?: string
  htmlUrl?: string
}

export function PanelUpdateModal({ open, onClose }: Props) {
  const { tr } = useApp()
  const [info, setInfo] = useState<UpdateInfo | null>(null)
  const [msg, setMsg] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setMsg('')
    api<UpdateInfo>('/server/update-info')
      .then(setInfo)
      .catch((e) => setMsg(e instanceof Error ? e.message : 'error'))
  }, [open])

  if (!open) return null

  async function runUpdate() {
    if (!confirm(tr('panelUpdateConfirm'))) return
    setBusy(true)
    setMsg('')
    try {
      const r = await api<{ message?: string }>('/server/update', { method: 'POST' })
      setMsg(r.message || 'OK')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  const outdated = info && info.latest && info.current !== info.latest
    && !info.latest.startsWith(info.current)
    && info.current !== info.latest.replace(/^v/, '')

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" style={{ width: 'min(480px, 100%)' }} onClick={(e) => e.stopPropagation()}>
        <h3 style={{ marginTop: 0 }}>{tr('panelUpdate')}</h3>
        {info ? (
          <div style={{ display: 'grid', gap: 8 }}>
            <div><span className="page-sub">current</span> <code style={{ fontFamily: 'var(--mono)' }}>{info.current}</code></div>
            <div><span className="page-sub">latest</span> <code style={{ fontFamily: 'var(--mono)' }}>{info.tag || info.latest}</code></div>
            {info.htmlUrl && (
              <a href={info.htmlUrl} target="_blank" rel="noreferrer" style={{ color: 'var(--accent)' }}>
                GitHub release
              </a>
            )}
            {outdated && <p className="page-sub" style={{ color: 'var(--accent)' }}>{tr('updateAvailable')}</p>}
          </div>
        ) : (
          <p className="page-sub">{msg || '…'}</p>
        )}
        <div className="row-actions" style={{ marginTop: 14 }}>
          <button className="btn" type="button" disabled={busy} onClick={() => void runUpdate()}>
            {busy ? '…' : tr('runUpdate')}
          </button>
          <button className="btn secondary" type="button" onClick={onClose}>{tr('cancel')}</button>
        </div>
        {msg && info && <p className="page-sub" style={{ marginTop: 10 }}>{msg}</p>}
      </div>
    </div>
  )
}
