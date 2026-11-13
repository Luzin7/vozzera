# Runbook: app rollback, migration revert and Neon PITR

Operational steps for Vozzera's Go API (Render + Neon + GHCR). Every command assumes the
repository root as working directory and a shell with `set -euo pipefail` already active for
the session. All timestamps are UTC.

## What CI proves and what it does not

`.github/workflows/migrations-verify.yml` runs on every PR that touches `sql/migrations/**` and
on demand: it starts an empty `postgres:17`, applies every migration with `goose up`, reverts
exactly one migration with `goose down`, and applies it again. The assertion is narrow: it reads
the latest applied version id from `goose_db_version` (`WHERE is_applied`), requires that version
to go down after the single `down`, and requires the same version back after `up`. Proven by CI:
the full migration chain applies on an empty PostgreSQL 17, and the **latest** Down block parses,
executes and can be re-applied. It proves nothing about the other Down blocks, and nothing about
staging: every run is an ephemeral `postgres:17` container, never the staging database.

Not proven by CI: the earlier Down blocks. `goose down` only reverses the most recent migration,
so the other eight are not exercised by the workflow. A one-off local run on a clean
`postgres:17` executed `goose down-to 0` across all nine migrations and then `goose up`, all
successful (recorded on issue #61). That run was an ephemeral local database too; issue #61's
"exercised against staging" step is still owed. Re-run the local cycle when an old migration is
edited:

```bash
goose -dir sql/migrations postgres "$DATABASE_URL" down-to 0
goose -dir sql/migrations postgres "$DATABASE_URL" up
```

## App rollback (bad deploy, schema unchanged)

The deploy webhook (`.github/workflows/deploy.yml`) always builds `main` HEAD and runs
migrations first, so re-triggering it unmodified cannot pin old code. Use the previous image tag.

1. Find the last good deploy: GitHub Actions → Deploy run that succeeded, or GHCR package page
   for `ghcr.io/luzin7/vozzera-backend`. Copy its `sha-<short>` tag (short SHA, 7 chars).
2. Image-backed Render service: Render Dashboard → the API service → **Manual Deploy** →
   **Deploy an existing image** → `ghcr.io/luzin7/vozzera-backend:sha-<short>`.
3. Git-backed Render service: redeploy the old commit instead — either the dashboard's
   **Rollback** on the previous deploy, or the hook with the full commit SHA:
   ```bash
   set -euo pipefail
   curl -fsS -X POST --max-time 30 "${RENDER_DEPLOY_HOOK_URL}&ref=<full-commit-sha>"
   ```
   This pins the repository commit, not the GHCR tag; the image is rebuilt from that commit.
4. If Render cannot pull the tag, make the GHCR package readable by the Render registry
   credentials, or add a registry credential in Render with a token that can read the package.
5. Verify (below). Do not roll back the database for an app-only problem.

## Mailjet release: deploy order and rollback

The transport moved from SendGrid to the Mailjet HTTP v3.1 API. The environment names changed and
production boot validates the new ones, so the order of operations matters: environment first,
merge second.

### Deploy order (mandatory)

Set these in the Render service environment **before** merging to `main`:

| Variable | Purpose |
| --- | --- |
| `MAILJET_API_KEY` | Mailjet API key, sent as the basic auth user |
| `MAILJET_API_SECRET` | Mailjet API secret, sent as the basic auth password |
| `MAIL_FROM_ADDRESS` | sender address on every message |
| `APP_URL` | base URL used to build the password-reset link |

`validateBootConfig` in `cmd/api/main.go` calls `ValidateMail` when the environment is production
and exits through `log.Fatalf` if any of the four is empty
(`internal/shared/config/config.go`). Merging first therefore produces a crash loop: the new
instance never binds, the previous one keeps serving, and the release silently does not land. Set
the four variables, then merge.

Before the first send, confirm in the Mailjet dashboard that the sender for `MAIL_FROM_ADDRESS` is
validated. An unvalidated sender is rejected at the message level, not at the transport level.

### Env rename

`SENDGRID_API_KEY` is replaced by `MAILJET_API_KEY` + `MAILJET_API_SECRET`. The new code does not
read `SENDGRID_API_KEY` at all, so leaving it in place is harmless while a rollback is still
plausible.

Keep `SENDGRID_API_KEY` in the Render environment while a rollback to a pre-Mailjet image is still
plausible. Remove it only after a Mailjet send has been confirmed (verification below).

### Rollback after the rename

An image built before the rename validates `SENDGRID_API_KEY` at boot (`ValidateMail` fails with
`ErrSendGridAPIKeyRequired`, also in `internal/shared/config/config.go`). Rolling back to such an
image requires `SENDGRID_API_KEY` to still exist in the Render environment; without it the old
image exits at boot instead of serving traffic. Re-add the value first, then follow **App
rollback** above.

### Post-deploy verification

1. Trigger one real password reset for a mailbox you control.
2. `curl -fsS https://vozzera.onrender.com/api/health/deep` returns 200 with `"mail":"ok"`.
   `"unknown"` (no send in the last hour) also returns 200 and proves nothing about the transport,
   so the send in step 1 is what makes this check meaningful.
3. The message appears in the Mailjet dashboard as a delivered message.

`/api/health` cannot prove mail works: it always returns 200 regardless of the `mail` field, and
`.github/workflows/keep-alive.yml` pings only that shallow path. An external monitor must target
`/api/health/deep`.

### Known limits of this transport

- No retry. A failed send is dropped, not retried. `POST /api/forgot-password` returns 200 to the
  user whether the send succeeded or not, so the HTTP response never distinguishes the two.
- Queue full (64 slots) drops the job without retry and logs
  `email.dispatch.rejected level=error reason=queue_full`.
- Shutdown drains for 5 seconds; jobs still queued after that are dropped and logged as
  `email.dispatch.drain_timeout level=error undelivered=<n>`.
- A single message-level rejection (HTTP 200 from Mailjet with `Status:"error"`, for example one
  rejected recipient) is recorded as a failure: the status code carried by the error is used when
  present, `400` otherwise, and `/api/health/deep` reports `failing` with 503 for up to one hour
  (`healthMaxAge`). The dispatch log line carries the real `provider_status` and response body, so
  read it before concluding the API key is wrong.

## Migration rollback (bad migration)

Discover the current version and the target version first:

```bash
export DATABASE_URL='postgres://user:password@host/db?sslmode=require'   # Neon connection string
goose -dir sql/migrations postgres "$DATABASE_URL" status
psql "$DATABASE_URL" -tAc "SELECT version_id FROM goose_db_version ORDER BY id DESC LIMIT 1"
```

The version is also the numeric prefix of the migration filename, for example
`20261105120000_add_messages_room_created_at_id_index.sql` → `20261105120000`.

Revert one migration:

```bash
goose -dir sql/migrations postgres "$DATABASE_URL" down
```

Revert everything newer than a known-good version (the target stays applied):

```bash
goose -dir sql/migrations postgres "$DATABASE_URL" down-to 20260817120000
goose -dir sql/migrations postgres "$DATABASE_URL" status
```

Local convenience: `make migrate-down-to version=20260817120000` (uses `DB_URL`, default
`postgres://admin:admin@localhost:5432/vozzera?sslmode=disable`; override with
`make migrate-down-to version=... DB_URL=...`).

Rules:

- Dump before reverting: `down` drops columns and tables and cannot be undone. Take a `pg_dump`
  to a local file first; there is no scheduled off-site dump yet (see the appendix).
- `goose down-to` is exclusive of the target: migrations after the target are reverted, the
  target remains applied.
- The `-- +goose NO TRANSACTION` migration (`20261105120000`, `CREATE INDEX CONCURRENTLY`) is
  safe to revert with `goose down`; do not wrap it in a transaction manually.
- After reverting the schema, redeploy the app image that matches the older schema. An app
  built for the newer schema will fail on missing columns.
- Re-apply with `goose -dir sql/migrations postgres "$DATABASE_URL" up`.

## Neon PITR (branch restore)

Neon restores by creating a new branch at a past point in time; it does not edit the current
branch in place. Retention is plan-dependent — the Free plan keeps a short window (6 hours at
the time of writing). Check the actual window for the project in the Neon Console before relying
on it: **Project → Settings → Billing / Plan**. Past that window there is no off-site dump to
fall back on (see the appendix).

1. Neon Console → project → **Branches** → **Restore** (or **Create branch → From history**).
2. Choose the source branch (`main`/`production`) and the timestamp in UTC, at least one minute
   before the damaging change. The console shows times in UTC; state the time as UTC in the
   incident notes.
3. Create the branch with a distinct name, for example `restore-20261109T1530Z`.
4. Read the new branch connection string. Point the API at it (Render env var `DATABASE_URL`),
   or promote it to primary if it fully replaces production.
5. Verify before pointing the app at it: connect with `psql` and check the schema and the goose
   version. The restored branch includes the `goose_db_version` table as of that moment.
6. Do not run `goose up` on the restored branch until the app is confirmed healthy; migrations
   were already applied by whatever ran before the restore point.

The restored branch is a separate database. Pointing the app at it is a Render environment
change plus a redeploy; it is not automatic.

## Decision table

| Symptom | First action | Command / place | Verify |
| --- | --- | --- | --- |
| Bad app deploy, schema untouched | Roll back the image | Render → Manual Deploy → `sha-<short>` | `curl -f https://vozzera.onrender.com/api/health` (always 200, even when mail is failing), login, one room |
| Bad migration, app still running | Stop deploy, dump, revert migration | `goose ... down-to <version>` | `goose status`, `\d <table>` in `psql` |
| Bad migration plus broken app | Revert migration, then redeploy the matching old image | `goose down-to` + Render image tag | health check, one write path |
| Data loss within the Neon window | Restore a PITR branch | Neon Console → Branches → Restore | row counts, goose version, app pointed at branch |
| Data loss older than the Neon window | No off-site dump exists yet (appendix): treat the data as unrecoverable and escalate to the owner | Neon Console → Branches (check the window) | row counts on the PITR branch while still inside the window |
| Migration fails during deploy (migrate job red) | Do not retry the deploy; inspect the failing SQL, take a dump, fix forward or revert | Actions log, then `goose status` | `goose status`, app still on previous image |

## Verify after each path

1. `curl -fsS https://vozzera.onrender.com/api/health` returns 200. This shallow endpoint is the
   deploy/rollback gate and the keep-alive target; it always returns 200. Its body reports
   `"mail": "ok" | "failing" | "unknown" | "disabled"` from the last send outcome (`unknown`
   means no send in the last hour).
2. `curl -fsS https://vozzera.onrender.com/api/health/deep` returns 503 when the last send failed
   or mail is disabled/unconfigured, 200 otherwise. This is the monitoring target for mail, not
   the Render health check path.
3. Log in and read one room's messages (exercises `users`, `rooms`, `messages`).
4. Write path: send and delete a message in a scratch room.
5. `goose -dir sql/migrations postgres "$DATABASE_URL" status` shows the expected version and
   no `Pending` rows behind an applied newer version.
6. For a restore, compare `users` and `messages` counts with the incident's expected values.

"Deployed" is not "works": only the checks above close the incident.

## Secrets the owner must create

Repository secrets (Settings → Secrets and variables → Actions), all consumed by `deploy.yml`:

| Name | Purpose |
| --- | --- |
| `DATABASE_URL` | Neon connection string; the migrate job applies `goose up` to it |
| `RENDER_DEPLOY_HOOK_URL` | Render deploy hook used by the deploy job |

## Appendix: off-site backups (not configured yet)

Off-site database backups are deliberately deferred. Nothing dumps the database today: there is
no backup workflow, no bucket, no retention rule and no restore-from-object-storage procedure in
this repository. Until one exists, recovery is bounded by the Neon PITR window above, and a loss
older than that window is not recoverable.

Cloudflare R2 is the intended object store for user media and files when those features land. It
is not configured today, and it is not a target for database dumps: no bucket, credential or
lifecycle rule exists, and this runbook makes no assumption that R2 will hold dumps.
