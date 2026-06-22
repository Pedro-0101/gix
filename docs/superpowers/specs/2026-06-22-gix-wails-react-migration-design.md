# Migração da UI do gix para Wails v3 + React

**Data:** 2026-06-22
**Status:** Aprovado para planejamento

## Contexto e motivação

O gix é um overlay de chat estilo Spotlight/Raycast escrito em Go, hoje com UI em
**Fyne**. A camada visual é quase toda hard-coded e o Fyne é propositalmente
rígido: não faz CSS, markdown rico, animações fluidas, gradientes ou sombras, e
limita layouts livres.

O objetivo desta migração é destravar a flexibilidade visual:

- **Capacidades que o Fyne não tem** — animações, gradientes, sombras, markdown
  renderizado, layouts livres.
- **Mais controle no código** — estilos centralizados e fáceis de alterar.
- **Layout/estrutura da janela** — liberdade pra dispor os elementos.

Não é objetivo (por ora) entregar temas customizáveis pro usuário final, embora a
arquitetura escolhida deixe esse caminho aberto.

## Decisão de arquitetura

Migrar a camada de apresentação para **Wails v3** (backend Go + frontend web),
com frontend em **React**. O Wails v3 (alpha) foi escolhido sobre o v2 porque tem,
de fábrica, exatamente o formato deste app: system tray nativo, janela frameless
always-on-top, janela escondida no boot e eventos Go→frontend. O risco assumido é
a instabilidade de API por ser alpha.

### O que é reaproveitado (sem reescrever)

- `internal/ai` — cliente e streaming, 100% reutilizável.
- `internal/config` — load/save, reutilizável (estendido com aparência mais tarde).
- `internal/db` — histórico SQLite, reutilizável.
- O hook global de teclado (`internal/ui/hotkey*.go`) — é puro syscall, não depende
  do Fyne; será movido para `internal/hotkey`.

### O que é substituído

Toda a UI Fyne: `window.go`, `card.go`, `button.go`, `settings.go`, `history.go`,
`theme.go`, `command.go`. Os truques nativos de janela (`removeTitleBar`,
`centerWindow` via `user32.dll`) deixam de ser necessários — o Wails v3 entrega
janela frameless e `window.Center()` nativos.

## Estrutura do repositório alvo

```
cmd/gix/main.go         → bootstrap do app Wails v3 (substitui ui.Run)
internal/ai/            → INTACTO
internal/config/        → INTACTO (estendido com aparência depois)
internal/db/            → INTACTO
internal/hotkey/        → hook global movido de internal/ui (puro syscall)
internal/app/           → NOVO: serviços Wails + wiring de janela/tray/hotkey
  ├─ chat.go            → ChatService (streaming via eventos)
  ├─ config.go          → ConfigService
  ├─ history.go         → HistoryService
  └─ shell.go           → janela frameless, tray, hotkey, center, esc-hide
frontend/               → app React (Vite)
internal/ui/            → DELETADO no fim (todo o Fyne)
```

## Backend — serviços bindados ao JS

Três serviços expõem o núcleo Go ao frontend via bindings do Wails:

### ChatService

- `Send(text string)` — dispara a chamada da IA numa goroutine e retorna na hora.
  Emite eventos conforme o streaming evolui:
  - `chat:delta` — cada pedaço de texto recebido.
  - `chat:usage` — tokens e custo acumulados (`config.ModelPrices`).
  - `chat:done` — resposta completa.
  - `chat:error` — erro durante o streaming.
  Internamente é o mesmo fluxo de `sendMessage` atual (`ai.Stream` +
  `db.CreateConversation`/`db.AddMessage` + manutenção do histórico), trocando o
  destino do delta de `label.SetText` para `app.Event.Emit`.
- `Cancel()` — cancela o streaming em curso (preserva o comportamento de cancelar
  ao esconder a janela).
- `NewConversation()` — reseta conversa, histórico, tokens e custo.

### ConfigService

- `Get()` — retorna o `Config` atual.
- `Save(cfg)` — persiste, reaplica a configuração de hotkey e atualiza o estado em
  memória.

