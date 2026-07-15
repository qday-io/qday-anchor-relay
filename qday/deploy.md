
# QDAY Anchor Relay — Deployment Guide

## 1. Prerequisites

### 1.1 Operating System

| Requirement | Details |
|-------------|---------|
| OS | Linux (Ubuntu 22.04+ / Debian 12+ recommended) |
| Architecture | **`linux/amd64` (x86_64)** — required. ARM (Apple Silicon / AWS Graviton) is **not** supported. |

### 1.2 Software

| Software | Minimum Version | Purpose |
|----------|----------------|---------|
| Docker | ≥ 24.0 | Container runtime |
| Docker Compose | ≥ v2.20 | Service orchestration |
| Git | ≥ 2.40 | Clone the repository |
| OpenSSL | — | Generate secrets (session secret) |
| Foundry (optional) | latest | Deploy `QDayAnchorRegistry` contract (only if not already deployed) |
| PostgreSQL client (optional) | 16 | Manual troubleshooting (`psql`) |

### 1.3 External Services (must be accessible from the server)

| Service | What to Obtain | Provider |
|---------|---------------|----------|
| **DA Chain RPC** | JSON-RPC endpoint URL | Operations team |
| **DA Chain** | Chain ID, Commit contract address | Operations team / Vike |
| **Abelian RPC** | Endpoint URL + username + password | Operations team |
| **GHCR** | `ghcr.io` (port 443) | Public internet |

### 1.4 Credentials (must be prepared before deployment)

| Item | Description | How to Obtain |
|------|-------------|---------------|
| **Relayer EOA private key** | EVM key with gas on DA chain for writing anchor records | Generate via `cast wallet new`, funded by team |
| **Abelian wallet mnemonic** | AIP-11 mnemonic phrase with sufficient ABEL balance | Generate via abewallet CLI, funded by team |
| **Portal admin password** | bcrypt hash for login | See step 4 below |
| **Session secret** | Random ≥32 bytes for Next.js session encryption | See step 4 below |

### 1.5 Network Connectivity

**Outbound (server → internet):**

| Destination | Port | Purpose |
|-------------|------|---------|
| DA RPC endpoint | varies | Read commit events, write anchor records |
| Abelian RPC endpoint | varies (typically 8666/8667) | Send transactions, query confirmations |
| `ghcr.io` | 443 (HTTPS) | Pull Docker images |
| Abelian Explorer (optional) | 443 | Portal transaction links |
| DA Explorer (optional) | 443 | Portal transaction links |

**Inbound (internet → server):**

| Port | Source | Purpose |
|------|--------|---------|
| `3000` (or `PORTAL_PORT`) | Operators / Nginx | Portal admin dashboard |
| `:8081` (relay admin API) | **Internal only** — never expose to public | Relay health/metrics |

---

## 2. Deployment Steps

### Step 1 — Install Docker & Docker Compose

```bash
# Install Docker
sudo apt update
sudo apt install -y docker.io

# Install Docker Compose v2
sudo apt install -y docker-compose-v2

# Add current user to docker group (avoid sudo for docker commands)
sudo usermod -aG docker $USER

# Log out and log back in for group membership to take effect, or run:
newgrp docker

# Verify installation
docker --version          # ≥ 24.0
docker compose version    # ≥ 2.20
```

### Step 2 — Clone the Repository

```bash
git clone https://github.com/qday-io/qday-anchor-relay.git
cd qday-anchor-relay
```

### Step 3 — Deploy QDayAnchorRegistry Contract (if not already deployed)

Skip this step if the registry contract has already been deployed and you have its address.

```bash
# Install Foundry
curl -L https://foundry.paradigm.xyz | bash
foundryup

# Install Solidity dependencies and build
cd contracts
forge install
forge build

# Deploy to DA chain
# Replace <DA_RPC_URL> and <DEPLOYER_PRIVATE_KEY> with actual values
forge script script/Deploy.s.sol \
  --rpc-url <DA_RPC_URL> \
  --private-key <DEPLOYER_PRIVATE_KEY> \
  --broadcast \
  -vvvv

# Record the deployed contract address from the output.
# You will need it as ANCHOR_REGISTRY_ADDRESS in Step 5.
cd ..
```

### Step 4 — Generate Secrets

