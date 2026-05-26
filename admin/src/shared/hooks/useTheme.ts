import { useState, useEffect } from 'react'

const STORAGE_KEY = 'groupbot-theme'

function getInitialTheme(): boolean {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored !== null) return stored === 'dark'
  return true
}

export function useTheme() {
  const [dark, setDark] = useState(getInitialTheme)

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', dark ? '' : 'light')
    localStorage.setItem(STORAGE_KEY, dark ? 'dark' : 'light')
  }, [dark])

  const toggleTheme = () => setDark(prev => !prev)

  return { dark, toggleTheme }
}
