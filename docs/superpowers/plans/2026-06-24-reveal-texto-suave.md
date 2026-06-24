# Reveal de texto suave no streaming — Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Revelar a resposta da IA caractere a caractere em ritmo contínuo e adaptativo, com fade sutil na borda e markdown ao vivo sem piscar marcadores incompletos.

**Architecture:** Uma reveal engine (função pura `nextShown` + hook `useReveal`) mantém um cursor que avança por `requestAnimationFrame` em ritmo de catch-up exponencial. A mensagem ativa exibe `content.slice(0, shown)`, amaciado por `softenMarkdown` antes do `ReactMarkdown`, com uma máscara CSS na borda enquanto revela. Mensagens passadas e histórico renderizam 100% na hora.

**Tech Stack:** React + TypeScript, `react-markdown` e `motion/react` (já em uso), Tailwind v4, Vitest.

## Global Constraints

- **Sem tokens literais de cor/raio/espaçamento** em componentes — usar utilitários Tailwind/CSS variables já existentes.
- **Núcleo Go intacto** — esta feature é 100% frontend; nenhum arquivo `internal/**` muda.
- **Commits:** Conventional Commits em pt, escopo entre parênteses (o repo roda lefthook com `go test`/`go vet` no pre-commit e commitlint no commit-msg).
- **Constantes ajustáveis:** `BASE_CPS ≈ 80`, `TAU ≈ 0.4` no topo de `reveal.ts`.
- **Testes** seguem o estilo dos `.test.ts` existentes (`import { describe, it, expect } from 'vitest'`).
- **Softener** cobre `**`, `__`, `~~`, `` ` `` e link parcial; deixa `*`/`_` de um caractere e fences de bloco literais de propósito.

---

## Visão de arquivos

**Criados:**
- `frontend/src/lib/reveal.ts` — `nextShown` (pura) + `useReveal` (hook).
- `frontend/src/lib/reveal.test.ts` — testes de `nextShown`.
- `frontend/src/lib/softenMarkdown.ts` — `softenMarkdown` (pura).
- `frontend/src/lib/softenMarkdown.test.ts` — testes de `softenMarkdown`.

**Modificados:**
- `frontend/src/components/MessageCard.tsx` — prop `revealing`, classe `reveal-mask`, markdown amaciado.
- `frontend/src/App.tsx` — fia `useReveal` na mensagem ativa, `slice(0, shown)`, scroll dependente de `shown`, flag `instant` no erro.
- `frontend/src/styles/tokens.css` — classe `.reveal-mask`.

---

## Task 1: `nextShown` — matemática do catch-up (pura)

**Files:**
- Create: `frontend/src/lib/reveal.ts`
- Test: `frontend/src/lib/reveal.test.ts`

**Interfaces:**
- Produces:
  - `export const BASE_CPS = 80`
  - `export const TAU = 0.4`
  - `export function nextShown(shown: number, targetLen: number, dtSec: number, done: boolean): number` — retorna o novo cursor (float) após um frame de `dtSec` segundos. Nunca passa de `targetLen`; faz snap para `targetLen` quando `done` e o backlog é < 1.

- [ ] **Step 1: Escrever o teste que falha**

`frontend/src/lib/reveal.test.ts`:
```ts
import { describe, it, expect } from 'vitest'
import { nextShown, BASE_CPS, TAU } from './reveal'

