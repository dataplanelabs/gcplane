# GCPlane on Coolify

Deploy GCPlane as a Docker Compose resource on a self-hosted [Coolify](https://coolify.io) instance.

## Prerequisites

- Coolify instance running (v4+)
- GoClaw instance reachable from the Coolify server
- GoClaw API token

## Deploy

### 1. Add resource

In the Coolify dashboard: **New Resource → Docker Compose**

Paste the contents of `docker-compose.yaml` into the compose editor.

### 2. Set environment variables

In the resource **Environment Variables** tab, add:

| Variable | Value |
|---|---|
| `GOCLAW_TOKEN` | Your GoClaw API token |

Add any additional variables your manifest references (provider API keys, etc.).

### 3. Add manifest

GCPlane needs a `manifest.yaml` at `/config/manifest.yaml` inside the container.

**Option A — File manager (volume mount)**

Upload your `manifest.yaml` via the Coolify file manager so it is available at the path mapped in the compose volume (`./manifest.yaml`).

**Option B — Git source**

Replace the volume mount with the `--repo` flag to pull the manifest from a Git repository:

```yaml
command:
  - serve
  - --repo
  - https://github.com/your-org/your-manifests
  - --interval
  - "30s"
```

Remove the `volumes` block when using this mode — no file upload needed.

### 4. Deploy

Click **Deploy**. Coolify builds, starts the container, and runs the health check at `/healthz`.

Coolify handles SSL termination and custom domains automatically — no extra config needed in the compose file.

## Verify

```bash
# Health check
curl https://<your-coolify-domain>/healthz

# Readiness
curl https://<your-coolify-domain>/readyz

# Metrics
curl https://<your-coolify-domain>/metrics
```

Or check the **Logs** tab in the Coolify dashboard.

## Upgrade

1. Update the image tag in the compose editor (e.g. `gcplane:v1.1.0`)
2. Click **Redeploy**

To always track `latest`, leave the tag as-is and click **Redeploy** when a new release is available.

## Teardown

Delete the resource in the Coolify dashboard. This stops and removes the container. Volumes and environment variables are removed with it.
