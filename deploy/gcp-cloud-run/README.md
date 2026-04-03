# GCPlane — Google Cloud Run Deployment

Deploy `gcplane serve` as a long-running daemon on Cloud Run.

> **Important:** GCPlane is a daemon, not a request handler. `min-instances=1` is required to prevent Cloud Run from scaling to zero and halting reconciliation.

## Prerequisites

- [gcloud CLI](https://cloud.google.com/sdk/docs/install) authenticated (`gcloud auth login`)
- GCP project with billing enabled
- APIs enabled: Cloud Run, Secret Manager

```sh
gcloud services enable run.googleapis.com secretmanager.googleapis.com
```

---

## 1. Store Secrets

```sh
# GoClaw API token (required)
echo -n "YOUR_GOCLAW_TOKEN" | gcloud secrets create goclaw-token --data-file=-

# Provider keys (add only what you use)
echo -n "sk-ant-..." | gcloud secrets create anthropic-api-key --data-file=-
echo -n "sk-..."     | gcloud secrets create openai-api-key --data-file=-
```

Store your manifest as a secret (Cloud Run has no persistent volume):

```sh
gcloud secrets create gcplane-manifest --data-file=manifest.yaml
```

To update the manifest later:

```sh
gcloud secrets versions add gcplane-manifest --data-file=manifest.yaml
```

---

## 2. Grant Secret Access

```sh
PROJECT_NUMBER=$(gcloud projects describe $(gcloud config get-value project) --format='value(projectNumber)')
SA="${PROJECT_NUMBER}-compute@developer.gserviceaccount.com"

gcloud secrets add-iam-policy-binding goclaw-token \
  --member="serviceAccount:${SA}" --role="roles/secretmanager.secretAccessor"

gcloud secrets add-iam-policy-binding gcplane-manifest \
  --member="serviceAccount:${SA}" --role="roles/secretmanager.secretAccessor"
```

Repeat for any additional secret (e.g. `anthropic-api-key`).

---

## 3. Deploy

### Option A — gcloud CLI

```sh
gcloud run deploy gcplane \
  --image ghcr.io/dataplanelabs/gcplane:latest \
  --args="serve,-f,/config/manifest.yaml,--interval,30s" \
  --port=8480 \
  --min-instances=1 \
  --max-instances=1 \
  --memory=128Mi \
  --cpu=0.25 \
  --set-secrets="GOCLAW_TOKEN=goclaw-token:latest,/config/manifest.yaml=gcplane-manifest:latest" \
  --no-allow-unauthenticated \
  --region=us-central1
```

Add provider keys as needed:

```sh
  --set-secrets="...,ANTHROPIC_API_KEY=anthropic-api-key:latest"
```

### Option B — YAML (service.yaml)

Edit `service.yaml` to set your project and region, then:

```sh
gcloud run services replace service.yaml --region=us-central1
```

---

## 4. Verify

```sh
gcloud run services describe gcplane --region=us-central1
```

Check health (requires IAM token since `--no-allow-unauthenticated`):

```sh
URL=$(gcloud run services describe gcplane --region=us-central1 --format='value(status.url)')
curl -sf -H "Authorization: Bearer $(gcloud auth print-identity-token)" "${URL}/healthz" && echo OK
curl -sf -H "Authorization: Bearer $(gcloud auth print-identity-token)" "${URL}/readyz"  && echo ready
```

---

## 5. Logs

```sh
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=gcplane" \
  --limit=50 --format=json | jq '.[].textPayload'
```

Stream live (requires gcloud beta):

```sh
gcloud beta run services logs tail gcplane --region=us-central1
```

---

## 6. Upgrade

```sh
# Pin to a specific release
gcloud run services update gcplane \
  --image ghcr.io/dataplanelabs/gcplane:v1.1.0 \
  --region=us-central1
```

Or re-run the deploy command with the new tag.

---

## 7. Teardown

```sh
gcloud run services delete gcplane --region=us-central1

# Remove secrets when no longer needed
gcloud secrets delete goclaw-token
gcloud secrets delete gcplane-manifest
```

---

## Caveats

| Constraint | Detail |
|---|---|
| `min-instances=1` required | Cloud Run scales to zero by default; gcplane must stay running to reconcile |
| No persistent disk | Git-source mode caches the repo in-memory only; lost on restart (safe, just re-clones) |
| Manifest via Secret Manager | Cloud Run has no volume mounts from GCS/disk; secrets are the supported path |
| Cold start on crash | If the container exits unexpectedly, Cloud Run restarts it within ~10 s |
| CPU throttling | CPU is only guaranteed during request handling by default; use `--cpu-always-allocated` if reconcile loops stall |

To ensure CPU is always available for the reconcile loop:

```sh
gcloud run services update gcplane \
  --cpu-always-allocated \
  --region=us-central1
```
