# Vozzera Backend

![Go](https://img.shields.io/badge/Go-1.26.0-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-GPLv3-blue)
![Postgres](https://img.shields.io/badge/Postgres-16-336791?logo=postgresql)
![Status](https://img.shields.io/badge/status-early--stage-orange)

Backend de chat em **texto** e **voz** (real-time com WebSocket e LiveKit Cloud). Sessões opacas em cookie HttpOnly com revogação, arquitetura em Vertical Slicing, sem framework HTTP — stdlib do Go + sqlc + pgx.

> Projeto em estágio inicial: as funcionalidades abaixo funcionam ponta a ponta, mas há pontos de robustez pendentes (ver [Roadmap](#roadmap)).

## Funcionalidades

- **Autenticação** — registro por invite code + login com sessão opaca em cookie `HttpOnly; SameSite=None; Secure`, expiração deslizante e revogação via logout.
- **Salas** — texto e voz, listagem e criação via REST.
- **Mensagens de texto** — envio em tempo real via WebSocket, histórico por sala, edição via `PATCH`.
- **Voz** — tokens LiveKit assinados pelo backend; mídia vai direto do browser pro LiveKit Cloud.

## Stack

| Camada        | Tecnologia                                                       |
|---------------|------------------------------------------------------------------|
| HTTP          | stdlib `net/http` (sem framework)                                |
| WebSocket     | `gorilla/websocket`                                              |
| Postgres      | `jackc/pgx/v5` + pool                                            |
| Codegen SQL   | `sqlc` (queries `.sql` → Go fortemente tipado, arquivos gerados) |
| Migrations    | `goose` (via `go tool`)                                          |
| Voz           | `livekit/protocol/auth` (apenas assinatura de token)             |
| Hash          | `golang.org/x/crypto` (bcrypt)                                   |

## Pré-requisitos

- **Go 1.26.0** — versão fixada via `toolchain` no `go.mod` e `golang:1.26.0-alpine` no Dockerfile. Se você instalar uma patch diferente (ex: 1.26.1), o próprio Go baixa a 1.26.0 pra compilar este projeto — você não precisa se preocupar em trocar de versão localmente.
- **Docker** (pra subir o Postgres em dev)
- **Conta no [LiveKit Cloud](https://cloud.livekit.io/)** — URL do projeto, `API key` e `API secret`. Em dev sem voz, o servidor sobe mesmo com essas variáveis vazias.

## Quick start

```bash
# 1. Subir o Postgres de dev
docker compose -f docker-compose.dev.yml up -d

# 2. Configurar env
cp .env.example .env
# preencha INVITE_CODE e as credenciais do LiveKit

# 3. Aplicar migrations e subir o servidor
make migrate-up
make run
```

Servidor em `http://localhost:8080`. Veja `CONTRATO-FRONTEND.md` para o formato de todos os endpoints e eventos WebSocket.

## Makefile

| Target                  | O que faz                                                   |
|-------------------------|-------------------------------------------------------------|
| `make generate`         | Roda `sqlc generate` — regera os arquivos `.go` das queries  |
| `make build`            | Compila pra `bin/vozzera`                                   |
| `make run`              | `go run -race ./cmd/api` (detector de race condition ligado)|
| `make tidy`             | `go mod tidy`                                               |
| `make migrate-create name=descricao` | Cria nova migration .sql                       |
| `make migrate-up`       | Aplica migrations                                           |
| `make migrate-down`     | Reverte a última migration                                  |
| `make migrate-status`   | Mostra estado atual das migrations                          |

`sqlc` e `goose` rodam via `go tool` — não precisam de install externo, são resolvidos pelo `go.mod`.

## Estrutura de pastas

```
cmd/api/main.go              # Montagem de dependências e server HTTP
internal/
  auth/   {handler,service,queries}   # Registro e login
  chat/   {hub,client,handler,protocol,queries}  # WebSocket + REST de mensagens
  voice/  {handler,livekit,queries}   # Endpoints de voz LiveKit
  shared/ {config,db,httpx}          # Código compartilhado entre domínios
sql/migrations/              # DDL puro, uma por arquivo (goose)
```

**Regra central**: domínio não importa domínio. `chat` não importa `auth`, `voice` não importa `chat`. Compartilhamento sobe pra `shared/` ou é costurado em `main.go` por injeção.

Veja [`ARCHITECTURE.md`](./ARCHITECTURE.md) para o detalhamento completo.

## Contribuindo

Issues e PRs são bem-vindos. Antes de abrir PR, leia [`CONTRIBUTING.md`](./CONTRIBUTING.md) — tem as convenções de código, o fluxo de SQL e o que o `sqlc` gera/espera.

## Licença

[GPLv3](./LICENSE).
