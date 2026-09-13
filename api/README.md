# Stashly API

A small file storage service written in Go on top of the Goravel framework. It
accepts uploads from external applications, keeps them organised per client and
per folder, and can package whole folders into a zip file in the background.

The service exposes two separate surfaces:

* **Client API** (`/api/...`) — used by the applications that store files. Every
  request carries the client credentials in headers and everything it can see is
  scoped to that one client.
* **Admin API** (`/admin/...`) — used by the web application. It authenticates
  with a JWT bearer token and can see every client, file, archive and audit log
  entry.

The complete endpoint list, request bodies and response shapes live in
`../API_CONTRACT.md`.

## How it works

Uploaded files are written to one of two disks depending on their visibility:

```
storage/app/public/{client_id}/{folder}/{YYYY}/{MM}/{uuid}.{ext}
storage/app/private/{client_id}/{folder}/{YYYY}/{MM}/{uuid}.{ext}
storage/app/archives/{archive_id}-{uuid}.zip
```

Only `storage/app/public` is served by the static `/storage` route. Private
files can be read through the download endpoints, which check ownership first.
Generated zip files are never served statically either.

The MIME type of an upload is detected from the first 512 bytes of the file
rather than trusted from the request, and the SHA-256 checksum is computed while
the file is streamed to disk, so nothing large is ever held in memory.

Archives are built by the `zip_folders` queue job. It streams every matching
file into the zip, updates the progress column as it goes, and finally records
the size, the path and an expiry date. If the archive request carried a
`callback_url`, the finished archive is POSTed to it (10 second timeout, three
attempts, failures are only logged).

Folder names are limited to `^[a-z0-9_-]+(/[a-z0-9_-]+){0,2}$`, so at most three
levels, no uppercase, no dots and no slashes at either end. Every path is joined
with `filepath.Join` and verified to stay inside its disk root before any read
or delete happens.

## Requirements

* Go 1.25 or newer
* PostgreSQL 14 or newer

## Setup

```shell
cp .env.example .env
# fill in the database credentials, then:
./artisan key:generate
./artisan jwt:secret
go run . artisan migrate
go run . artisan db:seed
go run .
```

The server listens on `APP_HOST:APP_PORT` (127.0.0.1:3000 by default). The same
process also runs the queue worker and the scheduler, so no extra command is
needed for background work.

The seeders create the two roles, one super admin user
(`stashly@example.com`, password from `SEED_ADMIN_PASSWORD`, `ChangeMe123!` when
that variable is unset) and one demo client (username `demo`, password
`demo12345`, 1 GB quota) owned by that user. Re-running the seeders also removes
the phone based scaffold account the project used to ship.

## Roles and ownership

There are exactly two roles:

* `super_admin` — reaches every endpoint and sees every client, file, archive,
  archive and audit log entry. Only this role manages panel accounts
  (`/admin/users`).
* `admin` — reaches every endpoint except `/admin/users` and
  `POST /admin/storage/sync`, and only sees the clients it owns
  (`clients.owner_id = user.id`) together with their files, folders, archives
  and audit entries. Anything owned by somebody else answers 404, never 403, so
  ids stay unguessable. `owner_id` in a client payload is ignored for this role
  and forced to the caller; a super admin may set it to any active user.

A panel account is invited, never created with a password: `POST /admin/users`
stores it with status `pending` and mails a six digit code. The invited user
calls `POST /admin/auth/verify` with that code and the password it chooses,
which activates the account. `blocked` accounts are refused at login and by the
`can_user` middleware, so blocking takes effect even for a token that was
already handed out.

## Mail and one time codes

Codes are six digits, valid for ten minutes, single use, invalidated after five
wrong attempts, and a new one can only be requested 60 seconds after the last.
Only an HMAC-SHA256 of the code (keyed with `APP_KEY` and bound to the address
and the purpose) is stored in `otp_codes`, so neither a database dump nor the
log of a query reveals a usable code.

`MAIL_MAILER` picks the transport:

* `log` (default) renders the message and writes it to the application log,
  which is how local development and the test suite read the code. Nothing
  leaves the machine.
* `smtp` sends through the Goravel mail facade. The framework derives the
  transport from the port — 465 is implicit TLS, 587 is STARTTLS, anything else
  is plain — so `MAIL_ENCRYPTION` is checked against `MAIL_PORT` before a
  message is handed over and a contradicting pair is refused rather than sent
  in the clear.

The message bodies live in `app/mail` as Go templates (plain text plus a simple
HTML part, English, subject `Your Stashly verification code`).

## Environment variables

