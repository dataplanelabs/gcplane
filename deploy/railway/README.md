# GCPlane — Railway Deployment

Deploy `gcplane serve` as a managed service on [Railway](https://railway.com).

## Prerequisites

- Railway account ([railway.com](https://railway.com))
- A running [GoClaw](https://github.com/nextlevelbuilder/goclaw) instance reachable from the internet
- GoClaw API token
- [railway CLI](https://docs.railway.com/guides/cli) (optional — dashboard works for all steps)

---

## Manifest delivery

Railway has **no persistent volumes**, so `manifest.yaml` cannot be bind-mounted at runtime. Pick one approach before deploying:

### Option A — Bake manifest into a custom image (simplest)

```dockerfile
FROM ghcr.io/dataplanelabs/gcplane:latest
COPY manifest.yaml /config/manifest.yaml
```

```bash
docker build -t your-registry/gcplane:custom .
docker push your-registry/gcplane:custom
```

Use this image in the Railway service (see Option A below).

### Option B — Git source mode (recommended for GitOps)

No custom image needed. Change the start command to:

```
gcplane serve --repo https://github.com/your-org/manifests.git --branch main --path gcplane/manifest.yaml --interval 30s
```

For private repos, add `GCPLANE_GIT_TOKEN` as an environment variable (see below).

---

## Option A: Deploy from Docker image (dashboard)

1. Go to [railway.com/new](https://railway.com/new) → **Deploy a Docker Image**
2. Image: `ghcr.io/dataplanelabs/gcplane:latest` (or your custom image)
3. After creation, go to **Settings → Deploy**:
   - Start command: `gcplane serve -f /config/manifest.yaml --interval 30s`
   - Health check path: `/healthz`
4. Go to **Variables** and add required env vars (see below)
5. Set **Networking → Public Networking** port to `8480`
6. Click **Deploy**

## Option B: Deploy from repo with railway.json

Railway auto-detects `Dockerfile` and `railway.json` when linking a GitHub repo.

```bash
# From repo root
railway login
railway link        # link to existing project, or:
railway init        # create new project
railway up          # deploy from current directory
```

Or via dashboard: **New Project → Deploy from GitHub repo** → select repo → Railway picks up `Dockerfile` and `deploy/railway/railway.json` automatically.

Set environment variables after the first deploy (Variables tab).

---

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `GOCLAW_TOKEN` | Yes | GoClaw API token |
| `PORT` | Yes | Set to `8480` |
| `GCPLANE_LOG_FORMAT` | No | `json` (default) or `text` |
| `GCPLANE_GIT_TOKEN` | If private repo | Git token for git source mode |

Set these in the Railway dashboard under **Variables**, or via CLI:

```bash
railway variables set GOCLAW_TOKEN=<token>
railway variables set PORT=8480
railway variables set GCPLANE_LOG_FORMAT=json
```

---

## Verify

```bash
# Stream logs
railway logs --follow

# Check health (replace <app-url> with your Railway public URL)
curl -sf https://<app-url>/healthz && echo OK
curl -sf https://<app-url>/readyz  && echo ready
```

Or check the **Deployments** tab in the Railway dashboard — a green status means `/healthz` returned 200.

---

## Upgrade

**From Docker image:** Update the image tag in **Settings → Deploy → Docker Image**, then click **Redeploy**.

**From linked repo:** Push to the connected branch — Railway triggers a new build automatically.

```bash
# Via CLI
railway up
```

---

## Teardown

Delete the service from the Railway dashboard: **Project → Service → Settings → Delete Service**.

```bash
# Via CLI
railway down
```

---

## Caveats

- **No persistent storage** — manifest must be baked into the image or pulled from git (see above).
- **Port 8480** — Railway exposes one public port. Set `PORT=8480` in Variables so Railway routes traffic correctly. `/healthz`, `/readyz`, and `/metrics` are all accessible via the public URL.
- **GHCR auth** — `ghcr.io/dataplanelabs/gcplane` is public; no registry credentials required.
- **railway.json location** — If deploying from repo root, Railway looks for `railway.json` at the root. The file in `deploy/railway/railway.json` is for reference; copy or symlink it to the repo root if needed.

---

## File Reference

| File | Purpose |
|---|---|
| [`railway.json`](railway.json) | Railway service configuration |
| [`../vps/manifest.yaml.example`](../vps/manifest.yaml.example) | Manifest template to bake or commit |
| [`examples/minimal.yaml`](../../examples/minimal.yaml) | Minimal manifest reference |
