# Stashly — API Contract

Base URL: `http://127.0.0.1:3000` (env `APP_URL`). All JSON. Timestamps RFC3339 (UTC).

Two auth surfaces:

1. **Client API** (`/api/...`) — for external apps uploading files. Middleware `api_check` (existing):
   headers `X-Username`, `X-Password`, `X-Language` (uz|ru|en), `X-Device` required. Optional `X-Platform`, `X-App-Version`.
   Sets `client_id` in context. All file/archive ops scoped to that client.
2. **Admin API** (`/admin/...`) — for the Nuxt web-app. JWT (`Authorization: Bearer <token>`), guard `user`,
   middleware `can_user` (existing; super_admin passes everything, other roles by route name in `role.permissions`).

Error shape (all endpoints): `{ "message": "...", "errors": { "field": ["msg"] } }` (errors only on 422).

Pagination shape: `{ "data": [...], "meta": { "page": 1, "per_page": 20, "total": 123, "last_page": 7 } }`.
Query params: `page`, `per_page` (max 100), `search`, `sort` (e.g. `-created_at`).

---

## Models

### File
```json
{
  "id": 1,
  "client_id": 3,
  "folder": "avatars",              // "" = root. Only [a-z0-9/_-], no "..", max 3 levels
  "name": "b3f1c2...9a.pdf",        // uuid + ext, unique on disk
  "original_name": "hisobot.pdf",
  "path": "3/avatars/2026/09/b3f1c2...9a.pdf",   // relative to disk root; first segment = client_id
  "url": "http://127.0.0.1:3000/storage/3/avatars/2026/09/b3f1c2...9a.pdf",  // null when private
  "mime": "application/pdf",
  "extension": "pdf",
  "size": 123456,
  "sha256": "…64 hex…",
  "visibility": "public",           // public | private
  "created_at": "2026-09-12T18:00:00Z"
}
```
Disk layout: public → `storage/app/public/{client_id}/{folder}/{YYYY}/{MM}/{uuid}.{ext}`,
private → `storage/app/private/{client_id}/{folder}/{YYYY}/{MM}/{uuid}.{ext}`.
`/storage/*` static serves only `storage/app/public`.

### Archive
```json
{
  "id": 7,
  "client_id": 3,                   // client whose folders are zipped
  "created_by": { "type": "user", "id": 1 } | { "type": "client", "id": 3 },
  "folders": ["avatars", "docs/2026"],   // [] or ["*"] = whole client root
  "status": "pending",              // pending | processing | done | failed | expired
  "progress": 42,                   // 0..100
  "files_count": 120,
  "size": 98765432,                 // zip bytes, null until done
  "path": "archives/7-…uuid.zip",   // null until done
  "url": "http://…/admin/archives/7/download" (admin) | "http://…/api/archives/7/download" (client); null until done
  "error": null,
  "callback_url": null,             // optional webhook, POSTed archive JSON on done/failed
  "expires_at": "2026-09-17T18:00:00Z",   // created_at + ARCHIVE_TTL_DAYS (default 5); null until done
  "created_at": "…",
  "finished_at": null
}
```

### Client
```json
{
  "id": 3, "name": "Mobile app", "username": "mobile", "status": "active",   // active|inactive|blocked
  "quota_bytes": 10737418240,       // null = unlimited
  "used_bytes": 123456789,          // sum of files.size (computed)
  "files_count": 321,
  "allowed_mimes": ["image/*", "application/pdf"],  // [] = all. Supports "type/*" wildcard
  "max_file_size": 52428800,        // null = use env MAX_FILE_SIZE
  "owner_id": 1, "owner": { "id": 1, "name": "Admin", "email": "…" },   // the admin who manages this client
  "creator_id": 1, "updater_id": 1,
  "created_at": "…", "updated_at": "…"
}
```
Password never returned. On create/reset the plain password is returned ONCE in the response (`"password": "..."`).

