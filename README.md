<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo-horizontal-dark.svg">
    <img src="assets/logo-horizontal.svg" alt="Stashly" width="360">
  </picture>
</p>

<p align="center">
  A small, self-hosted file storage service with a REST API and an admin panel.
</p>

<p align="center">
  <a href="LICENSE"><img alt="MIT" src="https://img.shields.io/badge/license-MIT-14b8a6.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white">
  <img alt="Nuxt" src="https://img.shields.io/badge/Nuxt-4-00DC82?logo=nuxt&logoColor=white">
  <img alt="PostgreSQL" src="https://img.shields.io/badge/PostgreSQL-14%2B-4169E1?logo=postgresql&logoColor=white">
</p>

Applications upload files through a simple HTTP API. Each application is a
**client** with its own credentials, quota, allowed file types and size limit.
An administrator manages clients, browses files, and can package any folders
into a ZIP archive that is built in the background and kept for a few days.

No external object storage is required. Files live on the local disk of the
server, metadata lives in PostgreSQL.

```
┌──────────────────┐      X-Username / X-Password      ┌──────────────────────┐
│  Your services   │ ────────── /api/... ────────────▶ │                      │
│ (backend, mobile)│                                   │   API  (Go/Goravel)  │──▶ PostgreSQL
└──────────────────┘                                   │                      │
                                                       │  • uploads, quotas   │──▶ storage/app/
┌──────────────────┐        JWT bearer token           │  • zip jobs (queue)  │      public/
│   Admin panel    │ ────────── /admin/... ──────────▶ │  • hourly cleanup    │      private/
│  (Nuxt 4 + Naive)│                                   │  • audit log         │      archives/
└──────────────────┘                                   └──────────────────────┘
```

## Features

- **Single and multiple upload** — up to 20 files per request, per-file
  validation, the whole request is rejected with per-file errors if any file
  fails.
- **Public and private files** — public files are served from `/storage/...`,
  private files only through an authenticated download endpoint.
- **Per-client limits** — storage quota, maximum file size, allowed MIME types
  (`image/*` style wildcards). MIME is detected from the file content, never
  trusted from the request.
- **Folders** — up to three levels, listed with file counts and sizes.
- **ZIP archives** — pick folders (or the whole client), the zip is streamed in
  a background job, progress is reported, an optional webhook is called when it
  is ready, and the file expires automatically after a configurable number of
  days.
- **Admin panel** — dashboard, client management with one-time password reveal,
  file browser with upload and bulk delete, archive builder with live status,
  audit log, storage reconciliation. Mobile friendly, dark mode, three
  languages (uz / ru / en).
- **Audit log** — every login, upload, delete, client change and archive
  request is recorded with actor and IP.
- **Hardening** — path traversal checks on every disk access, request body size
  cap enforced before the body is parsed, SSRF protection for webhooks, rate
  limits on uploads and on admin login, HTML/SVG uploads served as attachments
  with a sandboxed CSP.

## Repository layout

```
assets/     Logo files (SVG/PNG)
api/        Go backend (Goravel v1.18, gin, PostgreSQL)
web-app/    Admin panel (Nuxt 4, Naive UI, Tailwind CSS 4)
API_CONTRACT.md         Every endpoint, request and response shape
INTEGRATION_PROMPT.md   Ready-to-paste guide for teams integrating the client API
```

Each part has its own README with the full list of environment variables.

## Quick start

### Requirements

- Go 1.25+
- Node.js 20+
- PostgreSQL 14+

### 1. Database

```bash
createdb stashly
# or with docker:
docker run -d --name pgsql -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=secret \
  -e POSTGRES_DB=stashly -p 5432:5432 postgres:17-alpine
```

### 2. API

```bash
cd api
cp .env.example .env          # fill in DB_USERNAME / DB_PASSWORD
./artisan key:generate
./artisan jwt:secret
go run . artisan migrate
go run . artisan db:seed      # creates the first admin user and a demo client
go run .                      # http://127.0.0.1:3000
```

The API process also runs the queue worker and the scheduler, so a single
binary is enough.

Seeded accounts (change them before going live):

| Account | Login | Password |
|---|---|---|
| Super admin | `stashly@example.com` | `SEED_ADMIN_PASSWORD` from `.env` (default `ChangeMe123!`, local only) |
| Demo client | `demo` | `demo12345` |

### 3. Admin panel

```bash
cd web-app
cp .env.example .env          # NUXT_PUBLIC_API_BASE=http://127.0.0.1:3000
npm install
npm run dev                   # http://localhost:3000 (pick another port if the API uses it)
```

Production build:

```bash
npm run build
PORT=3003 node .output/server/index.mjs
```

## Using the client API

Every request carries the client credentials and a device identifier in
headers:

```
X-Username: demo
X-Password: demo12345
X-Language: uz            # uz | ru | en
X-Device:   backend-1     # any stable identifier of the calling instance
```

Upload two files into a folder as private:

```bash
curl -X POST http://127.0.0.1:3000/api/files \
  -H "X-Username: demo" -H "X-Password: demo12345" \
  -H "X-Language: uz" -H "X-Device: backend-1" \
  -F "files[]=@report.pdf" -F "files[]=@photo.png" \
  -F "folder=docs/2026" -F "visibility=private"
```

```json
{
  "data": [
    {
      "id": 42,
      "folder": "docs/2026",
      "name": "b3f1c2…9a.pdf",
      "original_name": "report.pdf",
      "path": "1/docs/2026/2026/09/b3f1c2…9a.pdf",
      "url": null,
      "mime": "application/pdf",
      "extension": "pdf",
      "size": 123456,
      "sha256": "…",
      "visibility": "private",
      "created_at": "2026-09-12T18:00:00Z"
    }
  ]
}
```

