import { useApp } from '../AppContext'

type Props = {
  open: boolean
  title: string
  message: string
  confirmLabel?: string
  danger?: boolean
  onConfirm: () => void
  onCancel: () => void
}

export function ConfirmModal({
  open,
  title,
  message,
  confirmLabel,
  danger,
  onConfirm,
  onCancel,
}: Props) {
  const { tr } = useApp()
  if (!open) return null
  return (
    <div className="modal-backdrop" onClick={onCancel}>
      <div
        className="modal"
        style={{ width: 'min(420px, 100%)' }}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirm-title"
      >
        <h3 id="confirm-title">{title}</h3>
        <p className="page-sub" style={{ margin: 0 }}>{message}</p>
        <div className="modal-footer">
          <button type="button" className="btn secondary btn-sm" onClick={onCancel}>
            {tr('cancel')}
          </button>
          <button
            type="button"
            className={`btn btn-sm${danger ? ' danger' : ''}`}
            onClick={onConfirm}
            autoFocus
          >
            {confirmLabel || tr('delete')}
          </button>
        </div>
      </div>
    </div>
  )
}