```bash
# 4a. Generate session secret (≥32 bytes random)
echo "SESSION_SECRET=$(openssl rand -hex 32)"

# 4b. Generate admin password hash (bcrypt)
# Option A: using htpasswd
ADMIN_PASSWORD="your-secure-password"
echo "ADMIN_PASSWORD_HASH=$(htpasswd -bnBC 10 "" "$ADMIN_PASSWORD" | tr -d ':\n')"

# Option B: using Node.js (if available)
# node -e "const bcrypt=require('bcryptjs');console.log(bcrypt.hashSync('$ADMIN_PASSWORD',10))"

# Save these values — you will paste them into .env in Step 5.
```

### Step 5 — Configure Environment Variables

```bash
# Copy the example file
cp .env.example .env

# Edit the file with all required values
vim .env
```

Fill in every field below. Fields marked `【REQUIRED】` must not be left empty.

```ini
# === Postgres ===
POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_DB=anchor
POSTGRES_USER=anchor
POSTGRES_PASSWORD=<your-db-password>               # 【REQUIRED】
PORTAL_DB_USER=portal_ro
PORTAL_DB_PASSWORD=<your-portal-db-password>       # 【REQUIRED】

# === QDAY DA Chain (EVM) ===
DA_RPC_URL=<da-rpc-endpoint>                       # 【REQUIRED】
DA_CHAIN_ID=<chain-id>                             # 【REQUIRED】
DA_COMMIT_CONTRACT=<commit-contract-address>       # 【REQUIRED】
DA_ROLLUP_ID=0
DA_CONFIRM_DEPTH=12
DA_POLL_INTERVAL_SECONDS=10
DA_START_BLOCK=0

# === AnchorRegistry ===
ANCHOR_REGISTRY_ADDRESS=<registry-address>         # 【REQUIRED】paste from Step 3
RELAYER_PRIVATE_KEY=<relayer-eoa-key>              # 【REQUIRED】funded EOA private key (without 0x prefix)

# === Abelian ===
ABEL_NETWORK=testnet                               # mainnet | testnet
ABEL_RPC_ENDPOINT=<abelian-rpc-endpoint>           # 【REQUIRED】
ABEL_RPC_USER=<abelian-rpc-username>               # 【REQUIRED】
ABEL_RPC_PASS=<abelian-rpc-password>               # 【REQUIRED】
ABEL_RPC_TLS_CERT_PATH=
ABEL_CONFIRM_DEPTH=6
ABEL_SCAN_START_HEIGHT=0
ABEL_WALLET_MNEMONIC=<24-word-mnemonic>            # 【REQUIRED】or use ABEL_WALLET_KEY_FILE
ABEL_WALLET_KEY_FILE=
ABEL_ACCOUNT_PRIVACY=pseudonymous
ABEL_TX_FEE_NEUTRINO=1000000

# === Relay Behavior ===
RETRY_MAX=5
RETRY_BACKOFF_BASE_SECONDS=30

# === Portal ===
PORTAL_PORT=3000
ADMIN_USERNAME=admin
ADMIN_PASSWORD_HASH=<bcrypt-hash-from-step-4>      # 【REQUIRED】
SESSION_SECRET=<hex-from-step-4>                   # 【REQUIRED】
ABEL_EXPLORER_BASE=https://explorer.pqabelian.io
DA_EXPLORER_BASE=<da-explorer-url-if-any>
TRUST_PROXY_HEADERS=false
SESSION_COOKIE_SECURE=

# === Relay Admin (internal only) ===
RELAY_ADMIN_ADDR=:8081
ABEL_ANS_URL=https://testnet-ans.pqabelian.io/ans/v1
```

### Step 6 — Create Production Deploy Directory

```bash
# Create directory structure
mkdir -p ~/deploy/qday-anchor-relay/{initdb,scripts}

# Copy production compose file
cp deploy/server/compose.yml ~/deploy/qday-anchor-relay/compose.yml

# Copy database init script (creates portal_ro read-only role)
cp deploy/initdb/01-portal-ro.sh ~/deploy/qday-anchor-relay/initdb/

# Copy auto-update script
cp deploy/server/scripts/update_anchor_relay.sh ~/deploy/qday-anchor-relay/scripts/

# Copy the .env you filled in Step 5
cp .env ~/deploy/qday-anchor-relay/.env
```

