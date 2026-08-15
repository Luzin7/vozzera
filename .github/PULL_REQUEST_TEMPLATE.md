<!-- Cole o número da issue que este PR fecha: -->
Closes #

## Resumo

<!-- Uma ou duas frases: o que mudou e por quê. -->

## Tipo

- [ ] `feat:` nova funcionalidade
- [ ] `fix:` correção de bug
- [ ] `refactor:` mudou código, não mudou comportamento
- [ ] `docs:` só documentação
- [ ] `chore:` deps / makefile / CI
- [ ] `test:` testes

## Onde mexi

- [ ] `internal/auth`
- [ ] `internal/chat`
- [ ] `internal/voice`
- [ ] `internal/shared`
- [ ] `sql/migrations`
- [ ] `cmd/api`
- [ ] docs / templates

## Checklist antes de pedir review

- [ ] `make tidy` rodou sem mudanças que devem entrar no commit
- [ ] Se mexi em `queries.sql`, rodei `make generate` e **commitei** os arquivos gerados (`db.go`, `models.go`, `queries.sql.go`) junto — o build do Dockerfile compila o que está no repo
- [ ] Se mexi em schema, criei migration nova com `make migrate-create name=...` e ela tem `-- +goose Up` e `-- +goose Down`
- [ ] `go build ./...` passa
- [ ] `go vet ./...` não reclama
- [ ] `go run -race ./cmd/api` sobe sem data race e sem panic
- [ ] Não tem `else` onde cabia early return
- [ ] Não instanciei nada com estado/goroutine própria dentro de handler (Hub, pool) — usei o que vem injetado
- [ ] Scripts/logs/segredos não foram commitados (`.env`, paths locais, chaves)

## Screenshots / logs

<!-- Se mexi em WebSocket, voz ou fluxo que não dá pra ver com curl, cole print/log mostrando funcionando. -->