describe('nextShown', () => {
  it('aplica o piso BASE_CPS quando o backlog é pequeno', () => {
    // backlog 5 → catch-up 5/0.4 = 12.5 c/s < piso 80 c/s
    const next = nextShown(0, 5, 0.016, false)
    expect(next).toBeCloseTo(BASE_CPS * 0.016, 5)
  })

  it('acelera acima do piso quando o backlog é grande', () => {
    // backlog 1000 → catch-up 1000/0.4 = 2500 c/s ≫ piso
    const next = nextShown(0, 1000, 0.016, false)
    expect(next).toBeCloseTo((1000 / TAU) * 0.016, 5)
    expect(next).toBeGreaterThan(BASE_CPS * 0.016)
  })

  it('faz snap para targetLen quando done e o backlog é < 1', () => {
    const next = nextShown(999.8, 1000, 0.001, true)
    expect(next).toBe(1000)
  })

  it('não faz snap se ainda não terminou (done=false), mesmo com backlog < 1', () => {
    const next = nextShown(999.8, 1000, 0.001, false)
    expect(next).toBeGreaterThan(999.8)
    expect(next).toBeLessThan(1000)
  })

  it('nunca ultrapassa targetLen', () => {
    expect(nextShown(995, 1000, 1, false)).toBe(1000)
  })

  it('retorna targetLen se já está em dia', () => {
    expect(nextShown(1000, 1000, 0.016, false)).toBe(1000)
  })
})
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd frontend && npx vitest run src/lib/reveal.test.ts`
Expected: FAIL — `reveal.ts` não exporta `nextShown`.

- [ ] **Step 3: Implementar `nextShown`**

`frontend/src/lib/reveal.ts`:
```ts
// Reveal engine: desacopla o texto que chega (target) do texto exibido (shown).
// Ritmo de catch-up exponencial — um só formato cobre stream e drain.

export const BASE_CPS = 80 // piso de caracteres/segundo durante o stream
export const TAU = 0.4     // constante de tempo (s) da aproximação ao alvo