| Variable | Meaning |
|---|---|
| `APP_NAME` | Application name, also used as the cache key prefix. |
| `APP_ENV` | Environment name, for example `local` or `production`. |
| `APP_KEY` | 32 character encryption key, generated by `artisan key:generate`. |
| `APP_DEBUG` | Enables verbose framework output. |
| `APP_URL` | Public base URL, used to build file and archive URLs. |
| `APP_HOST` | Address the HTTP server binds to. |
| `APP_PORT` | Port the HTTP server binds to. |
| `LOG_CHANNEL` | Log channel, `stack` by default. |
| `LOG_LEVEL` | Lowest level written to the log files. |
| `JWT_SECRET` | Signing key for admin tokens, generated by `artisan jwt:secret`. |
| `JWT_TTL` | Token lifetime in minutes. |
| `JWT_REFRESH_TTL` | How long a token may still be refreshed, in minutes. |
| `DB_CONNECTION` | Database driver, `postgres`. |
| `DB_HOST`, `DB_PORT`, `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD`, `DB_SCHEMA` | Database connection details. |
| `QUEUE_CONNECTION` | `database` in normal use, `sync` to run jobs in line. |
| `MAX_FILE_SIZE` | Largest single upload in bytes, 52428800 (50 MB) by default. A client may lower this for itself. |
| `ARCHIVE_TTL_DAYS` | How many days a finished archive stays downloadable, 5 by default. |
| `UPLOAD_RATE_PER_MINUTE` | Uploads allowed per minute and per client, 60 by default. |
| `CORS_ALLOWED_ORIGINS` | Comma separated list of browser origins allowed by CORS, `*` by default. |
| `MAIL_MAILER` | `log` (write the message to the log) or `smtp`. |
| `MAIL_HOST`, `MAIL_PORT` | SMTP server. The port decides the transport: 465 implicit TLS, 587 STARTTLS, otherwise plain. |
| `MAIL_USERNAME`, `MAIL_PASSWORD` | SMTP credentials. |
| `MAIL_ENCRYPTION` | `tls`, `ssl` or `none`. Checked against `MAIL_PORT`; a contradicting pair is refused. |
| `MAIL_FROM_ADDRESS`, `MAIL_FROM_NAME` | Sender of every message. |
| `SEED_ADMIN_PASSWORD` | Password the seeder gives the super admin, `ChangeMe123!` by default. |

### Upload size limits

Gin's `body_limit` is not a request size cap: the driver feeds it to
`MaxMultipartMemory`, so it only decides how much of a multipart body the
parser keeps in RAM before spilling to a temp file.

A Goravel middleware cannot cap an upload either. `goravel/gin` turns every
Goravel middleware into a gin handler that first builds a `Context`, and
building the request side of that context parses the multipart form. The body
is therefore already read, and already spilled to temp files, before the first
`Handle()` runs.

The cap is enforced in three places instead:

1. **Before anything is parsed.** `app/http/server` wraps the gin route in a
   plain `net/http` handler that runs above the whole Goravel stack. It rejects
   a `Content-Length` over the process wide hard cap with 413
   `{"message": "file too large"}` and wraps the body in
   `http.MaxBytesReader`, so a lying or chunked request is stopped at the same
   byte count. The hard cap is
   `MAX_FILE_SIZE * max_files_per_request + 1 MB` and is computed from the
   configuration only, because this runs before authentication.
2. **Per client.** The `limit_upload_size` middleware applies
   `(client.max_file_size ?? MAX_FILE_SIZE) * max_files_per_request + 1 MB` to
   the authenticated caller, and turns a body that already tripped the hard cap
   into a clean 413.
3. **Per file.** `UploadService` checks every part against the client limit
   using its multipart header size, before the part is copied anywhere, and
   again against the size that actually landed.

A reverse proxy in front of the service should still set its own limit, both
because it rejects the request one hop earlier and because it is the only thing
that protects the process if it is ever run behind a proxy that buffers:

```nginx
client_max_body_size 1024m;   # >= MAX_FILE_SIZE * max_files_per_request
```

### Static files

The public disk is served under `/storage` with `X-Content-Type-Options:
nosniff`. Anything whose type can execute in a browser (`text/html`,
`application/xhtml+xml`, `image/svg+xml`, `text/xml`, `application/xml`) is
additionally forced to download with `Content-Disposition: attachment` and
sandboxed with `Content-Security-Policy: sandbox`, so an uploaded page cannot
script this origin. Everything else stays `inline`.

## Console commands

```shell
go run . artisan archives:cleanup   # delete expired zip files, mark them expired
go run . artisan storage:sync       # reconcile the files table with the disks
```

`archives:cleanup` also runs automatically every hour through the scheduler.
When the server boots, any archive left in the `processing` state by a previous
crash is reset to `pending` and queued again.

## Tests

```shell
go test ./...
```

The feature tests exercise the real route stack and need the database from
`.env` to be reachable. They create their own throw away clients and clean up
after themselves.

## Project layout

```
app/console/commands   Artisan commands
app/http/controllers   api/ for the client surface, admin/ for the web app
app/http/middleware    api_check, can_user, jwt_auth
app/http/requests      form request structs used for validation
app/http/responses     shared JSON envelopes and pagination helpers
app/jobs               the zip_folders queue job
app/models             User, Role, Client, Device, File, Archive, AuditLog
app/resources          model to JSON transformers
app/services           storage, upload, archive, client, audit and sync logic
bootstrap              application wiring, migrations, seeders, schedule
config                 one file per concern
database               migrations and seeders
routes                 the whole route table
```
