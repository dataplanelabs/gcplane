# GCPlane — Fly.io Deployment

Deploy `gcplane serve` as a persistent service on [Fly.io](https://fly.io).

## Prerequisites

- [flyctl](https://fly.io/docs/hands-on/install-flyctl/) CLI installed and authenticated (`fly auth login`)
- Fly.io account
- A running [GoClaw](https://github.com/nextlevelbuilder/goclaw) instance reachable from the internet
- GoClaw API token

---

## Quick Start

```bash
# 1. Clone config into a working directory
cp -r deploy/fly-io /tmp/gcplane-fly && cd /tmp/gcplane-fly

# 2. Create the app (skips deploy — config already exists)
fly launch --copy-config --no-deploy

# 3. Create a persistent volume for manifest + git cache
fly volumes create gcplane_data --region sin --size 1

# 4. Set secrets
fly secrets set GOCLAW_TOKEN=your-token-here

# 5. Deploy
fly deploy
```

---

## Manifest Delivery

### Option A — Volume (default)

The `fly.toml` mounts a 1 GB volume at `/data`. Upload your manifest once:

```bash
fly ssh console
# inside the machine:
cat > /data/manifest.yaml << 'EOF'
# paste your manifest here
EOF
exit
```

Or copy a local file via sftp:

```bash
fly sftp shell
put manifest.yaml /data/manifest.yaml
exit
```

The volume persists across deploys and restarts.

### Option B — Git source mode (recommended for GitOps)

No volume needed for the manifest itself (volume can still be used as git clone cache). Replace the process command in `fly.toml`:

```toml
[processes]
  app = "gcplane serve --repo https://github.com/your-org/manifests.git --branch main --path gcplane/manifest.yaml --interval 30s"
```

For private repos, add the git token:

```bash
fly secrets set GCPLANE_GIT_TOKEN=ghp_your-token
```

---

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `GOCLAW_TOKEN` | Yes | GoClaw API token |
| `GCPLANE_LOG_FORMAT` | No | `json` (default) or `text` |
| `GCPLANE_GIT_TOKEN` | If private repo | Git token for git source mode |

---

## Verify

```bash
fly status
fly logs

# Health endpoints
curl -sf https://gcplane.fly.dev/healthz && echo OK
curl -sf https://gcplane.fly.dev/readyz  && echo ready
```

---

## Upgrade

Update the image tag in `fly.toml` then redeploy:

```bash
# Edit fly.toml: image = "ghcr.io/dataplanelabs/gcplane:vX.Y.Z"
fly deploy
```

---

## Teardown

```bash
fly apps destroy gcplane
```

This also destroys attached volumes. Export your manifest first if needed.

---

## Caveats

- **`auto_stop_machines = false`** — must stay `false`; gcplane runs a continuous reconciliation loop and must not be suspended.
- **Volume required for Option A** — create the volume before the first deploy or the machine will fail to start.
- **Single machine** — `min_machines_running = 1` ensures one instance is always running; do not scale beyond 1 (no leader election).
- **GHCR auth** — `ghcr.io/dataplanelabs/gcplane` is public; no registry credentials required.
- **Port 8480** — `/healthz`, `/readyz`, and `/metrics` are accessible via the app URL.

---

## File Reference

| File | Purpose |
|---|---|
| [`fly.toml`](fly.toml) | Fly.io app configuration |
| [`../vps/manifest.yaml.example`](../vps/manifest.yaml.example) | Manifest template |
| [`../../examples/minimal.yaml`](../../examples/minimal.yaml) | Minimal manifest reference |
