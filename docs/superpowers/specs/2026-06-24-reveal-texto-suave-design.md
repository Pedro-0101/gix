# Reveal de texto suave no streaming — Design

**Goal:** Desacoplar o texto que chega da rede (em rajadas) do texto exibido, revelando a resposta da IA caractere a caractere em ritmo contínuo, com um fade sutil na borda e markdown ao vivo sem piscar símbolos incompletos. Resolve o item #10 do `docs/todo` ("Fazer mecanica para letras e palavras aparecerem aos poucos suavemente de forma continua").

**Architecture:** Uma reveal engine (`useReveal` + função pura `nextShown`) mantém um cursor que avança por `requestAnimationFrame` em ritmo adaptativo (catch-up exponencial). A mensagem ativa exibe `content.slice(0, shown)`; esse prefixo passa por um softener de markdown (`softenMarkdown`) antes do `ReactMarkdown`, e recebe uma máscara CSS na borda enquanto está revelando. Mensagens passadas e histórico renderizam 100% imediatamente.

**Tech Stack:** React + TypeScript, `motion/react` (já em uso), `react-markdown` (já em uso), Tailwind v4, Vitest (mesma infra dos `.test.ts` existentes).

## Decisões de design (confirmadas)

1. **Feel híbrido:** avanço caractere a caractere em ritmo constante, com fade sutil na borda mais recente — nunca "pula".
2. **Markdown ao vivo escondendo marcadores não-fechados:** o prefixo visível é amaciado para não exibir símbolos parciais (`**bo` não aparece como literal).
3. **Ritmo adaptativo (catch-up):** velocidade-base confortável que acelera quando o buffer acumula e drena rápido ao terminar; nunca fica muito atrás da rede.
4. **Fade via máscara CSS (Abordagem A):** gradiente de máscara na borda da bolha durante o stream — robusto, barato, sem fatiar markdown. O fade é geométrico (última ~1 linha), não estritamente "últimos N caracteres".

## Componentes

### `frontend/src/lib/reveal.ts`

- `nextShown(shown: number, targetLen: number, dtSec: number, done: boolean): number` — **função pura** com a matemática do catch-up. Sem timers, totalmente testável.
- `useReveal(target: string, opts: { done: boolean }): { shown: number; revealing: boolean }` — hook que roda um loop `requestAnimationFrame` aplicando `nextShown` a cada frame enquanto `shown < target.length`. Reinicia o cursor quando o `target` corresponde a um novo envio (chave de reset).

### `frontend/src/lib/softenMarkdown.ts`

- `softenMarkdown(prefix: string): string` — função pura que esconde marcadores inline ainda não fechados antes de passar ao `ReactMarkdown`. Cobre: `**`, `__`, `~~`, `` ` `` (inline) e link parcial (`[texto](url` sem `)` de fechamento). Marcadores completos passam intactos.

  **Fora de escopo no softener:** ênfase de um caractere (`*`, `_`) é deixada literal de propósito — escondê-la quebraria listas com marcador (`* item`) e underscores no meio de palavras durante o stream. O flicker de itálico parcial é raro e menos incômodo que regredir esses casos. Fence de bloco (` ``` `) também fica literal: o `ReactMarkdown` renderiza um fence aberto como bloco de código em progresso, o que é aceitável.

### `frontend/src/components/MessageCard.tsx` (modificado)

- Aceita prop `revealing?: boolean`. Quando `true`, aplica a classe `reveal-mask` no container do conteúdo.
- O conteúdo passa por `softenMarkdown` antes do `ReactMarkdown` quando a mensagem é do assistente e está revelando (mensagens de sistema/usuário e finalizadas não precisam, mas aplicar o softener num texto completo é no-op, então pode ser sempre aplicado em conteúdo de assistente).

### `frontend/src/App.tsx` (modificado)

- Identifica a mensagem ativa (último assistente enquanto `streaming` ou enquanto o reveal não terminou de drenar).
- Chama `useReveal(activeContent, { done: !streaming })`.
- Renderiza a mensagem ativa com `content = activeContent.slice(0, shown)` e `revealing`. Demais mensagens com `content` completo e `revealing={false}`.
- O efeito de `scrollIntoView` passa a depender também de `shown` para acompanhar a revelação.

### CSS

Classe `reveal-mask` (em `frontend/src/styles/tokens.css` ou equivalente já importado):

