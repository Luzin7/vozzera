# Vozzera Backend — Arquitetura & Convenções

## Go

Linguagem compilada com concorrência nativa via goroutines/channels e stdlib HTTP robusta — sem framework externo. Ideal pro nosso caso: WebSocket hub + REST + voice signaling, tudo no mesmo binário.

---

## Estrutura de Pastas (Vertical Slicing)

Cada domínio é **auto-contido**: handler, service, queries SQL e código gerado vivem juntos. Nenhum domínio importa outro diretamente — compartilhamento só via `shared/`.

```
/cmd
  /api
    main.go                # Ponto de entrada, inicialização de dependências e server HTTP

/internal
  /auth
    handler.go             # Rotas POST /api/register e POST /api/login
    service.go             # Geração/validação de JWT, hash bcrypt
    queries.sql            # Arquivo para o sqlc gerar o repositório deste domínio
    db.go                  # (Gerado pelo sqlc — NÃO editar)
    models.go              # (Gerado pelo sqlc — NÃO editar)
    queries.sql.go         # (Gerado pelo sqlc — NÃO editar)

  /chat
    hub.go                 # Hub struct, map de clients, channels register/unregister/join/broadcast
    client.go              # Conexão WS individual com UserID/Username, readPump/writePump
    handler.go             # Rotas REST de rooms/mensagens, ServeWs (handshake HTTP→WS) e PATCH de edição
    protocol.go            # InboundEvent / OutboundEvent — envelope JSON do WebSocket
    queries.sql            # CreateMessage, GetMessagesByRoom, ListRooms, CreateRoom, UpdateMessage
    db.go                  # (Gerado pelo sqlc)
    models.go              # (Gerado pelo sqlc)
    queries.sql.go         # (Gerado pelo sqlc)

  /voice
    handler.go             # POST /api/voice/token e GET /api/voice/rooms
    livekit.go             # TokenIssuer — assina JWT do LiveKit (protocol/auth, não o server-sdk)
    queries.sql            # GetRoomByID, ListVoiceRooms
    db.go                  # (Gerado pelo sqlc)
    models.go              # (Gerado pelo sqlc)
    queries.sql.go         # (Gerado pelo sqlc)

  /shared                  # ÚNICO lugar para código compartilhado entre domínios
    /db                    # Conexão base do pgxpool
    /config                # Carregamento de env vars (godotenv)
    /httpx                 # Middleware de auth (UserFromContext, Auth()), CORS

/sql
  /migrations              # Arquivos .sql puros, um por migration (goose)
```

**Regra:** se `chat` precisa de algo de `auth` (ex: validar token), isso é feito no `main.go` (injeção) ou vai pra `shared/`. Nunca `import "internal/auth"` de dentro de `internal/chat`.

### Pontos de atenção com estado compartilhado (Hub)

O `Hub` guarda estado em memória (map de clientes por sala) e tem uma goroutine própria (`Run()` que consome os canais `register`/`unregister`/`join`/`broadcast`). Por isso ele é **injetado uma única vez em `main.go`** e repassado pra `chat.RegisterHandlers`. Não é recriado dentro de `RegisterHandlers`.

O `handleUpdateMessage` dispara `h.hub.broadcast <- event` pra notificar os clientes WebSocket da edição. Isso só funciona porque o `Hub` que o handler recebe é o mesmo que tem `Run()` rodando e onde os clientes WS estão registrados. Se cada ponto de entrada criasse seu próprio `Hub`, a edição seria despejada num canal que ninguém consome e os clientes nunca saberiam dela.

**Princípio geral:** qualquer objeto com estado compartilhado + goroutine própria (Hub, pool de DB) é instanciado em `main` e *passado* adiante por parâmetro. Função que precisa chamar método de X → X entra como parâmetro, nunca é instanciado dentro.

---

## Schema, Queries e sqlc

### Fluxo

```
1. Escreve/altera DDL em sql/migrations/
2. Escreve queries no queries.sql do domínio (internal/auth/, internal/chat/)
3. Roda: sqlc generate
4. Código Go gerado aparece no mesmo pacote do domínio (db.go, models.go, queries.sql.go)
```

### Schema (`sql/migrations/`)

DDL puro. Cada arquivo é uma migration numerada. O sqlc lê todos pra entender os tipos das colunas.

