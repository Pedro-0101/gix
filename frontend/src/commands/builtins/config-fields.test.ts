import { describe, it, expect } from 'vitest'
import { CONFIG_FIELDS, KEY_OPTIONS, fieldChoices, findField } from './config-fields'

// The JSON keys of the Go Config struct (internal/config/config.go). If a field
// is added there, it must be added here too — this guards that parity.
const CONFIG_KEYS = [
  'theme', 'language', 'open_key', 'open_interval_ms', 'close_key',
  'close_interval_ms', 'model', 'api_key', 'system_prompt', 'opacity',
]

describe('CONFIG_FIELDS', () => {
  it('covers exactly the Config struct keys', () => {
    const keys = CONFIG_FIELDS.map((f) => f.key)
    expect(new Set(keys)).toEqual(new Set(CONFIG_KEYS))
    expect(keys.length).toBe(CONFIG_KEYS.length) // no duplicates
  })
})

describe('enum choices', () => {
  const env = { lang: 'pt', models: ['google/gemini-2.5-flash', 'openai/gpt-4o'] }

  it('theme offers localized light/dark', () => {
    const f = findField('theme')!
    expect(f.kind).toBe('enum')
    if (f.kind !== 'enum') return
    expect(f.choices(env)).toEqual([
      { label: 'Claro', value: 'light' },
      { label: 'Escuro', value: 'dark' },
    ])
  })

  it('model is built from the backend model list', () => {
    const f = findField('model')!
    if (f.kind !== 'enum') return
    expect(f.choices(env).map((c) => c.value)).toEqual(env.models)
  })

  it('hotkey fields use the fixed key options', () => {
    const f = findField('open_key')!
    if (f.kind !== 'enum') return
    expect(f.choices(env).map((c) => c.value)).toEqual(KEY_OPTIONS)
  })
})

describe('number validators', () => {
  it('opacity accepts 0..100 and rejects out-of-range / non-numbers', () => {
    const f = findField('opacity')!
    if (f.kind !== 'number') return
    expect(f.validate('0')).toBeNull()
    expect(f.validate('100')).toBeNull()
    expect(f.validate('-1')).toBe('cfg_invalid_range')
    expect(f.validate('101')).toBe('cfg_invalid_range')
    expect(f.validate('abc')).toBe('cfg_invalid_range')
    expect(f.validate('')).toBe('cfg_invalid_range')
    expect(f.validate('50.5')).toBe('cfg_invalid_range')
  })

  it('intervals require a positive integer', () => {
    const f = findField('open_interval_ms')!
    if (f.kind !== 'number') return
    expect(f.validate('500')).toBeNull()
    expect(f.validate('0')).toBe('cfg_invalid_positive')
    expect(f.validate('-5')).toBe('cfg_invalid_positive')
    expect(f.validate('x')).toBe('cfg_invalid_positive')
  })
})

describe('text fields', () => {
  it('api_key and system_prompt are free text (no validator)', () => {
    expect(findField('api_key')!.kind).toBe('text')
    expect(findField('system_prompt')!.kind).toBe('text')
  })
})

describe('fieldChoices', () => {
  it('lists one localized choice per field, in order', () => {
    const choices = fieldChoices('pt')
    expect(choices.length).toBe(CONFIG_FIELDS.length)
    expect(choices[0]).toEqual({ label: 'Tema', value: 'theme' })
    expect(choices.map((c) => c.value)).toEqual(CONFIG_FIELDS.map((f) => f.key))
  })

  it('localizes labels to English', () => {
    expect(fieldChoices('en')[0]).toEqual({ label: 'Theme', value: 'theme' })
  })
})
