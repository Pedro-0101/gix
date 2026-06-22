# gix

Overlay de chat com IA no estilo Spotlight/Raycast para desktop. Uma janela
frameless que aparece com um atalho global, conversa com modelos via OpenRouter
(streaming, com markdown), guarda o histórico e mostra o custo em tokens.

Construído com **Go + [Wails v3](https://v3.wails.io)** no backend e **React +
TypeScript + Tailwind v4** no frontend. O núcleo (cliente de IA, config, banco
SQLite) é Go puro; a UI é web embarcada.

## Requisitos

- Go 1.25+
- Node 20+ e npm
- [Wails v3 CLI](https://v3.wails.io): `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
  (garanta `$(go env GOPATH)/bin` no PATH)
- WebView2 (já presente no Windows 10/11)
- Rode `wails3 doctor` para validar o ambiente.

## Configuração

A chave da API do OpenRouter pode vir das configurações do app (engrenagem) ou
da variável de ambiente `OPENROUTER_API_KEY` (também lida de um arquivo `.env`
no diretório atual ou ao lado do executável).

## Desenvolvimento

```sh
wails3 dev
```

Compila o frontend, gera os bindings, embute o `frontend/dist` e abre a janela
com hot-reload.

## Build

```sh
wails3 build
```

Gera os bindings (`-ts`), builda o frontend em produção, embute os assets e
compila o binário com ícone e manifest em `bin/gix.exe`.

## Empacotamento (instalador)

```sh
wails3 package
```

Produz o instalador da plataforma. No Windows usa a configuração NSIS em
`build/windows/nsis` — requer o [NSIS](https://nsis.sourceforge.io) instalado.

## Uso

- **Duplo-Espaço** (configurável): mostra/centraliza/foca a janela.
- **Enter** envia; **Shift+Enter** quebra linha.
- **Duplo-Esc**: esconde a janela e cancela o streaming em curso.
- `/new`: inicia uma nova conversa.
- Fechar a janela a esconde (o app continua na bandeja); use **Sair** no tray
  para encerrar.

## Estrutura

```
main.go                 entrypoint Wails (embed de frontend/dist + ícone)
internal/ai             cliente OpenRouter + streaming
internal/config         config (JSON em UserConfigDir) + preços de modelos
internal/db             histórico de conversas (SQLite, modernc.org/sqlite)
internal/hotkey         hook de teclado global (por plataforma)
internal/app            serviços Wails (Chat/Config/History) + bootstrap (shell.go)
frontend/               app React (Vite + TS + Tailwind v4)
```

## Temas

A aparência é controlada por design tokens (CSS variables) em
`frontend/src/styles/tokens.css`. O campo `theme` da config seleciona o conjunto
ativo via `data-theme` (`light`/`dark`). Adicionar um tema novo é só definir um
bloco de variáveis — nenhum componente usa cor literal.
