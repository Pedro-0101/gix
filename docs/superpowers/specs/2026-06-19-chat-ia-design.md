# Design: transformar o gix de app de notas em chat de IA

Data: 2026-06-19

## 1. Contexto e objetivo

O **gix** hoje é um app de **anotações** de acesso rápido, feito em Go com Fyne:
um hotkey global mostra a janela em qualquer lugar do sistema, o usuário digita
uma nota, ela é salva em SQLite e listada, e a janela fecha com Esc duplo. Tem
system tray, configurações (tema/idioma/hotkeys) e, no Windows, remove os botões
de minimizar/maximizar da janela.

O objetivo é **transformar o gix num chat de IA** mantendo exatamente a mesma
pegada de uso: abre rápido em qualquer lugar → você pergunta → a IA responde →
você fecha rápido. As conversas são salvas no banco. A IA é acessada pela
**OpenRouter** (API compatível com OpenAI), cuja chave já existe no `.env`
(`OPEN_ROUTER_API`).

Toda a infraestrutura existente é **reaproveitada** (hotkey, janela sem botões,
tray, Esc-duplo, foco automático no input, settings, SQLite). A UI de notas é
**substituída** pela UI de chat.

## 2. Decisões (definidas no brainstorming)

- **Modelo de conversa:** uma **nova conversa a cada abertura** da janela. Há
  contexto multi-turno enquanto a janela está aberta. Ao fechar, a conversa fica
  salva no histórico, acessível por um botão.
- **Streaming:** a resposta da IA aparece **token a token** conforme chega.
- **Modelo do OpenRouter:** padrão é um **modelo gratuito** (slug terminando em
  `:free`); se não houver um adequado, o mais barato. É **editável nas
  configurações**. O slug exato do padrão é confirmado contra a lista ao vivo do
  OpenRouter na implementação.
- **Chave da API:** lida das **configurações** (campo salvo na pasta de config
  do usuário); se vazio, cai para a variável de ambiente `OPEN_ROUTER_API`
  (carregada do `.env` em desenvolvimento).
