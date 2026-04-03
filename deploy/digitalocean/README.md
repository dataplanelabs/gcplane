# GCPlane — DigitalOcean App Platform Deployment

Deploy `gcplane serve` as a managed service on [DigitalOcean App Platform](https://www.digitalocean.com/products/app-platform).

## Prerequisites

- [doctl](https://docs.digitalocean.com/reference/doctl/how-to/install/) CLI installed and authenticated
- DigitalOcean account
- A running [GoClaw](https://github.com/nextlevelbuilder/goclaw) instance reachable from the internet
- GoClaw API token

---

## Manifest delivery

DO App Platform has **no persistent volumes**, so `manifest.yaml` cannot be bind-mounted at runtime. Pick one approach before deploying:

### Option A — Bake manifest into a custom image (simplest)

```dockerfile
FROM ghcr.io/dataplanelabs/gcplane:latest
COPY manifest.yaml /config/manifest.yaml
```

```bash
docker build -t your-registry/gcplane:custom .
docker push your-registry/gcplane:custom
```

Update `app.yaml` → `image.registry` / `repository` / `tag` to point at your image.

### Option B — Git source mode (recommended for GitOps)

No custom image needed. Replace `run_command` in `app.yaml` with:

```yaml
run_command: gcplane serve --repo https://github.com/your-org/manifests.git --branch main --path gcplane/manifest.yaml --interval 30s
```

For private repos, add `GCPLANE_GIT_TOKEN` as a secret env var (see below).

---

## Option A: Deploy with doctl

```bash
doctl apps create --spec deploy/digitalocean/app.yaml
```

Set the `GOCLAW_TOKEN` secret after creation:

```bash
APP_ID=$(doctl apps list --format ID --no-header | head -1)
doctl apps update $APP_ID --spec deploy/digitalocean/app.yaml
```

Or set secrets interactively via the dashboard (Apps → your app → Settings → Environment Variables).

## Option B: Deploy via Dashboard

1. Go to [cloud.digitalocean.com/apps](https://cloud.digitalocean.com/apps) → **Create App**
2. Choose **Deploy from spec** and paste the contents of `app.yaml`
3. Set `GOCLAW_TOKEN` as an encrypted secret when prompted

---

## Environment Variables

| Variable | Type | Required | Description |
|---|---|---|---|
| `GOCLAW_TOKEN` | Secret | Yes | GoClaw API token |
| `GCPLANE_LOG_FORMAT` | Plain | No | `json` (default) or `text` |
| `GCPLANE_GIT_TOKEN` | Secret | If private repo | Git token for git source mode |

---

## Verify

```bash
# List apps and their status
doctl apps list

# Get logs (replace <app-id>)
doctl apps logs <app-id> --follow

# Check health endpoint (replace <app-url>)
curl -sf https://<app-url>/healthz && echo OK
curl -sf https://<app-url>/readyz  && echo ready
```

---

## Upgrade

Update the spec (e.g. bump image tag) then:

```bash
doctl apps update <app-id> --spec deploy/digitalocean/app.yaml
```

If using Option A (custom image), push the new image first, then trigger a redeploy:

```bash
doctl apps create-deployment <app-id>
```

---

## Teardown

```bash
doctl apps delete <app-id>
```

---

## Caveats

- **No persistent storage** — manifest must be baked into the image or pulled from git (see above).
- **Instance size** — `basic-xxs` (512 MB / 1 vCPU) is sufficient; gcplane is lightweight (~30 MB RSS).
- **Port 8480** — DO App Platform exposes only one HTTP port externally. `/healthz`, `/readyz`, and `/metrics` are accessible via the app URL. Restrict webhook ingress with DO Firewall if not needed publicly.
- **GHCR auth** — `ghcr.io/dataplanelabs/gcplane` is public; no registry credentials required.

---

## File Reference

| File | Purpose |
|---|---|
| [`app.yaml`](app.yaml) | DO App Platform spec |
| [`../vps/manifest.yaml.example`](../vps/manifest.yaml.example) | Manifest template to bake or commit |
| [`examples/minimal.yaml`](../../examples/minimal.yaml) | Minimal manifest reference |