### Device (existing) — `{ id, uid, platform, app_version, ip, last_seen_at, client_id, created_at }`

### User (admin panel account)
```json
{ "id": 1, "name": "Admin", "email": "stashly@example.com", "avatar_src": null,
  "status": "active",            // pending (OTP not verified yet) | active | blocked
  "role": { "id": 1, "name": "Super Admin", "slug": "super_admin", "permissions": [] },
  "clients_count": 3, "last_login_at": "…", "created_at": "…" }
```
No phone number anywhere. Roles: `super_admin` (everything, manages users, sees all clients) and
`admin` (only clients it owns: `clients.owner_id = user.id`; cannot manage users).

### AuditLog
`{ id, actor: {type: "user"|"client", id, name}, action, subject_type, subject_id, client_id, details: {}, ip, created_at }`

`client_id` is the client an entry belongs to (null when there is none). It is what the audit listing is
scoped by for the `admin` role.
Actions: `client.create|update|delete|reset_password`, `file.upload|delete`, `archive.create|delete`, `auth.login`,
`user.create|update|delete|verify|block`, `user.email_change`, `user.password_change`, `otp.send`.

---

## Client API — `/api` (api_check)

| Method | Path | Body / Query | Response |
|---|---|---|---|
| POST | `/api/files` | multipart: `file` (single) **or** `files[]` (multiple, max 20); optional `folder`, `visibility` (public default) | 201 `{ "data": [File, …] }` always array |
| GET | `/api/files` | `folder`, `page`, `per_page`, `search` | paginated File |
| GET | `/api/files/{id}` | | `{ "data": File }` |
| GET | `/api/files/{id}/download` | | binary, `Content-Disposition: attachment; filename="original_name"` (works for private) |
| DELETE | `/api/files/{id}` | | 204 |
| GET | `/api/folders` | `parent` (optional) | `{ "data": [ { "name": "avatars", "path": "avatars", "files_count": 12, "size": 12345 } ] }` |
| POST | `/api/archives` | `{ "folders": ["a","b"], "callback_url": "https://…" }` | 202 `{ "data": Archive }` |
| GET | `/api/archives` | `page` | paginated Archive |
| GET | `/api/archives/{id}` | | `{ "data": Archive }` |
| GET | `/api/archives/{id}/download` | | binary zip (404 if not done / expired) |
| DELETE | `/api/archives/{id}` | | 204 (deletes zip file too) |
| GET | `/api/me` | | `{ "data": Client }` (with used_bytes, quota) |

Upload rules:
- 413 `quota exceeded` when `used_bytes + size > quota_bytes`.
- 413 `file too large` when size > client.max_file_size ?? env MAX_FILE_SIZE (default 50MB). Gin `body_limit` = MAX_FILE_SIZE * 20 + 1MB.
- 415 `mime not allowed` when client.allowed_mimes non-empty and no match. MIME detected from content (http.DetectContentType), not trusted from header.
- 422 invalid folder name.
- Multiple upload: validate all first; if any fails, reject whole request with per-file errors `{ "message": "...", "errors": { "files.2": ["mime not allowed"] } }`.
- Rate limit: 60 uploads / minute per client (Goravel RateLimiter), 429.

---

## Admin API — `/admin` (JWT + can_user; routes named `admin.<resource>.<action>`)

