import type { ReactNode } from 'react'

export function FormRow({
  label,
  hint,
  children,
  stack,
}: {
  label?: string
  hint?: string
  children: ReactNode
  stack?: boolean
}) {
  if (stack || !label) {
    return (
      <div className="form-row--stack">
        {label && <label className="form-row__label">{label}</label>}
        <div className="form-row__control">
          {children}
          {hint && <div className="field-hint">{hint}</div>}
        </div>
      </div>
    )
  }
  return (
    <div className="form-row">
      <label className="form-row__label">{label}</label>
      <div className="form-row__control">
        {children}
        {hint && <div className="field-hint">{hint}</div>}
      </div>
    </div>
  )
}