- **Prompt de sistema:** opcional, com um padrão **conciso** (ex.: "Responda de
  forma direta e objetiva."), editável nas configurações.
- **Renderização das respostas:** **texto simples** por enquanto (sem Markdown e
  sem botão de copiar — fica para depois).

## 3. Escopo

### Dentro do escopo
- Reescrever a UI principal (`internal/ui/window.go`) para um chat.
- Novo pacote `internal/ai`: cliente OpenRouter com streaming SSE.
- Estender `internal/db` com tabelas/métodos de conversas e mensagens.
- Estender `internal/config` e `internal/ui/settings.go` com modelo, chave da
  API e prompt de sistema.
- Carregar o `.env` em desenvolvimento (parser mínimo, sem dependência nova).
- Tratamento de erros visível dentro do próprio chat.
- Testes para o cliente OpenRouter (parser SSE) e para o banco.

### Fora do escopo (YAGNI)
- Múltiplas conversas simultâneas / threads estilo ChatGPT com troca lateral.
- Continuar/editar conversas antigas do histórico (histórico é só leitura).
- Renderização Markdown e botão de copiar nas respostas.
- Controles de temperatura, `max_tokens`, top_p etc.
- Anexos, imagens, voz.
- Retry automático e troca de modelo por conversa.

## 4. Arquitetura e pacotes

```
cmd/gix/main.go            -> inalterado (chama ui.Run)
internal/config/config.go  -> + Model, APIKey, SystemPrompt; loader de .env
internal/db/db.go          -> + conversations, messages (CRUD); remove CRUD de notas
internal/ai/client.go      -> NOVO: cliente OpenRouter com streaming
internal/ui/window.go      -> reescrito: UI de chat + orquestração
internal/ui/settings.go    -> + campos de modelo/chave/prompt; strings de tradução
internal/ui/history.go     -> NOVO (opcional): janela de histórico
internal/ui/hotkey*.go     -> inalterado
```

Princípio: a UI fica fina; a lógica de rede mora em `internal/ai` e a
persistência em `internal/db`. Cada unidade é testável isoladamente (o cliente
contra um `httptest.Server`, o banco contra um arquivo temporário).

## 5. Modelo de dados

Novas tabelas (criadas com `IF NOT EXISTS`). A tabela `notes` antiga é
**preservada** (sem `DROP`), apenas deixa de ser usada.

```sql
CREATE TABLE IF NOT EXISTS conversations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT NOT NULL,
    model      TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL,
    role            TEXT NOT NULL,   -- 'user' | 'assistant'
    content         TEXT NOT NULL,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id)
);
```

Tipos e métodos em `internal/db`:

```go
type Conversation struct { ID int64; Title, Model string; CreatedAt time.Time }
type Message       struct { ID int64; Role, Content string }

func (d *Database) CreateConversation(title, model string) (int64, error)
func (d *Database) AddMessage(convID int64, role, content string) error
func (d *Database) ListConversations() ([]Conversation, error) // mais recentes primeiro
func (d *Database) GetMessages(convID int64) ([]Message, error) // ordem cronológica
func (d *Database) DeleteConversation(id int64) error           // apaga mensagens + conversa
```

`ExtractTitle` é reaproveitada/adaptada para gerar o título da conversa a partir
da primeira mensagem do usuário (primeira linha, truncada em ~40 caracteres). Os
métodos de CRUD de notas (`Create`/`List`/`Delete` + tipo `Note`) são removidos.

**Persistência incremental:** a conversa é criada no banco na primeira mensagem
do usuário; cada mensagem (do usuário e a resposta da IA) é gravada assim que
concluída, para não se perder nada se o app for fechado.

## 6. Fluxo de interação

Estado em memória da conversa atual: `convID int64` (0 = ainda não persistida) e
`history []ai.Message`.

1. **Abrir (hotkey):** a janela aparece e o input recebe foco. Se a conversa
   atual já tiver ≥1 mensagem, ela é finalizada e **começa uma nova** (limpa
   `convID`/`history` e a área de mensagens). Se estiver vazia, mantém como está
   (não cria conversas vazias).
2. **Enviar (Enter):** Shift+Enter insere nova linha; Enter envia. Texto vazio é
   ignorado. Enquanto há um streaming em andamento, Enter é ignorado (flag
   `streaming`).
   - Se `convID == 0`: cria a conversa (`CreateConversation(title, model)`).
   - Grava a mensagem do usuário (`AddMessage(convID, "user", texto)`), adiciona
     a `history`, e mostra na área de mensagens.
3. **Responder (streaming):** monta as mensagens (prompt de sistema opcional +
   `history`) e chama `ai.Stream(ctx, model, msgs, onDelta)` numa goroutine. Um
   placeholder "IA" aparece com indicador "pensando…" até o primeiro token;
   depois cada delta é **acrescentado ao label** via `fyne.Do`, com a área
   rolando para o fim. Ao terminar, a resposta completa é gravada
   (`AddMessage(convID, "assistant", textoCompleto)`) e adicionada à `history`.
4. **Fechar (Esc duplo):** esconde a janela e **cancela** o streaming em
   andamento (via `context.CancelFunc`). Se houver texto parcial recebido, ele é
   gravado como mensagem da IA; se nada chegou, nada é gravado.

## 7. Cliente OpenRouter (`internal/ai`)

```go
type Message struct { Role string `json:"role"`; Content string `json:"content"` }

type Client struct { httpClient *http.Client; apiKey string }

func New(apiKey string) *Client

// Stream faz POST com stream:true e chama onDelta a cada pedaço de texto.
func (c *Client) Stream(ctx context.Context, model string, messages []Message,
    onDelta func(string)) error
```

- **Endpoint:** `POST https://openrouter.ai/api/v1/chat/completions`
- **Headers:** `Authorization: Bearer <chave>`, `Content-Type: application/json`,
  `X-Title: gix` (recomendado pelo OpenRouter; opcional).
- **Body:** `{ "model": <slug>, "messages": [...], "stream": true }`. Se o prompt
  de sistema não for vazio, ele entra como primeira mensagem `{role:"system"}`.
- **SSE:** ler o corpo linha a linha. Linhas relevantes começam com `data: `.
  - `data: [DONE]` encerra.
  - caso contrário, faz `json.Unmarshal` e extrai
    `choices[0].delta.content` (pode vir vazio/ausente) → chama `onDelta`.
  - linhas vazias e linhas de comentário iniciadas por `:` (ex.:
    `: OPENROUTER PROCESSING`, keep-alive) são ignoradas.
- Status HTTP != 2xx → retorna erro com o corpo (para exibir no chat).
- `ctx` cancelado → o request é abortado (usar `http.NewRequestWithContext`).

## 8. Configuração e chave da API

Novos campos em `config.Config` (com defaults em `Default()`):

```go
Model        string `json:"model"`         // "" => usa DefaultModel (slug :free)
APIKey       string `json:"api_key"`       // "" => usa env OPEN_ROUTER_API
SystemPrompt string `json:"system_prompt"` // default: "Responda de forma direta e objetiva."
```

- `const DefaultModel` = um slug gratuito do OpenRouter, confirmado na
  implementação contra a lista ao vivo.
- **Resolução da chave:** `cfg.APIKey` se não vazio; senão
  `os.Getenv("OPEN_ROUTER_API")`.
- **Loader de `.env`:** na inicialização, se existir um `.env` (no diretório de
  trabalho ou ao lado do executável), um parser mínimo lê linhas `CHAVE=VALOR`
  (ignora `#` e linhas em branco, remove aspas) e popula apenas variáveis de
  ambiente ainda não definidas. Sem dependência externa. O `.env` continua no
  `.gitignore`.

## 9. UI

Mantém janela **400×600 fixa**, sem botões no Windows, e o comportamento de
foco/abrir/fechar já existentes.

- **Topo:** botão `⚙` (configurações) e `🕘` (histórico), alinhados à direita.
- **Centro:** área de mensagens rolável (`container.NewVScroll` sobre um `VBox`).
  Cada mensagem é um `widget.Label` com quebra de linha por palavra; mensagem do
  usuário com prefixo/rótulo distinto ("Você") e a da IA com rótulo "IA". Texto
  simples por enquanto.
- **Base:** `escEntry` multilinha (reaproveitado) como caixa de pergunta, com
  placeholder "pergunte algo…"; indicador "pensando…" enquanto aguarda o
  primeiro token.
- **Histórico (`🕘`):** abre uma janela separada (como a de settings) listando as
  conversas salvas (título + data). Selecionar uma mostra suas mensagens em
  **modo leitura**; cada item tem botão de apagar. Sem continuar/editar.

Widgets de nota removidos: `saveBtn`, `notesList` e toda a lógica de notas.

## 10. Tratamento de erros

Todos os erros aparecem como uma **mensagem da IA no próprio chat** (não-fatais):
- Sem chave configurada (nem settings nem env).
- Erro de rede / timeout.
- Erro da API (status != 2xx), incluindo modelo gratuito indisponível, rate
  limit ou modelo inválido — mostra a mensagem retornada para facilitar o
  ajuste do slug nas settings.
- Cancelamento ao fechar a janela não é mostrado como erro.

## 11. Estratégia de testes

- **`internal/ai`** (`client_test.go`): `httptest.Server` devolvendo SSE
  controlado. Casos: acúmulo de múltiplos deltas; `[DONE]`; linhas de comentário
  `:`/vazias ignoradas; chunk de `data:` partido entre leituras; status de erro
  vira `error`; cancelamento via `context`.
- **`internal/db`** (`db_test.go`): banco em arquivo temporário. Casos: criar
  conversa, anexar mensagens, listar (ordem), buscar mensagens, apagar (remove
  mensagens junto).
- **Título** da conversa: testes da função de extração.
- **UI Fyne:** verificação manual (difícil de automatizar de forma confiável).

Hooks do projeto rodam `go vet ./...` e `go test ./...` no pre-commit, e
`go build ./...` no pre-push — tudo deve passar.

## 12. Riscos e observações

- **Modelos gratuitos do OpenRouter** têm limites de taxa e podem exigir ajustes
  na conta; sua disponibilidade muda com o tempo. Mitigação: o modelo é editável
  nas settings e os erros aparecem no chat.
- **Churn de slug:** o `DefaultModel` pode ficar desatualizado; corrigível
  editando as configurações.
- **Thread-safety do Fyne:** toda atualização de UI a partir da goroutine de
  streaming usa `fyne.Do`.

## 13. Resumo das unidades

| Unidade | O que faz | Como se usa | Depende de |
|---|---|---|---|
| `internal/ai.Client` | Streaming de chat via OpenRouter | `New(key).Stream(ctx, model, msgs, onDelta)` | `net/http`, stdlib |
| `internal/db` | Persistir conversas e mensagens | métodos CRUD | `modernc.org/sqlite` |
| `internal/config` | Carregar/salvar config + `.env` | `Load()`, `Save()` | stdlib |
| `internal/ui` | UI de chat + orquestração | `ui.Run()` | `ai`, `db`, `config`, Fyne |