### Auth (public unless marked Bearer)
| Method | Path | Body | Response |
|---|---|---|---|
| POST | `/admin/auth/login` | `{ "email": "…", "password": "…" }` | `{ "token": "…", "user": User }`; 403 `{"message":"account not verified"}` when status pending, 403 when blocked. Rate limit 10/min per IP+email |
| POST | `/admin/auth/refresh` | (Bearer, expired ok) | `{ "token": "…" }` |
| POST | `/admin/auth/logout` | (Bearer) | 204 |
| GET | `/admin/auth/me` | (Bearer) | `{ "data": User }` |
| POST | `/admin/auth/verify` | `{ "email", "code", "password", "password_confirmation" }` | 200 `{ "token", "user" }` — first-time activation of an invited user (status pending → active) |
| POST | `/admin/auth/verify/resend` | `{ "email" }` | 204 always (no user enumeration). Rate limit 3/10min per email |
| POST | `/admin/auth/forgot` | `{ "email" }` | 204 always; sends OTP purpose `password_reset` |
| POST | `/admin/auth/reset` | `{ "email", "code", "password", "password_confirmation" }` | 204 |

### OTP rules
- 6 digits, valid 10 minutes, max 5 wrong attempts then invalidated, resend allowed after 60 s.
- Stored hashed in `otp_codes` (email, purpose: `verify|email_change|password_reset`, code_hash, expires_at, attempts, consumed_at). Only the latest code per (email, purpose) is valid.
- Sent through the Goravel mail facade; SMTP settings from `.env` (`MAIL_MAILER=smtp|log`, `MAIL_HOST`, `MAIL_PORT`, `MAIL_USERNAME`, `MAIL_PASSWORD`, `MAIL_ENCRYPTION=tls|ssl|none`, `MAIL_FROM_ADDRESS`, `MAIL_FROM_NAME`). With `MAIL_MAILER=log` the message body (including the code) is written to the log for local development.
- Email templates: plain text + simple HTML, English, subject `Your Stashly verification code`.
- Password policy for `verify`, `reset` and the profile password change: at least 8 characters, at least one
  letter and at least one digit, and `password_confirmation` has to match. A break answers
  422 `{"errors": {"password": ["..."]}}`.

---

## Deviations from this contract

The implementation follows the contract except for the points below.

1. **`verify/resend` and `forgot` answer 429 inside the 60 s resend gap**, not 204. Answering 204 there would
   turn both endpoints into an unmetered mail relay for any address. The gap is claimed per `(IP, address)`
   before the address is looked up, so an unknown address behaves exactly like a known one: 204 on the first
   call, 429 on a second call inside 60 s. That is deliberate — enforcing the gap only for addresses that
   exist would make the 204/429 difference an account enumeration oracle. The first call of any address still
   answers 204, so nothing is leaked about who has an account. On top of it both endpoints are throttled 3 per
   10 minutes per address and 10 per minute per IP.
2. **A wrong or expired code answers 422** with `{"errors": {"code": ["the code is invalid or has expired"]}}`
   rather than a bare message, so the panel can attach the error to its code field. Every rejection reason
   (no code, wrong code, expired, used, too many attempts) shares that one message.
3. **`POST /admin/storage/sync` is super_admin only.** It walks the whole disk and can delete rows of any
   client, so it is not covered by "admin reaches everything except users".
4. **`PUT /admin/users/{id}` cannot flip a `pending` user to `active`.** Activation happens through the code
   the invited user receives; a hand activated row would have no password and could never sign in.
5. **Self-protection and reassignment answer 422**, e.g. deleting yourself, blocking yourself, changing your
   own role, or deleting an owner without `reassign_to`.
6. **`MAIL_ENCRYPTION` is validated against `MAIL_PORT`.** The framework derives the transport from the port
   alone (465 implicit TLS, 587 STARTTLS, otherwise plain), so a pair that disagrees is refused instead of
   quietly sending in the clear.
7. **`clients.owner_id` is nullable.** A client whose owner is deleted without `reassign_to` cannot happen
   through the API, but the column stays nullable so the foreign key can be `null on delete` rather than
   cascading a user deletion into client rows.
8. **A code that could not be mailed answers 500 `{"message": "could not send email"}`.** The row is removed
   again, so the resend gap does not hold a code shut that nobody received, and the reason (SMTP host,
   credentials, transport errors) goes to the log only. `POST /admin/users` creates the account and its
   activation code in one transaction, so a delivery failure leaves no pending account whose address is taken
   and which nobody can ever activate.
