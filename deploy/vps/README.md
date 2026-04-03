# GCPlane VPS Deployment Guide

Deploy `gcplane serve` as a long-running systemd service on a generic Linux VPS.

## Prerequisites

- Linux VPS: Ubuntu 22+, Debian 12+, RHEL 9+, or Amazon Linux 2023
- `curl` and `sudo` / root access
- A running [GoClaw](https://github.com/nextlevelbuilder/goclaw) instance (self-hosted or managed)
- GoClaw API token

---

## 1. Install Binary

```sh
curl -fsSL https://raw.githubusercontent.com/dataplanelabs/gcplane/main/deploy/install.sh | sh
```

To pin a specific version:

```sh
GCPLANE_VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/dataplanelabs/gcplane/main/deploy/install.sh | sh
```

Verify:

```sh
gcplane version
```

---

## 2. Create Service User

```sh
sudo useradd -r -s /usr/sbin/nologin -m -d /var/lib/gcplane gcplane
```

---

## 3. Configure

### Create config directory

```sh
sudo mkdir -p /etc/gcplane
```

### Option A — File source (manifest stored locally)

Copy the example manifest and edit it:

```sh
sudo cp /path/to/your/manifest.yaml /etc/gcplane/manifest.yaml
# or start from the bundled example:
sudo curl -fsSL https://raw.githubusercontent.com/dataplanelabs/gcplane/main/deploy/vps/manifest.yaml.example \
  -o /etc/gcplane/manifest.yaml
sudo chmod 644 /etc/gcplane/manifest.yaml
```

See [`manifest.yaml.example`](manifest.yaml.example) in this directory for a commented starting point.

### Option B — Git source (manifest pulled from a Git repo)

No local manifest file needed. Credentials are passed via the env file in step below.

### Create environment file

```sh
sudo cp /path/to/gcplane.env.example /etc/gcplane/gcplane.env
# or reference the upstream example:
# deploy/systemd/gcplane.env.example
sudo chmod 600 /etc/gcplane/gcplane.env
sudo chown gcplane:gcplane /etc/gcplane/gcplane.env
```

Edit `/etc/gcplane/gcplane.env`:

```sh
sudo nano /etc/gcplane/gcplane.env
```

Minimum required:

```
GOCLAW_TOKEN=<your-goclaw-api-token>
ANTHROPIC_API_KEY=sk-ant-...   # if using Anthropic provider
```

For git-source mode, also add:

```
GCPLANE_GIT_REPO=https://github.com/your-org/your-manifests.git
GCPLANE_GIT_BRANCH=main
GCPLANE_GIT_PATH=gcplane/manifest.yaml
```

### Fix ownership

```sh
sudo chown -R gcplane:gcplane /etc/gcplane /var/lib/gcplane
```

---

## 4. Install systemd Service

### File source

```sh
sudo curl -fsSL https://raw.githubusercontent.com/dataplanelabs/gcplane/main/deploy/systemd/gcplane.service \
  -o /etc/systemd/system/gcplane.service
```

### Git source

```sh
sudo curl -fsSL https://raw.githubusercontent.com/dataplanelabs/gcplane/main/deploy/systemd/gcplane-git.service \
  -o /etc/systemd/system/gcplane.service
```

### Enable and start

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now gcplane
```

---

## 5. Verify

```sh
sudo systemctl status gcplane
```

```sh
curl -sf http://localhost:8480/healthz && echo OK
curl -sf http://localhost:8480/readyz  && echo ready
```

Follow live logs:

```sh
journalctl -u gcplane -f
```

---

## 6. Firewall

GCPlane's HTTP port (`:8480`) should **not** be exposed publicly unless required (e.g. for webhook ingress). Lock it down or bind it to localhost.

### Ubuntu / Debian (ufw)

```sh
# Allow only localhost access (default — no rule needed if ufw default-deny)
# If you need webhook ingress from a specific IP:
sudo ufw allow from 203.0.113.10 to any port 8480

# Block public access explicitly:
sudo ufw deny 8480
sudo ufw reload
```

### RHEL / Amazon Linux (firewalld)

```sh
# Webhook ingress from a specific source only:
sudo firewall-cmd --permanent --add-rich-rule='rule family="ipv4" source address="203.0.113.10" port port="8480" protocol="tcp" accept'
sudo firewall-cmd --reload
```

---

## 7. Monitoring (optional)

Prometheus metrics are available at `/metrics`:

```sh
curl -s http://localhost:8480/metrics
```

Example `prometheus.yml` scrape config (add to your Prometheus server):

```yaml
scrape_configs:
  - job_name: gcplane
    static_configs:
      - targets: ['<vps-ip>:8480']
```

If the port is not publicly exposed, use a Prometheus push gateway or SSH tunnel:

```sh
ssh -L 8480:localhost:8480 user@vps-ip
# then scrape http://localhost:8480/metrics locally
```

---

## 8. Upgrade

Re-run the install script (it overwrites the binary in-place), then restart:

```sh
GCPLANE_VERSION=v1.1.0 curl -fsSL https://raw.githubusercontent.com/dataplanelabs/gcplane/main/deploy/install.sh | sh
sudo systemctl restart gcplane
sudo systemctl status gcplane
```

---

## 9. Uninstall

```sh
sudo systemctl stop gcplane
sudo systemctl disable gcplane
sudo rm /etc/systemd/system/gcplane.service
sudo systemctl daemon-reload

sudo rm -f /usr/local/bin/gcplane
sudo rm -rf /etc/gcplane
sudo rm -rf /var/lib/gcplane
sudo userdel -r gcplane 2>/dev/null || true
```

---

## File Reference

| File | Purpose |
|------|---------|
| [`deploy/install.sh`](../install.sh) | Binary installer (auto-detects OS/arch, verifies checksum) |
| [`deploy/systemd/gcplane.service`](../systemd/gcplane.service) | systemd unit — file source mode |
| [`deploy/systemd/gcplane-git.service`](../systemd/gcplane-git.service) | systemd unit — git source mode |
| [`deploy/systemd/gcplane.env.example`](../systemd/gcplane.env.example) | Environment variable template |
| [`deploy/vps/manifest.yaml.example`](manifest.yaml.example) | Manifest template for VPS deployment |
| [`examples/minimal.yaml`](../../examples/minimal.yaml) | Minimal manifest reference |
