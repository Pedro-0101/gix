import { tr } from '../i18n'

type Rule = { freq?: string; interval?: number; weekday?: string; time?: string }

const FREQ_KEY: Record<string, string> = {
  daily: 'alert_recurrence_daily',
  weekly: 'alert_recurrence_weekly',
  monthly: 'alert_recurrence_monthly',
  yearly: 'alert_recurrence_yearly',
}

// recurrenceLabel turns a stored recurrence JSON string into a short, localized
// label. Empty/invalid input (one-shot alerts) yields ''.
export function recurrenceLabel(lang: string, recurrenceJSON: string): string {
  if (!recurrenceJSON) return ''
  let rule: Rule
  try {
    rule = JSON.parse(recurrenceJSON)
  } catch {
    return ''
  }
  const key = rule.freq ? FREQ_KEY[rule.freq] : undefined
  if (!key) return ''
  const base = tr(lang, key)
  const n = rule.interval ?? 1
  if (n > 1) return `${tr(lang, 'alert_recurrence_every')} ${n} ${base}`
  return base
}

// formatFireAt renders a UTC ISO timestamp as a local date+time. Invalid input
// yields '' so callers can skip rendering.
export function formatFireAt(iso: string, lang: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString(lang === 'en' ? 'en-US' : 'pt-BR', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}