9. **Tokens issued before a credential change are refused with 401.** goravel's JWT payload is fixed (key and
   subject), so nothing can be carried in the token; the token's `iat` claim is compared against
   `users.credentials_changed_at` instead. That timestamp is bumped on password reset, profile password
   change, email change, and when a user is blocked — so `PUT /admin/profile/password` and
   `POST /admin/profile/email/confirm` succeed and then invalidate the very token that made the call. The
   panel has to sign in again afterwards. The comparison is truncated to the second, so a token minted in the
   same second as the change is still accepted.
10. **`POST /admin/auth/refresh` answers 403 for a missing, deleted or non-active account**, and 401 for a
   token older than the account's last credential change. Refresh only reads the claims, so without this a
   blocked user could keep minting tokens for the whole refresh window.
11. **The last active super admin cannot be blocked, demoted or deleted**: 422
   `{"message": "at least one active super administrator is required"}`. A super admin never reaches this
   case (it would always leave itself behind); a custom role holding `admin.users.*` does.
12. **Additional throttles on the authenticated endpoints**, keyed by the authenticated user id and falling
   back to the IP: `POST /admin/profile/email` 5 per 10 minutes, `POST /admin/profile/email/confirm` 10 per
   minute, `POST /admin/auth/refresh` 30 per minute. All answer 429 `{"message": "too many attempts"}`.
13. **`MAIL_MAILER` defaults to `smtp` and `log` is refused unless `APP_ENV` is `local` or `testing`.** The
   log mailer writes one time codes into the application log; a deployment that inherits it would answer 204
   to every OTP endpoint while sending nothing. An unusable mail configuration, or an empty `APP_KEY`, stops
   the process at boot rather than at the first request.
14. **`DELETE /admin/clients/{id}/devices/{device_id}` writes a `client.device_delete` audit entry.**

### Profile (Bearer, any role)
| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/admin/profile` | | `{ "data": User }` |
| PUT | `/admin/profile` | `{ "name" }` | `{ "data": User }` |
| PUT | `/admin/profile/password` | `{ "current_password", "password", "password_confirmation" }` | 204; 422 when current password wrong |
| POST | `/admin/profile/email` | `{ "email", "current_password" }` | 204 — sends OTP to the NEW address (purpose `email_change`), 422 if email taken |
| POST | `/admin/profile/email/confirm` | `{ "code" }` | `{ "data": User }` — email updated |

### Users (Bearer, super_admin only; admin gets 403)
| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/admin/users` | pagination, `search`, `role`, `status` | paginated User |
| POST | `/admin/users` | `{ "name", "email", "role": "admin"\|"super_admin" }` | 201 `{ "data": User }` with status `pending`; OTP (purpose `verify`) emailed |
| GET | `/admin/users/{id}` | | `{ "data": User }` |
| PUT | `/admin/users/{id}` | `{ "name"?, "role"?, "status"?: "active"\|"blocked" }` | `{ "data": User }`; cannot block/demote yourself |
| POST | `/admin/users/{id}/resend-otp` | | 204 (only when pending) |
| DELETE | `/admin/users/{id}` | `?reassign_to=<user_id>` required when the user owns clients | 204; cannot delete yourself |

### Ownership scoping (applies to every /admin list/show/mutate of clients, files, folders, archives, audit logs, dashboard)
- `super_admin`: no filter. Client create/update accept `owner_id` (defaults to self).
- `admin`: only rows whose client `owner_id` = current user (audit logs: actor is self or subject belongs to own clients). Other rows → 404. `owner_id` in payloads is ignored (forced to self).
- Route names stay `admin.<resource>.<action>`; `can_user` grants `super_admin` everything and `admin` everything except `admin.users.*`.

