# Stashly — Admin Web App

Nuxt 4 admin panel for the S3 file-storage service. It talks to the backend admin API
(`/admin/...`) documented in [`../API_CONTRACT.md`](../API_CONTRACT.md).

Built with Naive UI (`@bg-dev/nuxt-naiveui`), Tailwind CSS v4, `@nuxtjs/i18n` (en / ru / uz,
English is the default and the fallback), `@vicons/ionicons5` and the Saira variable font. Light and dark mode
are supported, and every page is mobile-responsive: the sidebar collapses into a drawer,
tables switch to card lists, and forms stack into a single column.

## Roles

Two roles, mirroring the API contract:

- `super_admin` — sees everything, manages users, may pick a client's owner and run the storage
  sync.
- `admin` — only the clients it owns. The **Users** and **Storage sync** menu entries are hidden
  and `/users` refuses to render; the API 403s these routes as well.

`useAuth()` exposes `isSuperAdmin` for these checks.

## Authentication

Accounts are created by a super admin from the **Users** page; there is no public sign-up. All
auth pages use the `auth` layout and are reachable without a token.

- `/auth/sign-in` — email + password (`POST /admin/auth/login`). A 403 `account not verified`
  shows a link to the activation page; a 403 for a blocked account shows a dedicated notice.
- `/auth/verify?email=` — first-time activation: the 6-digit code from the invitation email plus
  a new password (`POST /admin/auth/verify`). On success the token is stored and the panel opens.
  "Resend code" (`POST /admin/auth/verify/resend`) is throttled by a 60 s countdown.
- `/auth/forgot` — requests a reset code (`POST /admin/auth/forgot`). The result is always
  reported as success so the form cannot be used to probe for accounts.
- `/auth/reset?email=` — email + code + new password (`POST /admin/auth/reset`), then sign in.

OTP codes are 6 digits, valid 10 minutes, resendable after 60 s. The password rule shared by every
form lives in `app/utils/validation.ts` (min 8 characters, at least one letter and one digit) and
is unit tested.

## Features

- **Profile** (`/profile`, also in the navbar account menu) — change your name, change your
  password, and change your email through an OTP confirmation step.
- **Users** (`/users`, super admin only) — searchable, filterable, paginated account list with
  role and status tags, client counts and last login. Create sends an invitation code by email;
  editing can block or promote other accounts (never your own); deleting an account that still
  owns clients asks for a user to reassign them to.
- **Dashboard** — client / file / size / pending-archive counters, per-client usage bars and a
  30-day upload chart (plain CSS/SVG bars, no chart library).
- **Clients** — searchable, filterable, paginated list with create/edit (including the owner
  select for super admins), password reset,
  device management and delete (optionally purging files from disk). Generated passwords are
  shown once in a copy-to-clipboard modal.
- **Files** — per-client browser with breadcrumb folder navigation, image thumbnails, download,
  single and bulk delete, and multi-file upload with a real progress bar.
- **Archives** — status badges with live progress, download links, expiry countdown, a lazy-loaded
  folder tree for creating new archives, and 5-second polling while any archive is pending or
  processing.
- **Audit logs** — filterable, paginated table with expandable JSON details.
- **Storage sync** — runs `POST /admin/storage/sync` and reports the reconciliation counters.

## Environment

Copy `.env.example` to `.env` and adjust:

```
NUXT_PUBLIC_API_BASE=http://127.0.0.1:3000
NUXT_PUBLIC_COOKIE_SECURE=false
```

`NUXT_PUBLIC_API_BASE` is the base URL of the backend. It defaults to `http://127.0.0.1:3000`
when unset. The admin JWT is stored in a `token` cookie and sent as `Authorization: Bearer …`.
`NUXT_PUBLIC_COOKIE_SECURE` controls the cookie's `Secure` flag; set it to `true` only when the
app is served behind HTTPS — enabling it over plain HTTP makes the browser drop the cookie.

## Development

```bash
npm install
npm run dev            # http://localhost:3000
PORT=3001 npm run dev  # when the backend already occupies port 3000
```

## Production

```bash
npm run build
npm run preview
```

```
./node_modules/.bin/nuxt-ship --user=kuadmin --ip=213.230.122.210 --port=222 --path=/home/www/stashly.tiuac.uz/web-app --strip-sourcemaps
```