### HistoryService

- `List()` — conversas salvas.
- `Messages(id)` — mensagens de uma conversa.
- `Delete(id)` — remove uma conversa.

## Frontend — React de janela única, com views

Tudo vira **views dentro de uma única janela** (decisão travada), substituindo as
janelas OS separadas que o Fyne abria pra settings e history — mais fácil de
estilizar e mais coerente com um overlay.

- **Chat** (view padrão): input multiline + lista de mensagens. Escuta `chat:delta`,
  acumula e renderiza **markdown incremental** com `react-markdown`. Exibe
  tokens/custo a partir de `chat:usage`. O comando `/new` é reimplementado no parse
  do input (chama `ChatService.NewConversation`).
- **Settings**: formulário mapeando o `Config` (tema, idioma, hotkeys e intervalos,
  modelo, API key, system prompt). As traduções pt/en migram pro frontend.
- **History**: lista de conversas + detalhe (reproduz o split atual), com exclusão.

### Estratégia visual (objetivo central)

- **Design tokens em CSS variables** (`--bg`, `--surface`, `--radius`, `--font`,
  cores por papel etc.) como fonte única de verdade visual. Restyle global vira
  trivial e abre caminho para temas no futuro.
- Animações de entrada de card e fade da janela via CSS/Framer Motion.
- Gradientes, sombras e markdown rico: destravados pelo uso de HTML/CSS.

**Decisão de estilização (travada):** **Tailwind v4** configurado sobre **design
tokens em CSS variables**. Regra de ouro: nenhum componente usa cor/raio/espaçamento
literal — tudo referencia um token (`bg-surface`, `text-fg`, `rounded-card` etc.)
mapeado para uma CSS variable. Isso padroniza o visual e deixa **temas** prontos
para entrar trocando apenas o conjunto de variáveis (ex.: `[data-theme="dark"]`).
O `Config.Theme` (hoje light/dark) passa a selecionar o conjunto de tokens ativo.

## Janela, tray e hotkey

- **Boot:** janela criada escondida, `Frameless: true`, `AlwaysOnTop: true`. System
  tray com itens **Exibir** e **Sair**.
- **Abrir:** hotkey global (duplo-Espaço, configurável) chama `window.Show()` +
  `window.Center()` + foco no input. Reaproveita `internal/hotkey`; o
  `doublePressDetector` é mantido.
- **Fechar/esconder:** `Esc` duplo no frontend chama um método `Hide()` bindado que
  esconde a janela **e cancela o streaming em curso** (preserva o comportamento
  atual). Hook `WindowClosing` esconde em vez de destruir.
- Centralização e remoção de barra de título deixam de usar `user32.dll`.

## Ordem de migração

Cada passo mantém o app funcional:

1. Scaffold Wails v3 + React (Vite), janela vazia rodando.
2. `ConfigService` + view Settings.
3. `ChatService` com streaming via eventos + view Chat com markdown.
4. `HistoryService` + view History.
5. Frameless + AlwaysOnTop + tray + hotkey + center + esc-hide.
6. Passada de estilo: design tokens, animações, polish.
7. Deletar `internal/ui` (Fyne) e ajustar build (`goreleaser`, `lefthook`,
   scripts) para `wails3 build`.

## Testes

- Os serviços Go são unit-testáveis; os testes existentes de `ai`, `config` e `db`
  continuam válidos.
- Adicionar testes da camada de serviço (`internal/app`), especialmente do
  ChatService (sequência de eventos, cancelamento, persistência no db).
- Testes de componente no frontend são opcionais nesta fase.

## Riscos

- **Wails v3 é alpha** — API pode mudar entre versões; fixar a versão e revisar a
  doc oficial durante a implementação.
- **Interação tray ↔ hotkey ↔ janela** no Windows precisa de validação manual
  (mostrar/esconder/focar com a hotkey global enquanto o tray vive).
- **Build/empacotamento** — `goreleaser` e hooks precisam migrar do build Go puro
  pro fluxo `wails3 build` (frontend embarcado).
- **Frameless no Windows** — validar comportamento de foco e posicionamento.