// nextShown avança o cursor após um frame de dtSec segundos.
export function nextShown(shown: number, targetLen: number, dtSec: number, done: boolean): number {
  if (shown >= targetLen) return targetLen
  const backlog = targetLen - shown
  const step = Math.max(BASE_CPS, backlog / TAU) * dtSec
  const advanced = shown + step
  if (advanced >= targetLen) return targetLen
  if (done && targetLen - advanced < 1) return targetLen
  return advanced
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd frontend && npx vitest run src/lib/reveal.test.ts`
Expected: PASS (6 testes).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/reveal.ts frontend/src/lib/reveal.test.ts
git commit -m "feat(reveal): funcao pura nextShown com catch-up adaptativo"
```

---

## Task 2: `softenMarkdown` — esconder marcadores não-fechados (pura)

**Files:**
- Create: `frontend/src/lib/softenMarkdown.ts`
- Test: `frontend/src/lib/softenMarkdown.test.ts`

**Interfaces:**
- Produces: `export function softenMarkdown(s: string): string` — remove marcadores inline ainda não fechados (`**`, `__`, `~~`, `` ` ``) e a marcação de link parcial (`[texto](url` sem `)`), preservando o texto. Marcadores completos passam intactos. Não toca em `*`/`_` de um caractere nem em fences de bloco.

- [ ] **Step 1: Escrever o teste que falha**

`frontend/src/lib/softenMarkdown.test.ts`:
```ts
import { describe, it, expect } from 'vitest'
import { softenMarkdown } from './softenMarkdown'

describe('softenMarkdown', () => {
  it('esconde ** não-fechado', () => {
    expect(softenMarkdown('isto é **importante')).toBe('isto é importante')
  })

  it('mantém ** fechado intacto', () => {
    expect(softenMarkdown('isto é **importante**')).toBe('isto é **importante**')
  })

  it('esconde __ e ~~ e ` não-fechados', () => {
    expect(softenMarkdown('um __sub')).toBe('um sub')
    expect(softenMarkdown('um ~~ris')).toBe('um ris')
    expect(softenMarkdown('rode `cod')).toBe('rode cod')
  })

  it('mostra só o texto de um link parcial', () => {
    expect(softenMarkdown('veja [docs](http://exemplo')).toBe('veja docs')
  })

  it('mantém link completo intacto', () => {
    expect(softenMarkdown('veja [docs](http://exemplo)')).toBe('veja [docs](http://exemplo)')
  })

  it('deixa * e _ de um caractere e listas inalterados', () => {
    expect(softenMarkdown('* item de lista')).toBe('* item de lista')
    expect(softenMarkdown('a * b')).toBe('a * b')
  })

  it('não altera texto sem marcadores', () => {
    expect(softenMarkdown('texto simples e direto')).toBe('texto simples e direto')
  })
})
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd frontend && npx vitest run src/lib/softenMarkdown.test.ts`
Expected: FAIL — `softenMarkdown.ts` não existe.

- [ ] **Step 3: Implementar `softenMarkdown`**

`frontend/src/lib/softenMarkdown.ts`:
```ts
// Amacia o prefixo visível durante o streaming: esconde marcadores inline
// ainda não fechados para o ReactMarkdown não exibir símbolos parciais.
// Deixa * / _ de um caractere e fences de bloco literais de propósito
// (ver docs/superpowers/specs/2026-06-24-reveal-texto-suave-design.md).

// Remove a última ocorrência ímpar (sem par) do token, preservando o texto.
function hideUnpaired(s: string, tok: string): string {
  const idxs: number[] = []
  let i = s.indexOf(tok)
  while (i !== -1) {
    idxs.push(i)
    i = s.indexOf(tok, i + tok.length)
  }
  if (idxs.length % 2 === 1) {
    const last = idxs[idxs.length - 1]
    return s.slice(0, last) + s.slice(last + tok.length)
  }
  return s
}

export function softenMarkdown(s: string): string {
  let out = s

  // Link parcial: [texto](url  sem ')' de fechamento → mostra só [texto].
  const linkOpen = out.lastIndexOf('](')
  if (linkOpen !== -1 && out.indexOf(')', linkOpen) === -1) {
    const bracket = out.lastIndexOf('[', linkOpen)
    if (bracket !== -1) {
      out = out.slice(0, bracket) + out.slice(bracket + 1, linkOpen)
    }
  }

  // Marcadores de dois caracteres antes dos de um (para ** não casar com *).
  for (const tok of ['**', '__', '~~']) out = hideUnpaired(out, tok)
  // Code inline (um caractere, mas seguro — não conflita com listas).
  out = hideUnpaired(out, '`')

  return out
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd frontend && npx vitest run src/lib/softenMarkdown.test.ts`
Expected: PASS (7 testes).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/softenMarkdown.ts frontend/src/lib/softenMarkdown.test.ts
git commit -m "feat(reveal): softenMarkdown esconde marcadores nao-fechados"
```

---

## Task 3: `useReveal` — hook com loop de animação

**Files:**
- Modify: `frontend/src/lib/reveal.ts`

**Interfaces:**
- Consumes: `nextShown` (Task 1).
- Produces: `export function useReveal(target: string, opts: { done: boolean; resetKey: number }): { shown: number; revealing: boolean }`
  - `shown` é o número inteiro de caracteres a exibir (`Math.floor` do cursor interno).
  - `revealing` é `true` enquanto `!done || shown < target.length`.
  - `resetKey` muda a cada novo envio: quando muda, o cursor volta a 0.

- [ ] **Step 1: Implementar o hook em `reveal.ts`**

Acrescentar ao final de `frontend/src/lib/reveal.ts`:
```ts
import { useEffect, useRef, useState } from 'react'

// useReveal avança um cursor por requestAnimationFrame até alcançar target.length.
// O cursor real é float (acúmulo sub-caractere); expõe Math.floor para exibir.
export function useReveal(
  target: string,
  opts: { done: boolean; resetKey: number },
): { shown: number; revealing: boolean } {
  const { done, resetKey } = opts
  const cursorRef = useRef(0)
  const [shown, setShown] = useState(0)

  // Reset do cursor a cada novo envio.
  useEffect(() => {
    cursorRef.current = 0
    setShown(0)
  }, [resetKey])

  const targetLen = target.length

  useEffect(() => {
    if (cursorRef.current >= targetLen) {
      if (shown !== targetLen) setShown(targetLen)
      return
    }
    let raf = 0
    let prev = performance.now()
    const tick = (now: number) => {
      const dt = Math.min((now - prev) / 1000, 0.05) // clamp p/ abas em background
      prev = now
      cursorRef.current = nextShown(cursorRef.current, targetLen, dt, done)
      const floored = Math.floor(cursorRef.current)
      setShown((s) => (s !== floored ? floored : s))
      if (cursorRef.current < targetLen) raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [targetLen, done, shown])

  return { shown: Math.min(shown, targetLen), revealing: !done || shown < targetLen }
}
```

- [ ] **Step 2: Verificar typecheck**

Run: `cd frontend && npx tsc --noEmit`
Expected: sem erros.

- [ ] **Step 3: Garantir que os testes de `nextShown` seguem passando**

Run: `cd frontend && npx vitest run src/lib/reveal.test.ts`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/reveal.ts
git commit -m "feat(reveal): hook useReveal com loop de requestAnimationFrame"
```

---

## Task 4: Máscara CSS + `MessageCard` com `revealing`

**Files:**
- Modify: `frontend/src/styles/tokens.css`
- Modify: `frontend/src/components/MessageCard.tsx`

**Interfaces:**
- Consumes: `softenMarkdown` (Task 2).
- Produces: `MessageCard` aceita `revealing?: boolean`. Quando `true`, aplica a classe `reveal-mask` no container do conteúdo e passa o conteúdo por `softenMarkdown` antes do `ReactMarkdown`.

- [ ] **Step 1: Adicionar a classe `.reveal-mask` em `frontend/src/styles/tokens.css`**

Acrescentar ao final do arquivo:
```css
/* Fade na borda inferior enquanto o texto é revelado no streaming. */
.reveal-mask {
  -webkit-mask-image: linear-gradient(to bottom, #000 0, #000 calc(100% - 1.1em), transparent);
          mask-image: linear-gradient(to bottom, #000 0, #000 calc(100% - 1.1em), transparent);
}
```

- [ ] **Step 2: Atualizar `MessageCard.tsx`**

Adicionar `revealing` à assinatura e aplicar máscara + softener. O container do conteúdo é o `<div>` que hoje renderiza o `ReactMarkdown` (`MessageCard.tsx:23`).

Trocar a assinatura (linha 4-5):
```tsx
export function MessageCard({ role, content, label, pending, revealing }:
  { role: 'user' | 'assistant' | 'system'; content: string; label: string; pending?: boolean; revealing?: boolean }) {
```

Adicionar o import no topo do arquivo (após o import de `react-markdown`):
```tsx
import { softenMarkdown } from '../lib/softenMarkdown'
```

No `<div>` do conteúdo (linha 23-34), acrescentar a classe condicional `revealing ? 'reveal-mask' : ''` à composição de `className` (concatenar no template string existente).

Trocar a renderização do markdown (linha 45) para amaciar quando estiver revelando:
```tsx
<ReactMarkdown>{revealing ? softenMarkdown(content) : content}</ReactMarkdown>
```

- [ ] **Step 3: Verificar typecheck**

Run: `cd frontend && npx tsc --noEmit`
Expected: sem erros.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/styles/tokens.css frontend/src/components/MessageCard.tsx
git commit -m "feat(reveal): MessageCard com mascara de fade e markdown amaciado"
```

---

## Task 5: Fiação no `App.tsx` + verificação manual

**Files:**
- Modify: `frontend/src/App.tsx`

**Interfaces:**
- Consumes: `useReveal` (Task 3), `MessageCard` com `revealing` (Task 4).
- Produces: a mensagem ativa do assistente revela gradualmente; erro faz snap; scroll acompanha.

- [ ] **Step 1: Importar `useReveal`**

No bloco de imports do topo de `frontend/src/App.tsx`, adicionar:
```tsx
import { useReveal } from './lib/reveal'
```

- [ ] **Step 2: Marcar erros como instantâneos**

No `ChatMsg` (App.tsx:19), adicionar o campo opcional `instant`:
```tsx
type ChatMsg = { role: 'user' | 'assistant' | 'system'; content: string; pending?: boolean; instant?: boolean }
```

No handler `onChatError` (App.tsx:140), marcar a mensagem como `instant` para não revelar o texto de erro:
```tsx
if (last && last.role === 'assistant') copy[i] = { ...last, content: text, pending: false, instant: true }
```

- [ ] **Step 3: Adicionar um `revealKey` resetado a cada envio**

Junto aos outros `useState` (perto de App.tsx:50), adicionar:
```tsx
const [revealKey, setRevealKey] = useState(0)
```

No `send()` (App.tsx:308-309), bumpar a chave ao iniciar um novo envio de chat:
```tsx
setMsgs((m) => [...m, { role: 'user', content: text }, { role: 'assistant', content: '', pending: true }])
setRevealKey((k) => k + 1)
setStreaming(true)
```

- [ ] **Step 4: Calcular o estado de reveal da mensagem ativa**

Antes do `return (` do componente (perto de App.tsx:314, junto de `const bar = analyzeBar(input)`), adicionar:
```tsx
// A última mensagem do assistente (não pendente, não-erro) é a que revela ao vivo.
const lastIdx = msgs.length - 1
const last = msgs[lastIdx]
const activeTarget =
  last && last.role === 'assistant' && !last.pending && !last.instant ? last.content : ''
const { shown, revealing } = useReveal(activeTarget, { done: !streaming, resetKey: revealKey })
```

- [ ] **Step 5: Aplicar o slice e o `revealing` na renderização**

No `.map` das mensagens (App.tsx:444-448), substituir o ramo do `MessageCard` por uma versão que revela só a mensagem ativa:
```tsx
) : (
  <MessageCard key={i} role={m.role}
    content={
      m.pending
        ? tr(lang, 'thinking')
        : i === lastIdx && m.role === 'assistant' && !m.instant
          ? m.content.slice(0, shown)
          : m.content
    }
    pending={m.pending}
    revealing={i === lastIdx && m.role === 'assistant' && !m.pending && !m.instant && revealing}
    label={m.role === 'user' ? tr(lang, 'you') : m.role === 'system' ? tr(lang, 'system') : tr(lang, 'ai')} />
)
```

- [ ] **Step 6: Fazer o scroll acompanhar a revelação**

O efeito de scroll hoje depende de `[msgs]` (App.tsx:161). Trocar a dependência para acompanhar também o cursor:
```tsx
useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [msgs, shown])
```

- [ ] **Step 7: Verificar typecheck e testes**

Run: `cd frontend && npx tsc --noEmit && npx vitest run`
Expected: typecheck sem erros; toda a suíte de testes passa.

- [ ] **Step 8: Verificação manual com o app**

Run: `wails3 dev`
Validar (marcar cada item):
- Enviar uma pergunta: a resposta surge caractere a caractere em ritmo contínuo, não em saltos.
- A borda inferior tem um fade suave enquanto revela e fica nítida ao terminar.
- Negrito/itálico/código não piscam símbolos parciais (`**`, `` ` `` etc.).
- Resposta longa: o reveal acelera para não ficar muito atrás e termina logo após a rede.
- Erro / chave de API ausente: a mensagem de erro aparece de imediato (sem reveal).
- `/new`, duplo-Esc e reabrir a janela resetam corretamente (próxima resposta revela do zero).
- O scroll acompanha o texto sendo revelado.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/App.tsx
git commit -m "feat(reveal): revela a resposta ativa com fade e ritmo adaptativo"
```

---

## Self-review (cobertura do spec)

- **Feel híbrido (char + fade)** → Task 1 (cadência) + Task 4 (máscara). ✓
- **Markdown ao vivo sem piscar marcadores** → Task 2 (softener) + Task 4 (aplicação). ✓
- **Ritmo adaptativo (catch-up)** → Task 1 (`nextShown`) + Task 3 (`useReveal`). ✓
- **Máscara CSS (Abordagem A)** → Task 4. ✓
- **Casos de borda: erro snap, reset, pensando…, scroll** → Task 5 (steps 2, 3, 6) + `pending` preservado. ✓
- **Perf (re-render só quando o inteiro muda)** → Task 3 (`setShown` só quando `floored` muda). ✓
- **TDD nas funções puras** → Tasks 1 e 2. ✓
- **`*`/`_` e fence deixados literais** → Task 2 (escopo). ✓
