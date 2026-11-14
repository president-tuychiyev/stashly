# nuxt-ship 🚢

Zero-config Docker deployment for [Nuxt 4](https://nuxt.com) — the Nuxt port of [goravel-ship](https://github.com/president-tuychiyev/goravel-ship).

Build your Nuxt app locally, bake `.output` into a Docker image, and ship it to any server via SSH — no Docker Hub, no CI/CD setup required.

---

## Installation

```bash
npm install -D nuxt-ship
```

Or use without installing:

```bash
npx nuxt-ship install
```

Scaffold the required files into your project:

```bash
npx nuxt-ship install
```

This creates:

- `Dockerfile` — copies pre-built `.output/` into a `node:20-alpine` image
- `.dockerignore`
- `docker-compose-prod.yml` — production compose config
- `.env.prod.example` — production env template
- `deploy.sh` — remote deployment script

Edit `.env.prod.example` with your production values, then you're ready to ship.

---

## Usage

```bash
npx nuxt-ship --user=<user> --ip=<server-ip> [flags]
```

### Flags

| Flag             | Alias | Default                       | Required |
| ---------------- | ----- | ----------------------------- | -------- |
| `--user`         | `-u`  | —                             | ✅       |
| `--ip`           | —     | —                             | ✅       |
| `--port`         | `-P`  | `22`                          | ❌       |
| `--path`         | `-p`  | `/opt/app`                    | ❌       |
| `--tag`          | `-t`  | `latest`                      | ❌       |
| `--image`        | `-i`  | _(auto from `package.json`)_  | ❌       |
| `--container`    | `-c`  | _(same as image name)_        | ❌       |
| `--root`             | —     | `false`                       | ❌       |
| `--skip-build`       | —     | `false`                       | ❌       |
| `--strip-sourcemaps` | —     | `false`                       | ❌       |

> `--skip-build` reuses an existing `.output/` directory instead of running `npm run build`.
>
> `--strip-sourcemaps` recursively deletes every `*.map` file under `.output/` before `docker build` — defense in depth in case `sourcemap: false` was forgotten in `nuxt.config`.

### Examples

**Minimal:**

```bash
npx nuxt-ship -u root --ip=192.168.1.100
```

**Custom port and path:**

```bash
npx nuxt-ship -u deploy --ip=1.2.3.4 -P 2222 -p /home/deploy/myapp
```

**With version tag:**

```bash
npx nuxt-ship -u root --ip=1.2.3.4 -t v1.2.0
```

**Skip build (reuse existing `.output`):**

```bash
npx nuxt-ship -u root --ip=1.2.3.4 --skip-build
```

**With sudo on the remote:**

```bash
npx nuxt-ship -u deploy --ip=1.2.3.4 --root
```

> **Git Bash (MINGW64) note:** Prepend `MSYS_NO_PATHCONV=1` to prevent path conversion:
>
> ```bash
> MSYS_NO_PATHCONV=1 npx nuxt-ship -u root --ip=1.2.3.4 -p /home/deploy/app
> ```

---

## Source-code protection

**No source files (`.vue`, `.ts`, `pages/`, `components/`, `server/api/`, `nuxt.config.ts`, `package.json`, `node_modules`, `.git`) ever leave your machine.** The Docker image is built from the pre-built `.output/` directory only — that's just the bundled Nitro server and static client assets.

For maximum source protection, set the following in `nuxt.config.ts`:

```ts
export default defineNuxtConfig({
  sourcemap: {
    server: false,   // no .mjs.map files → no source reconstruction
    client: false,   // no source visible in browser DevTools
  },
  nitro: {
    minify: true,    // minify the server bundle (default false!)
  },
});
```

If you might forget, pass `--strip-sourcemaps` and nuxt-ship will recursively delete every `*.map` under `.output/` before `docker build`. Note: this is insurance, not a substitute — the right thing is to disable sourcemaps at build time.

### What ends up on the server

After a successful deploy, `/opt/app/` (or your `--path`) contains only:

```
.env
docker-compose.yml
```

Everything else is cleaned up automatically:

| File                 | Status after success      |
| -------------------- | ------------------------- |
| `<hash>.tar.gz`      | deleted by `deploy.sh`    |
| `deploy.sh`          | self-deletes              |
| `.env`               | **kept** — used by compose |
| `docker-compose.yml` | **kept** — for `up/down/logs` |

The `.output/` build artifacts live **inside the Docker image only**, never on the host filesystem. The image itself is in the local Docker daemon and can be inspected only by users in the `docker` group on the server.

---

## How it works

1. Rebuilds `.env` from `.env.prod.example` on every deploy (always up to date)
2. Runs `npm run build` locally → produces `.output/`
3. Runs `docker build` — Dockerfile just `COPY .output ./.output`, no `npm install` inside the image
4. Saves the image as `<sha256-hash>.tar.gz` (unguessable filename)
5. Opens a single SSH tunnel — **password asked only once**
6. Uploads via SCP: `tar.gz`, `.env`, `docker-compose-prod.yml` (→ `docker-compose.yml` on server), `deploy.sh`
7. Runs `deploy.sh` on the server:
   - Loads the image via `docker load` — no Docker Hub needed
   - Passes `IMAGE_NAME` env so compose uses the correct local image
   - Recreates the container via `docker compose up -d`
   - Prunes dangling images
   - Cleans up `tar.gz` and self-deletes `deploy.sh`
   - On error: also removes `docker-compose.yml` for a clean state
8. Closes the SSH tunnel and removes the local `tar.gz`

---

## Configuration tips

### Container name

By default `--container` equals `--image` (auto-detected from `package.json` `name` field). If your `docker-compose.yml` uses a different `container_name`, pass it explicitly:

```bash
npx nuxt-ship -u root --ip=1.2.3.4 --container my_app_container
```

### Port

The default `docker-compose-prod.yml` maps host `3000` → container `3000`. Change the port mapping there if needed (and the `PORT` in `.env.prod.example`).

### External network

The compose file expects an external network called `app_network`. Create it once on the server:

```bash
docker network create app_network
```

This makes it easy to share the network with other services (db, redis, reverse proxy, etc.).

---

## Server requirements

- Docker + Docker Compose v2
- User must be in the `docker` group (run once):
  ```bash
  sudo usermod -aG docker <username>
  newgrp docker
  ```
- An external network named `app_network` (see above), or edit the compose file.

---

## Security

All user-controlled values (`--user`, `--ip`, `--path`, `--tag`, `--image`, `--container`) are validated against strict whitelist regexes at the CLI layer **and** wrapped with POSIX single-quote escaping at every shell interpolation site. Values that don't match the whitelist are rejected before any subprocess is started.

Whitelists:

| Flag          | Regex                                                                                  |
| ------------- | -------------------------------------------------------------------------------------- |
| `--user`      | `^[a-z_][a-z0-9_-]{0,31}$`                                                             |
| `--ip`        | IPv4, IPv6, or RFC 1123 hostname                                                       |
| `--path`      | `^/[A-Za-z0-9._\-/]{1,255}$` (no `..`, no `//`)                                        |
| `--tag`       | `^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}$`                                                  |
| `--image`     | `^[a-z0-9][a-z0-9._-]{0,253}(/...)*$` (Docker name conventions, max 6 path segments)   |
| `--container` | `^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`                                                   |

The auto-detected image name from `package.json` `name` is sanitized **and** revalidated before use, so a crafted upstream package can't smuggle metacharacters into Docker commands.

To report a vulnerability privately: use **GitHub Security Advisories** on the repository ("Report a vulnerability" tab) rather than opening a public issue.

---

## License

MIT
