# Ausente na sala

Resumo do desenho da função "ausente na sala". A fonte completa é a spec aprovada; este documento cobre o que importa para quem mantém o backend.

## Regras de produto

| Situação | Resultado |
|---|---|
| Sozinho na sala de voz e sem atividade por 2:30 | Saída completa (desconecta do LiveKit, sem entrada de ausente). A aba mostra "Você saiu da call por inatividade" e "Entrar de novo". |
| Com pelo menos um participante ativo, sem fala e sem interação por 5:00 | Ausente (desconecta do LiveKit, continua visível na mesma sala como ausente). |
| Compartilhando ou assistindo uma live | Nunca fica ausente nem sai; o relógio recomeça quando a live acaba. |
| 30 s antes de qualquer um dos dois gatilhos | Aviso com "Continuar aqui" e notificação do sistema se a aba estiver oculta. |
| LiveKit desiste de reconectar | Ausente (motivo `connection`) se havia participante ativo; senão saída completa com "A conexão de voz caiu.". Sem reconexão automática. |
| Visto pelos outros | Mesma sala, avatar e linha da barra lateral semitransparentes com "zzz" e dica "Ausente, não está ouvindo". |
| Só ausentes restaram na sala | Todas as entradas de ausente da sala somem na hora; as abas delas mostram a saída por inatividade. |
| A conexão WebSocket dona da entrada fecha | Entrada removida. |
| O processo da API morre sem rodar o hook de fechamento | Entrada removida pelo varredor em até 60 s. |

Participante ativo é quem está na presença do LiveKit da sala e não está no conjunto de ausentes dela. Ausentes não são participantes do LiveKit, então nunca contam como companhia.

## Eventos WebSocket

Envelope padrão: `{"v":1,"type":"...","topic":"...","ts":"<RFC3339>","data":...}`. O roteador despacha só por `type`.

Cliente para servidor:

| type | topic | data |
|---|---|---|
| `voice.away.enter` | `room:<room_id>` | `{"room_id":"<uuid>","reason":"idle"\|"connection"}` |
| `voice.away.leave` | `__presence__` | `{}` |

O servidor nunca lê `user_id` do payload: usa o usuário e a conexão da sessão WebSocket.

Servidor para cliente:

| type | topic | data |
|---|---|---|
| `voice.away.snapshot` | `room:<room_id>` | `[{"user_id","username","since"}]`, lista completa, pode ser `[]` |
| `voice.away.ended` | `room:<room_id>` | `{"user_id","reason":"room_empty"\|"returned"\|"connection_closed"}` |
| `voice.away.resync` | `__presence__` | `{}` |
| `voice.presence.joined`, `left`, `snapshot` | `room:<room_id>` | formato atual, agora lido do KV |

Os eventos de ausente vão no tópico da sala porque só inscritos autorizados na sala os recebem. `voice.presence.*` nunca mistura ausentes.

## Chaves no KV

Prefixo `P` = `vz:` + `KEY_VALUE_PREFIX` (em produção, `vz:prod`). Tempos dentro do JSON em milissegundos Unix.

| Chave | Tipo | Campo e valor | TTL |
|---|---|---|---|
| `P:room:{room_id}:presence` | hash | `sid` e JSON com `user_id` e `username` | nenhum (limpa no `room_finished`) |
| `P:room:{room_id}:away` | hash | `user_id` e JSON com `username`, `since`, `conn_id`, `instance_id` | nenhum |
| `P:away:index` | hash | `user_id` e `room_id` (no máximo uma entrada de ausente por usuário) | nenhum |
| `P:instance:{instance_id}` | string | `1` | 30 s, renovada a cada 10 s |
| `P:epoch` | string | id da instância que criou | nenhum; `SET NX` revela uma limpeza do KV |

Toda escrita passa por scripts Lua idempotentes (`internal/voice/scripts/`). O estado fica atrás da porta `VoiceRoomState`: `state_kv.go` em produção e `state_memory.go` só em testes, e `cmd/api/main.go` nunca referencia a de memória. As duas passam pela mesma suíte de contrato (`state_contract_test.go`).

## Variáveis de ambiente

- `KEY_VALUE_URL`: URL do Key Value (`redis://` ou `rediss://`). Em dev, `redis://localhost:6379/0`.
- `KEY_VALUE_PREFIX`: só letras minúsculas, números e hífen. Ambientes que dividem a mesma instância usam prefixos diferentes (`prod`, `staging`).
- As duas são obrigatórias: se faltarem ou forem inválidas, o processo encerra na subida (`log.Fatalf`).

## Falhas e o que acontece

| Caminho | Comportamento |
|---|---|
| Webhook do LiveKit com erro ou timeout no KV | HTTP 503 e log; o LiveKit reenvia em ordem. Se desistir, a sala se corrige no `room_finished`. |
| `voice.away.enter` ou `leave` com erro no KV | Log e nada publicado. O cliente continua ausente localmente e reenvia a intenção no próximo open do WS ou no `voice.away.resync`. |
| Liberação ao fechar o WS com erro no KV | 3 tentativas (100, 300 e 900 ms), depois log. O resto é limpo quando o usuário volta, a sala esvazia ou o varredor age. |
| Snapshot no `room.subscribe` com erro no KV | Nenhum snapshot (nunca `[]`) e log; o cliente mantém o que tem. |
| KV inalcançável na subida | Log `kv.unreachable_at_startup` e segue (chat e auth não dependem do KV). O heartbeat continua tentando. |
| KV zerado (reinício do plano Free) | O `Beat` detecta pela época; presença é reidratada a partir do LiveKit (se falhar, repete a cada batida até dar certo; só a primeira falha e a recuperação são logadas). O `voice.away.resync` sai logo após a detecção, independente da reidratação, e as abas ausentes reenviam o `enter`. |
| API morreu sem rodar o hook | Varredor (a cada 30 s) remove entradas de instâncias sem heartbeat. Cada passada examina no máximo 500 entradas do índice (suficiente na escala atual). |

Entradas de uma instância morta duram menos de 60 s. Um reinício do KV Free esvazia a barra lateral por cerca de 10 s mais o tempo da API do LiveKit.

## Conexões com o KV

Cada instância usa no máximo 7 conexões (pool de 5 mais pool de pipeline de 2; o padrão do go-redis v9.23 seria 10 extras). Durante a sobreposição de deploy são 14, bem abaixo das 50 do plano Free.

## Ordem de deploy

1. Frontend com o filtro de eventos desconhecidos (F1) precisa estar em produção antes de este backend chegar lá.
2. O backend (porta, KV, handlers e webhook) sai num único PR; as variáveis do item 3 precisam existir antes do deploy.
3. KV e variáveis: produção já tem `KEY_VALUE_URL` e `KEY_VALUE_PREFIX=prod`. Qualquer outro serviço com este backend (por exemplo staging) precisa estar em Virginia, usar a mesma URL interna, ter prefixo próprio e ter as duas variáveis antes deste deploy, porque a falta delas é fatal.
4. Backend: staging, verificação, depois produção.
5. A parte do frontend com a tela de ausente só vai depois de o backend estar em produção.

## Verificação em staging

- Com duas ou três contas: aviso aos 4:30 e ausente aos 5:00 com a aba oculta; sozinho, aviso aos 2:00 e saída aos 2:30; nada acontece durante live; "Voltar" restaura sala, microfone e surdez.
- Reiniciar a API: presença da barra lateral sobrevive e abas ausentes voltam como ausentes.
- Logs: `kv.connected`, `voice.hydrate` no primeiro boot, `voice.away.entered` e `voice.away.cleared`, nenhum `voice.webhook level=error` no uso normal.