Build a ZIP of a folder and get notified when it is ready:

```bash
curl -X POST http://127.0.0.1:3000/api/archives \
  -H "X-Username: demo" -H "X-Password: demo12345" \
  -H "X-Language: uz" -H "X-Device: backend-1" \
  -H "Content-Type: application/json" \
  -d '{"folders":["docs/2026"],"callback_url":"https://example.com/hook"}'
```

The response is `202 Accepted` with `status: "pending"`. Poll
`GET /api/archives/{id}` or wait for the webhook; when the status is `done`,
`GET /api/archives/{id}/download` returns the zip until `expires_at`.

| Method | Endpoint | Purpose |
|---|---|---|
| `POST` | `/api/files` | Upload one (`file`) or many (`files[]`) files |
| `GET` | `/api/files` | List files (`folder`, `search`, `page`, `per_page`) |
| `GET` | `/api/files/{id}` | File metadata |
| `GET` | `/api/files/{id}/download` | Download (works for private files) |
| `DELETE` | `/api/files/{id}` | Delete a file |
| `GET` | `/api/folders` | List folders (`parent`) |
| `POST` | `/api/archives` | Queue a ZIP of folders |
| `GET` | `/api/archives`, `/api/archives/{id}` | Archive list / status |
| `GET` | `/api/archives/{id}/download` | Download the ZIP |
| `DELETE` | `/api/archives/{id}` | Delete an archive |
| `GET` | `/api/me` | Client info, quota and usage |

Error responses always look like
`{ "message": "...", "errors": { "field": ["..."] } }`. Status codes:
`401` bad credentials, `403` blocked client, `404` not found (including other
clients' files), `413` too large or quota exceeded, `415` MIME not allowed,
`422` validation, `429` rate limited.

The full contract, including the admin API, is in
[API_CONTRACT.md](API_CONTRACT.md). A copy-paste guide for integrating teams is
in [INTEGRATION_PROMPT.md](INTEGRATION_PROMPT.md).

## Admin panel

Sign in with your email address and password. The panel talks to the
`/admin/...` API with a JWT and offers:

- **Dashboard** — totals, usage per client, uploads over the last 30 days.
- **Clients** — create, edit, block, reset password (shown once), quota and
  limits, connected devices, delete with optional purge of files.
- **Files** — browse per client and folder, filter by visibility and MIME,
  preview images, upload, download, bulk delete.
- **Archives** — pick folders in a tree, watch progress, download, delete.
- **Users** (super admin) — invite admins by email; they receive a one-time
  code, set their password and activate the account. Every admin sees only the
  clients it owns; a super admin sees everything.
- **Profile** — change your name, password, or email (confirmed with a code
  sent to the new address). Forgot-password works the same way.
- **Audit logs** — who did what, when, from where.
- **Storage sync** (super admin) — reconcile the database with what is on disk.

## Background work

| Task | When | What it does |
|---|---|---|
| `zip_folders` job | on every archive request | Streams the selected files into a zip, updates progress, sets `expires_at`, calls the webhook |
| `archives:cleanup` | hourly (scheduler) | Deletes zip files past `expires_at`, marks them `expired` |
| boot recovery | on start | Re-queues archives left in `processing` or stuck in `pending` |
| `storage:sync` | manual (`artisan` or admin panel) | Removes records without files and files without records |

## Configuration

The most relevant API variables (see `api/.env.example` for all of them):

| Variable | Default | Meaning |
|---|---|---|
| `MAX_FILE_SIZE` | `52428800` | Largest single upload in bytes (50 MB) |
| `ARCHIVE_TTL_DAYS` | `5` | Days a finished archive stays downloadable |
| `UPLOAD_RATE_PER_MINUTE` | `60` | Uploads per minute per client |
| `CORS_ALLOWED_ORIGINS` | `*` | Comma-separated browser origins allowed to call the API |
| `QUEUE_CONNECTION` | `database` | Queue driver; keep `database` so archives build in the background |
| `MAIL_MAILER` | `smtp` | `smtp` for real mail (`MAIL_HOST`, `MAIL_PORT`, `MAIL_USERNAME`, `MAIL_PASSWORD`, `MAIL_ENCRYPTION`, `MAIL_FROM_ADDRESS`); `log` writes messages to the log, allowed only in `local`/`testing` |
| `SEED_ADMIN_PASSWORD` | `ChangeMe123!` | Password of the seeded super admin; the default is refused outside `local`/`testing` |

Admin panel variables (`web-app/.env.example`):

| Variable | Default | Meaning |
|---|---|---|
| `NUXT_PUBLIC_API_BASE` | `http://127.0.0.1:3000` | Where the API is reachable from the browser |
| `NUXT_PUBLIC_COOKIE_SECURE` | `false` | Set `true` only behind HTTPS, otherwise the login cookie is dropped |

## Deployment notes

- Put a reverse proxy (nginx, Caddy) in front of the API and set its own body
  limit, for example `client_max_body_size 1024m;` (at least
  `MAX_FILE_SIZE × 20`).
- Mount `api/storage` on persistent disk; it holds every uploaded file and zip.
- Set `CORS_ALLOWED_ORIGINS` to the admin panel origin and
  `NUXT_PUBLIC_COOKIE_SECURE=true` once TLS is in place.
- Set `SEED_ADMIN_PASSWORD` and real SMTP settings before the first seed; delete the demo client.

## Development

```bash
# API
cd api && go test ./...

# Admin panel
cd web-app && npx vue-tsc --noEmit && npm test && npm run build
```

## Contributing

Issues and pull requests are welcome. Please keep the API contract file in sync
with any endpoint change and add a feature test for backend behaviour changes.

## License

MIT. See `LICENSE`.