### Dashboard
| GET | `/admin/dashboard` | | scoped to the caller's clients for `admin`. `{ "data": { "clients_count", "files_count", "total_size", "archives_pending", "by_client": [ { "client_id", "name", "files_count", "size" } ], "uploads_last_30_days": [ { "date": "2026-09-01", "count", "size" } ] } }` |

### Clients
| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/admin/clients` | pagination, `search`, `status` | paginated Client |
| POST | `/admin/clients` | `{ name, username, password?, status?, quota_bytes?, allowed_mimes?, max_file_size?, owner_id? (super_admin only) }` (password auto-generated if omitted, 16 chars) | 201 `{ "data": Client, "password": "plain" }` |
| GET | `/admin/clients/{id}` | | `{ "data": Client }` |
| PUT | `/admin/clients/{id}` | same fields, all optional | `{ "data": Client }` |
| POST | `/admin/clients/{id}/reset-password` | `{ "password"? }` | `{ "data": Client, "password": "plain" }` |
| DELETE | `/admin/clients/{id}` | `?purge=true` also deletes its files from disk | 204 |
| GET | `/admin/clients/{id}/devices` | pagination | paginated Device |
| DELETE | `/admin/clients/{id}/devices/{device_id}` | | 204 |

Deleting a client clears cache key `client:{username}`.

### Files (admin sees all clients)
| Method | Path | Query/Body | Response |
|---|---|---|---|
| GET | `/admin/files` | `client_id`, `folder`, `visibility`, `mime`, `search`, pagination | paginated File (each with `client: {id,name}`) |
| GET | `/admin/files/{id}` | | `{ "data": File }` |
| GET | `/admin/files/{id}/download` | | binary |
| DELETE | `/admin/files/{id}` | | 204 |
| POST | `/admin/files/bulk-delete` | `{ "ids": [1,2,3] }` | `{ "deleted": 3 }` |
| POST | `/admin/files` | multipart like client upload + `client_id` | 201 `{ "data": [File] }` |
| GET | `/admin/folders` | `client_id` (required), `parent` | same as `/api/folders` |

### Archives
| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/admin/archives` | `client_id`, `status`, pagination | paginated Archive (with `client: {id,name}`) |
| POST | `/admin/archives` | `{ "client_id": 3, "folders": ["a","b"] }` | 202 `{ "data": Archive }` |
| GET | `/admin/archives/{id}` | | `{ "data": Archive }` |
| GET | `/admin/archives/status?ids=1,2,3` | | `{ "data": [ { id, status, progress, url, expires_at } ] }` (for 5s polling) |
| GET | `/admin/archives/{id}/download` | | binary zip |
| DELETE | `/admin/archives/{id}` | | 204 |

### Audit
| GET | `/admin/audit-logs` | `actor_type`, `action`, `subject_type`, pagination | paginated AuditLog |

### Storage tools
| POST | `/admin/storage/sync` (super_admin only, `admin` gets 403) | | `{ "orphans_on_disk": n, "missing_on_disk": m, "removed_records": k, "removed_files": j, "errors": e }` (`errors` counts rows that could not be judged, e.g. stat failed with something other than "does not exist"; those rows are never deleted) — same as `artisan storage:sync` |

---

## Background

- Queue driver `database`. Job `zip_folders` (payload: archive_id). Streams zip with `archive/zip` + `io.Copy` (no full-file RAM). Updates `progress` every 2% or 50 files. On success sets status `done`, `size`, `path`, `expires_at`, `finished_at`; POSTs callback_url if set (10s timeout, 3 retries, failure ignored).
- Schedule (every hour): `archives:cleanup` — delete zip files where `expires_at < now`, set status `expired`. Also on boot: set `processing` → `pending` and re-dispatch.
- `artisan storage:sync` — reconcile DB vs disk.
- Env: `MAX_FILE_SIZE=52428800`, `ARCHIVE_TTL_DAYS=5`, `UPLOAD_RATE_PER_MINUTE=60`.
