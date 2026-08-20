# Contribuindo pro Vozzera

Obrigado por querer contribuir. Antes de abrir PR, leia isto aqui — é curto.

## Comece aqui

```bash
git clone <fork>
cd vozzera
cp .env.example .env           # preencha INVITE_CODE e LiveKit
docker compose -f docker-compose.dev.yml up -d
make migrate-up
make run
```

Servidor sobe em `http://localhost:8080`. Pra confirmar que tá tudo pé:

```bash
curl localhost:8080/api/rooms   # 401 sem cookie = servidor no ar
```

Trabalhe sempre em branch a partir de `develop`. `main` só recebe releases via merge de `develop` — não abra PR direto pra `main`.

### Versão do Go

Fixada em `go 1.26.0` + `toolchain go1.26.0` no `go.mod` (e `golang:1.26.0-alpine` no Dockerfile). Se você instalar uma patch diferente na sua máquina, o próprio Go baixa a 1.26.0 pra compilar — nada de trocar de versão à mão. **Não sobe a versão do Go no `go.mod` num PR casual.** Se um dia precisar subir (security fix, novo recurso), isso é um PR separado e explícito, com `fix:` ou `chore:` conforme o caso.

## Antes de abrir PR

Rode isso na raiz:

```bash
make tidy
make generate          # só se mexeu em queries.sql
go build ./...
go vet ./...
go run -race ./cmd/api  # sobe sem panic
```

O `go run -race` importa: ele detecta acesso concorrente inseguro, que é quando mais aparece nesse projeto (WebSocket + goroutines). Cole no terminal e confirme que não rola data race.

**Nunca commite arquivos gerados pelo sqlc**: `db.go`, `models.go`, `queries.sql.go`. Eles vêm de `make generate` a partir dos `.sql`, editá-los à mão é trabalho perdido no próximo generate.

## Convenções de código

Estas convenções são o espelho das regras do projeto em Go. As que não são óbvias vêm com exemplo.

### Early return, sem `else`

Caminho de erro sai primeiro, caminho feliz fica sem indentação. `else` é exceção rara — se apareceu, normalmente cabia extrair função.

```go
// ✅
if err != nil {
    http.Error(w, "Erro", 500)
    return
}
writeJSON(w, 200, data)

// ❌
if err != nil {
    http.Error(w, "Erro", 500)
    return
} else {
    writeJSON(w, 200, data)   // desnecessariamente indentado
}
```

### Guard clause no topo

Valida entrada, autorização e existência antes de qualquer I/O ou construção de objeto.

```go
func (h *Handler) handleX(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(r.PathValue("id"))
    if err != nil {
        http.Error(w, "ID inválido", 400)
        return
    }
    user, ok := httpx.UserFromContext(r.Context())
    if !ok {
        http.Error(w, "Não autorizado", 401)
        return
    }
    // daqui pra baixo você sabe que id e user são confiáveis
}
```

### `http.Error` não interrompe — sempre seguido de `return`

Esquecer o `return` depois de `http.Error` escreve a resposta duas vezes e enche o log de `superfluous response.WriteHeader`.

### Erro é (`valor`, `error`), não `throw`/`panic`

Em service/repository, retorne `(resultado, error)`. Em handler HTTP, trate o erro e saia. Não lance `panic` em fluxo normal — `panic` é pra bugs invariantes).

### Injeção por construtor, nunca estado criado dentro de handler

Dependências entram como parâmetro e ficam guardadas na struct `Handler`. O ponto de montagem é `main.go`.

```go
// ✅ main.go instancia uma vez
hub := chat.NewHub(chatQueries)
go hub.Run()
chat.RegisterHandlers(mux, chatQueries, hub, authMw)

// ✅ RegisterHandlers guarda a referência, não cria
func RegisterHandlers(mux *http.ServeMux, queries *Queries, hub *Hub, authMw ...) {
    h := &Handler{queries: queries, hub: hub}
    // ...
}
```

**Anti-padrão** (já aconteceu): `RegisterHandlers` chamando `NewHub` internamente. Isso cria um segundo Hub, sem `Run()` rodando, e o broadcast morre num canal que ninguém consome.