### Step 7 — Set File Permissions

```bash
cd ~/deploy/qday-anchor-relay

# Protect sensitive credentials
chmod 600 .env

# Make scripts executable
chmod +x scripts/update_anchor_relay.sh
```

### Step 8 — Pull Images and Start Services

```bash
cd ~/deploy/qday-anchor-relay

# Pull latest images from GHCR
docker compose pull

# Start all services in detached mode
docker compose up -d
```

This starts 3 containers:
- `postgres` — PostgreSQL 16, runs init script to create `portal_ro` role
- `qday-anchor-relay` — Core relay engine, auto-runs database migrations at startup
- `qday-anchor-portal` — Next.js admin dashboard, exposed on `PORTAL_PORT`

### Step 9 — Check Startup Logs

```bash
# Watch all service logs
docker compose logs -f

# Check that all 3 services are running
docker compose ps
```

Expected output:

| Container | Status |
|-----------|--------|
| `postgres` | `Up (healthy)` |
| `qday-anchor-relay` | `Up` |
| `qday-anchor-portal` | `Up` |

Key log lines to look for in the relay:

```
INFO  da poller started
INFO  coin scanner started
INFO  abelian submitter started
INFO  abelian confirmer started
INFO  registry recorder started
```

If any service keeps restarting, check logs for errors:

```bash
docker compose logs relay   | tail -100
docker compose logs portal  | tail -100
docker compose logs postgres | tail -50
```

### Step 10 — Verify Deployment

```bash
# 10a. Verify PostgreSQL is healthy
docker compose exec postgres pg_isready -U anchor -d anchor
# Expected: /var/run/postgresql:5432 - accepting connections

# 10b. Verify relay is processing jobs
docker compose logs relay | grep -E "job|error|INFO" | tail -20

# 10c. Verify portal responds
curl -s -o /dev/null -w "%{http_code}" http://localhost:3000
# Expected: 200

# 10d. Open the portal in your browser
# http://<server-ip>:3000
# Login with your ADMIN_USERNAME and password from Step 4.
# Navigate to "Jobs" to see anchor job statuses.
```



---

## 3. Verification Checklist

After completing all steps, verify the following:

- [ ] `docker compose ps` shows all 3 services as `Up`
- [ ] `docker compose exec postgres pg_isready -U anchor -d anchor` returns `accepting connections`
- [ ] `curl -s -o /dev/null -w "%{http_code}" http://localhost:3000` returns `200`
- [ ] Portal login works with your configured username and password
- [ ] Portal "Jobs" page loads without errors
- [ ] Relay logs show no `ERROR` or `FATAL` messages: `docker compose logs relay | grep -iE "error|fatal"`
- [ ] Relay logs show poller, scanner, submitter, confirmer, and recorder goroutines all started
- [ ] Jobs appear in Portal and transition from `PENDING` through to `DONE`
- [ ] Nginx is proxying HTTPS correctly (open `https://portal.example.com`)

---

## 4. Troubleshooting Quick Reference

| Symptom | First Check |
|---------|-------------|
| `docker compose up -d` fails | `docker compose logs postgres` — is Postgres starting? Is `.env` correctly placed and formatted? |
| Relay restarts in a loop | `docker compose logs relay` — are DA_RPC_URL and ABEL_RPC_ENDPOINT reachable? Is RELAYER_PRIVATE_KEY valid? |
| Jobs stuck at `PENDING` | Check DA poller logs for RPC errors. Verify `DA_COMMIT_CONTRACT` and `DA_ROLLUP_ID` are correct. |
| Jobs stuck at `ABEL_SUBMITTED` | Check Abelian confirmer logs. Verify `ABEL_CONFIRM_DEPTH` and RPC connectivity. |
| Portal shows "500 Internal Server Error" | `docker compose logs portal` — check DB credentials (`PORTAL_DB_USER`/`PORTAL_DB_PASSWORD`). |
| Cannot log into Portal | Verify `ADMIN_PASSWORD_HASH` is a valid bcrypt hash. Regenerate if needed (Step 4b). |
| `SESSION_SECRET` too short | Must be ≥32 bytes. Regenerate with `openssl rand -hex 32`. |
