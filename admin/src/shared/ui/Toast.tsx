import { useState, useEffect, useCallback } from 'react'
import { setGlobalErrorHandler, ApiError } from '../api/client'

interface ToastMsg {
  id: number
  text: string
  type: 'error' | 'success'
}

let pushToast: (text: string, type: 'error' | 'success') => void = () => {}

export default function Toast() {
  const [toasts, setToasts] = useState<ToastMsg[]>([])
  let id = 0

  const add = useCallback((text: string, type: 'error' | 'success') => {
    const msg = { id: ++id, text, type }
    setToasts((prev) => [...prev, msg])
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== msg.id)), 4000)
  }, [])

  useEffect(() => {
    pushToast = add
    setGlobalErrorHandler((err: ApiError) => add(err.message, 'error'))
  }, [add])

  return (
    <div className="toast-container">
      {toasts.map((t) => (
        <div key={t.id} className={`toast toast-${t.type}`}>{t.text}</div>
      ))}
    </div>
  )
}

export function toast(text: string, type: 'error' | 'success' = 'error') {
  pushToast(text, type)
}