**Regra geral:** objeto com estado em memória + goroutine própria (Hub, pool de DB) é instanciado em `main` e *passado* adiante. Se uma função precisa chamar método de X, X entra como parâmetro — nunca é instanciado dentro dela.

### Vertical slicing — domínio não importa domínio

Cada domínio em `internal/<domínio>/` é auto-contido: handler, service, queries, código gerado. `chat` não importa `auth`, `voice` não importa `chat`. Compartilhamento sobe pra `shared/` ou é costurado em `main.go`.

Precisa da mesma query em dois domínios? Duplique o SELECT. É o custo mais barto hoje; criar acoplamento entre domínios é mais caro amanhã.

### Comentário é exceção

Se precisa comentar pra explicar, o código não está bom o suficiente — melhora o nome, extrai a função, nomeia a constante. Não comente decisão, data nem quem pediu.

### Complexidade por função

Mais de 3-5 `if` numa função é sinal de extração, não de função grande. As verificações viram uma função com nome (`canSubmit`, `blockedReasonsFor`), e a principal fica legível.

Mais de 2-3 parâmetros pede objeto nomeado (struct) ou quebra da função. Ordem posicional de 4 argumentos é bug esperando.

### Identificador em inglês, mensagem ao usuário em pt-BR

```go
http.Error(w, "Não autorizado", http.StatusUnauthorized)   // mensagem em pt
type UpdateMessageRequest struct { ... }                  // nome em en
```

## Fluxo de SQL

1. Editar ou criar `internal/<domínio>/queries.sql` — uma query por bloco, com o comentário mágico de nome/retorno:
   ```sql
   -- name: UpdateMessage :one
   UPDATE messages SET content = $1, updated_at = NOW()
   WHERE id = $2 AND user_id = $3
   RETURNING id, room_id, content, updated_at;
   ```
2. `make generate` → regera `db.go`, `models.go`, `queries.sql.go` no mesmo pacote.
3. Usar no código: `h.queries.UpdateMessage(ctx, UpdateMessageParams{...})`.

Pra mudar schema (tabela nova, coluna, índice):

```bash
make migrate-create name=descricao
# edita sql/migrations/<timestamp>_descricao.sql com -- +goose Up / -- +goose Down
make migrate-up
```

Depois que o schema mudou, o `sqlc` precisa regenerar tipos: `make generate`.

**Nunca edite arquivos gerados pelo sqlc.** Qualquer mudança é no `.sql` e re-gera. Se você achar precisando mexer em `db.go`/`models.go`/`queries.sql.go`, é sinal de que faltou mexer no `.sql`.

## Mensagens de commit

Conventional commits, em pt ou en (desde que consistente no PR):

| Prefixo  | Quando                                            |
|----------|---------------------------------------------------|
| `feat:`  | Nova funcionalidade ou endpoint                   |
| `fix:`   | Correção de bug                                   |
| `refactor:` | Mudou código, não mudou comportamento          |
| `docs:`  | Só documentação                                   |
| `chore:` | Deps, makefile, configs que não tocam runtime     |
| `ci:`    | Workflows do GitHub                               |
| `test:`  | Testes (quando existirem)                         |

Exemplos:

```
feat: adicionar PATCH /api/rooms/{id}/messages/{content_id}
fix: usar o Hub injetado em vez de criar novo em RegisterHandlers
docs: atualizar ARCHITECTURE.md com voice e PATCH de edição
```

Pode squash de commits intermediários no PR, mas cada commit final deve compilar.

## Templates de issue e PR

- Bugs e features têm template YAML em `.github/ISSUE_TEMPLATE/`. Preencha o que for solicitado — isso economiza ida e volta.
- PRs colam o template de `.github/PULL_REQUEST_TEMPLATE.md` no corpo. O checklist ali é pra você rodar antes de pedir review.

## Onde pedir ajuda

Abra uma issue com o template `bug_report` se encontrar comportamento estranho. Se a dúvida for conceitual ("como funciona X?"), prefira Discussions em vez de issue — issues são pra rastrear trabalho.

Iniciantes em Go são bem-vindos. As convenções aqui são deliberadamente explícitas pra que você não precise adivinhar.
