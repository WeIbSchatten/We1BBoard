import { useRef, useState } from 'react'
import { API_BASE } from '../api'
import { useApp } from '../AppContext'

type Props = {
  open: boolean
  onClose: () => void
}

type RestoreResult = {
  path?: string
  bytes?: number
  willExit?: boolean
  message?: string
}

export function BackupModal({ open, onClose }: Props) {
  const { tr } = useApp()
  const inputRef = useRef<HTMLInputElement>(null)
  const [msg, setMsg] = useState('')
  const [busy, setBusy] = useState(false)

  if (!open) return null

  async function downloadBackup() {
    setMsg('')
    setBusy(true)
    try {
      const res = await fetch(API_BASE + '/server/backup', { credentials: 'include' })
      if (!res.ok) {
        const data = await res.json().catch(() => null)
        throw new Error(data?.error || res.statusText)
      }
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'we1bboard-backup.db'
      a.click()
      URL.revokeObjectURL(url)
      setMsg('OK')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  async function onFile(file: File | null) {
    if (!file) return
    if (!confirm(tr('restoreConfirm'))) return
    setMsg('')
    setBusy(true)
    try {
      const fd = new FormData()
      fd.append('file', file)
      const res = await fetch(API_BASE + '/server/restore', {
        method: 'POST',
        credentials: 'include',
        body: fd,
      })
      const data = await res.json()
      if (!data.success) throw new Error(data.error || res.statusText)
      const r = data.data as RestoreResult
      setMsg(r.message || 'OK')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
      if (inputRef.current) inputRef.current.value = ''
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" style={{ width: 'min(480px, 100%)' }} onClick={(e) => e.stopPropagation()}>
        <h3 style={{ marginTop: 0 }}>{tr('backupRestore')}</h3>
        <p className="page-sub" style={{ marginTop: 0 }}>
          SQLite only. Postgres: use pg_dump / pg_restore.
        </p>
        <div className="row-actions" style={{ marginTop: 12 }}>
          <button className="btn" type="button" disabled={busy} onClick={() => void downloadBackup()}>
            {tr('downloadBackup')}
          </button>
          <button className="btn secondary" type="button" disabled={busy} onClick={() => inputRef.current?.click()}>
            {tr('uploadRestore')}
          </button>
          <input
            ref={inputRef}
            type="file"
            accept=".db,application/octet-stream"
            style={{ display: 'none' }}
            onChange={(e) => void onFile(e.target.files?.[0] || null)}
          />
        </div>
        {msg && <p className="page-sub" style={{ marginTop: 12 }}>{msg}</p>}
        <div className="row-actions" style={{ marginTop: 16, justifyContent: 'flex-end' }}>
          <button className="btn secondary" type="button" onClick={onClose}>{tr('cancel')}</button>
        </div>
      </div>
    </div>
  )
}