```css
.reveal-mask {
  -webkit-mask-image: linear-gradient(to bottom, #000 0, #000 calc(100% - 1.1em), transparent);
          mask-image: linear-gradient(to bottom, #000 0, #000 calc(100% - 1.1em), transparent);
}
```

## Fluxo de dados

```
delta SSE ──► msgs[ativa].content  (target: tudo que chegou)
                     │
            useReveal(target, done) ──► shown (cursor, avança por rAF)
                     │
        target.slice(0, shown) ──► softenMarkdown ──► ReactMarkdown
                     │
              revealing? ──► classe .reveal-mask (gradiente na borda)
```

`onChatDelta` continua acumulando em `content` (o *target*). `onChatDone` define o `content` final (drena até o fim). A diferença é que a mensagem ativa exibe `content.slice(0, shown)` em vez de `content` cru.

## Algoritmo do reveal (catch-up adaptativo)

Aproximação exponencial — um único formato cobre stream e drain:

```
backlog = targetLen - shown
step    = max(BASE_CPS, backlog / TAU) * dtSec     // chars neste frame
shown   = min(targetLen, shown + step)
se done e (targetLen - shown) < 1 → snap = targetLen
```

- `BASE_CPS` ≈ 80 caracteres/segundo — piso confortável durante o stream.
- `TAU` ≈ 0.4 s — constante de tempo da aproximação; `backlog / TAU` acelera quando o buffer acumula e drena naturalmente no `done`.
- Ambos são constantes ajustáveis no topo de `reveal.ts`.
- `revealing = streaming || shown < targetLen` controla a máscara.

`nextShown` trabalha com `shown` em ponto flutuante internamente (acúmulo sub-caractere por frame); o componente exibe `Math.floor(shown)` caracteres.

## Casos de borda

- **Erro / `no_api_key`:** snap imediato — texto de sistema/erro não passa pelo reveal (`revealing={false}`).
- **`/new`, cancelar (duplo-Esc → janela esconde), nova janela (`onWindowShown`):** o estado de `msgs` é resetado e o reveal reinicia com `shown = 0` no próximo envio (chave de reset por envio).
- **"pensando…":** mantém o comportamento atual (dots) enquanto `pending`; o reveal começa quando o primeiro caractere é revelado.
- **Scroll:** `scrollIntoView` passa a depender de `shown` para acompanhar a revelação linha a linha.

## Perf

Re-parse do `ReactMarkdown` a cada frame pode pesar em respostas longas (o `rAF` dispara ~60 fps). Mitigação **se necessário** (não pré-otimizar): throttle do re-parse a cada ~2 frames ou só quando `Math.floor(shown)` muda. O `useReveal` já evita re-render quando o número inteiro de caracteres exibidos não mudou entre frames.

## Testes (TDD)

Seguindo o estilo dos `.test.ts` existentes (`highlight.test.ts`, `promptHistory.test.ts`):

- **`frontend/src/lib/reveal.test.ts`** → `nextShown`:
  - aplica o piso `BASE_CPS` quando o backlog é pequeno;
  - acelera (passo > piso) quando o backlog é grande;
  - faz snap para `targetLen` quando `done` e o backlog é < 1;
  - nunca ultrapassa `targetLen`.
- **`frontend/src/lib/softenMarkdown.test.ts`** → tabela:
  - `**` solto → escondido; `**bold**` completo → intacto;
  - `__`, `~~`, `` ` `` não-fechados → escondidos;
  - link parcial `[x](url` → exibe só o texto `x` até fechar `)`; link completo `[x](url)` → intacto;
  - ênfase de um caractere (`*`, `_`) e listas com marcador → inalteradas;
  - texto sem marcadores → inalterado.

## Arquivos

**Criados:**
- `frontend/src/lib/reveal.ts`
- `frontend/src/lib/reveal.test.ts`
- `frontend/src/lib/softenMarkdown.ts`
- `frontend/src/lib/softenMarkdown.test.ts`

**Modificados:**
- `frontend/src/components/MessageCard.tsx` — prop `revealing`, classe de máscara, markdown amaciado.
- `frontend/src/App.tsx` — fia `useReveal` na mensagem ativa + scroll dependente de `shown`.
- `frontend/src/styles/tokens.css` (ou o CSS já importado) — classe `.reveal-mask`.

## Fora de escopo (YAGNI)

- Fade preciso por caractere/palavra (Abordagem B) — descartado por fragilidade.
- Som/haptics, configuração de velocidade pela UI (constantes ajustáveis no código bastam por ora).
- Reveal em mensagens do histórico — só a resposta ao vivo revela.