```sql
-- sql/migrations/001_init.sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Queries (`internal/<domínio>/queries.sql`)

Cada query tem um **comentário mágico** que define o nome do método Go e o tipo de retorno:

```sql
-- name: CreateUser :one
INSERT INTO users (username, password_hash)
VALUES ($1, $2)
RETURNING id, username, created_at;
```

| Anotação      | Retorno Go              | Quando usar                                    |
|---------------|-------------------------|-------------------------------------------------|
| `:one`        | `(Row, error)`          | INSERT RETURNING, SELECT ... LIMIT 1            |
| `:many`       | `([]Row, error)`        | SELECT que retorna N rows                       |
| `:exec`       | `error`                 | UPDATE/DELETE sem retorno                       |
| `:execrows`   | `(int64, error)`        | Quando precisa do count de rows afetadas        |

### sqlc.yaml

Cada domínio é um entry separado que aponta pra **seu próprio** `queries.sql`, mas **todos compartilham o mesmo schema** (migrations):

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/auth/queries.sql"      # queries do auth
    schema: "sql/migrations/"                  # schema compartilhado
    gen:
      go:
        package: "auth"                        # package = nome do domínio
        out: "internal/auth"                   # output = pasta do domínio
        sql_package: "pgx/v5"
        emit_json_tags: true
        overrides:
          - db_type: uuid
            go_type: github.com/google/uuid.UUID
          - db_type: timestamptz
            go_type: time.Time

  - engine: "postgresql"
    queries: "internal/chat/queries.sql"
    schema: "sql/migrations/"
    gen:
      go:
        package: "chat"
        out: "internal/chat"
        # ... mesmos overrides
```

### Gerar

```bash
make generate    # ou: sqlc generate
```

Isso gera `db.go`, `models.go` e `queries.sql.go` dentro de cada domínio. **Nunca editar arquivos gerados** — qualquer mudança é no `.sql` e re-gera.

Para usar no código do domínio, basta chamar `auth.New(pool)` ou `chat.New(pool)` — o `New()` é gerado pelo sqlc e aceita qualquer coisa que implemente a interface `DBTX` (pgxpool.Pool, pgx.Conn, pgx.Tx).

---

## O Que Já Temos

| Domínio    | Status       | Detalhes |
|------------|-------------|----------|
| **Auth**   | Funcional   | `handler.go`: register com invite code + login com JWT 30d via cookie HttpOnly. `service.go`: HashPassword, CheckPassword, GenerateToken, ParseToken. Queries: `CreateUser`, `GetUserByUsername`. |
| **Chat**   | Funcional   | `hub.go`: broker pattern com map + channels, singleton injetado pelo `main.go`. `client.go`: readPump/writePump com backpressure handling (ping/pong/deadlines). `handler.go`: ServeWs, `GET/POST /api/rooms`, `GET /api/rooms/{id}/messages`, `PATCH /api/rooms/{id}/messages/{content_id}` (edição com broadcast `message_edited`). Queries: `CreateMessage`, `GetMessagesByRoom`, `ListRooms`, `CreateRoom`, `UpdateMessage`. |
| **Voice**  | Funcional   | `livekit.go`: `TokenIssuer` assina JWT do LiveKit via `protocol/auth`. `handler.go`: `POST /api/voice/token` (valida sala, exige `type=voice`, assina token) e `GET /api/voice/rooms`. Queries: `GetRoomByID`, `ListVoiceRooms`. |
| **Shared** | Funcional   | `config.go`: `Load()` com godotenv. `db.go`: `Connect()` retorna `*pgxpool.Pool`. `httpx/`: `Auth()` põe user no context, `UserFromContext()`, `CORS()` (provisório — reflete qualquer origin, ver T2 do roadmap). |
| **Main**   | Funcional   | Carrega config → conecta pool → cria `auth/chat/voice` queries → cria `Hub` único e dispara `go hub.Run()` → registra rotas REST/WS com middleware de auth. O `Hub` é injetado em `ServeWs` e em `chat.RegisterHandlers` — mesma instância. |

### Dependências (`go.mod`)

| Pacote | Uso |
|--------|-----|
| `gorilla/websocket` | Upgrade HTTP → WS |
| `jackc/pgx/v5` | Driver PostgreSQL + pool |
| `golang-jwt/v5` | Geração e parsing de JWT |
| `golang.org/x/crypto` | bcrypt |
| `joho/godotenv` | .env loader |
| `google/uuid` | UUIDs |

### Makefile

```bash
make generate   # sqlc generate
make build      # go build -o bin/vozzera ./cmd/api
make run        # go run ./cmd/api
make tidy       # go mod tidy
```
