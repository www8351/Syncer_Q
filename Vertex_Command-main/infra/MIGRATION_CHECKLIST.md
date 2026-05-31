# Vertex Command — Production Migration Checklist

Step-by-step guide for migrating from Replit to a dedicated colocated server.

---

## Phase 1: Server Provisioning

### 1.1 Server Requirements
- [ ] Ubuntu 22.04 LTS (or newer) dedicated server / VPS
- [ ] Minimum specs: 4 vCPU, 8 GB RAM, 80 GB SSD
- [ ] Static public IPv4 address
- [ ] SSH access with key-based authentication
- [ ] Firewall configured (ports 22, 80, 443 open)

### 1.2 Initial Server Setup
```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Set timezone
sudo timedatectl set-timezone UTC

# Create deploy user
sudo adduser deploy
sudo usermod -aG sudo deploy

# Disable root SSH login
sudo sed -i 's/PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config
sudo systemctl restart sshd

# Configure UFW firewall
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

---

## Phase 2: Install Docker & Docker Compose

### 2.1 Install Docker Engine
```bash
# Install dependencies
sudo apt install -y ca-certificates curl gnupg lsb-release

# Add Docker GPG key
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

# Add Docker repository
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Install Docker
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Add deploy user to docker group
sudo usermod -aG docker deploy
newgrp docker

# Verify installation
docker --version
docker compose version
```

### 2.2 Configure Docker Logging
```bash
sudo tee /etc/docker/daemon.json <<EOF
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  }
}
EOF
sudo systemctl restart docker
```

---

## Phase 3: DNS Setup

### 3.1 Configure DNS A Records
Replace `yourdomain.com` with the value you set in the `DOMAIN` env variable.
- [ ] `yourdomain.com` → Server IP
- [ ] `www.yourdomain.com` → Server IP
- [ ] `api.yourdomain.com` → Server IP

### 3.2 Verify DNS Propagation
```bash
# Check DNS resolution (run from any machine, replace with your domain)
dig +short yourdomain.com
dig +short api.yourdomain.com
dig +short www.yourdomain.com

# All should return the server's public IP
```

> **Note:** DNS propagation can take up to 48 hours. Wait until all records resolve correctly before proceeding.

---

## Phase 4: Deploy Application

### 4.1 Clone Repository
```bash
# As deploy user
cd /home/deploy
git clone <repository-url> vertex-command
cd vertex-command
```

### 4.2 Configure Environment Variables
```bash
cd infra

# Copy the template
cp .env.template .env

# Edit and fill in all required values
nano .env
```

**Required variables to configure:**
- [ ] `DOMAIN` — Your domain (e.g., `vertexcommand.com`). Used by Nginx template for subdomain routing
- [ ] `DATABASE_URL` — PostgreSQL connection string
- [ ] `SESSION_SECRET` — Generate: `openssl rand -hex 32`
- [ ] `CREDENTIALS_ENCRYPTION_KEY` — Generate: `openssl rand -hex 32` (must be 64 hex / 32 bytes)
- [ ] `REDIS_PASSWORD` — Generate: `openssl rand -hex 24`
- [ ] `WEBHOOK_SECRET` — Generate: `openssl rand -hex 32`
- [ ] `STRIPE_SECRET_KEY` — From Stripe Dashboard
- [ ] `STRIPE_WEBHOOK_SECRET` — From Stripe Dashboard
- [ ] `TRADOVATE_CID` — Tradovate Client ID
- [ ] `TRADOVATE_SEC` — Tradovate Client Secret
- [ ] `CUSTOM_DOMAIN` — Same as DOMAIN, used by the Node.js API

### 4.3 Set Up External PostgreSQL
If using an external PostgreSQL instance (recommended for production):
```bash
# Verify database connectivity
psql $DATABASE_URL -c "SELECT 1"

# Run database migrations
cd /home/deploy/vertex-command
npm install
npm run db:push
```

If running PostgreSQL locally, add it to docker-compose.yml or install separately.

---

## Phase 5: SSL Certificate Generation

### 5.1 Initial Certificate Setup (Before Starting Nginx with SSL)
```bash
cd /home/deploy/vertex-command/infra

# First, temporarily comment out the SSL server blocks in nginx/conf.d/default.conf.template
# (This template is auto-processed by Nginx via envsubst at container start)
# Keep only the HTTP server block with the ACME challenge location

# Start only nginx for ACME challenge
docker compose up -d nginx-proxy

# Generate certificates
docker compose run --rm certbot certonly \
  --webroot \
  -w /var/www/certbot \
  -d $DOMAIN \
  -d www.$DOMAIN \
  -d api.$DOMAIN \
  --email admin@$DOMAIN \
  --agree-tos \
  --no-eff-email

# Restore the full nginx config (uncomment SSL server blocks)
# Restart nginx to apply SSL
docker compose restart nginx-proxy
```

### 5.2 Verify SSL
```bash
# Test SSL
curl -I https://vertexcommand.com
curl -I https://api.vertexcommand.com
```

---

## Phase 6: Build & Start Services

### 6.1 Build All Images
```bash
cd /home/deploy/vertex-command/infra

# Build all services
docker compose build --no-cache

# Verify images were created
docker images | grep vertex
```

### 6.2 Start Services (Ordered)
```bash
# Start Redis first
docker compose up -d redis
sleep 5

# Verify Redis is healthy
docker compose exec redis redis-cli -a "$REDIS_PASSWORD" ping
# Should output: PONG

# Start Go routing engine
docker compose up -d go-routing-engine
sleep 3

# Start Node.js API
docker compose up -d nodejs-api
sleep 10

# Start frontend
docker compose up -d frontend
sleep 3

# Start Nginx reverse proxy
docker compose up -d nginx-proxy

# Start certbot renewal service
docker compose up -d certbot
```

### 6.3 Verify All Services
```bash
# Check all containers are running
docker compose ps

# Expected output: all services showing "Up" and "healthy"
```

---

## Phase 7: Smoke Tests

### 7.1 Health Check Endpoints
```bash
# Go routing engine health
curl -s http://localhost:8080/health | jq .

# Node.js API health
curl -s http://localhost:5000/api/health | jq .

# Frontend via Nginx
curl -I https://www.vertexcommand.com

# API via Nginx
curl -s https://api.vertexcommand.com/health | jq .

# Node.js API via Nginx
curl -s https://www.vertexcommand.com/api/health | jq .
```

### 7.2 WebSocket Connectivity
```bash
# Test follower WebSocket connection (requires wscat: npm install -g wscat)
# Include the auth token from FOLLOWER_WS_AUTH_TOKEN (or WEBHOOK_SECRET)
wscat -c "wss://api.$DOMAIN/ws/follower?group_id=test&token=$FOLLOWER_WS_AUTH_TOKEN"
# Should receive: {"type":"connected","session_id":"follower_1",...}
```

### 7.3 Webhook → Follower Fan-Out Test
```bash
# In terminal 1: Connect a follower WebSocket (authenticated)
wscat -c "wss://api.$DOMAIN/ws/follower?group_id=test&token=$FOLLOWER_WS_AUTH_TOKEN"

# In terminal 2: Send a webhook signal
WEBHOOK_SECRET="your-webhook-secret"
PAYLOAD='{"ticker":"ESH2025","action":"buy","contracts":1,"price":5100.50,"group_id":"test"}'
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" | awk '{print $2}')

curl -X POST https://api.$DOMAIN/webhook/tradingview \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Signature: $SIGNATURE" \
  -d "$PAYLOAD"

# Terminal 1 should display the trade signal message received via Redis Pub/Sub
```

### 7.4 Webhook Test
```bash
# Send a test webhook (replace with actual secret)
WEBHOOK_SECRET="your-webhook-secret"
PAYLOAD='{"ticker":"ESH2025","action":"buy","contracts":1,"price":5100.50}'
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" | awk '{print $2}')

curl -X POST https://api.vertexcommand.com/webhook/tradingview \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Signature: $SIGNATURE" \
  -d "$PAYLOAD"
```

### 7.4 Redis Pub/Sub Verification
```bash
# Subscribe to the trade signals channel (in one terminal)
docker compose exec redis redis-cli -a "$REDIS_PASSWORD" SUBSCRIBE vertex:trade_signals

# Send a test webhook (in another terminal) and verify the signal appears
```

---

## Phase 8: Post-Migration Verification

### 8.1 Application Checks
- [ ] Login page loads at `https://www.vertexcommand.com`
- [ ] User registration works
- [ ] User login works
- [ ] Dashboard loads with account data
- [ ] Copy trading groups display correctly
- [ ] Stripe checkout flow works
- [ ] WebSocket connections establish (check latency monitor)

### 8.2 Performance Checks
```bash
# Check container resource usage
docker stats --no-stream

# Check Redis latency
docker compose exec redis redis-cli -a "$REDIS_PASSWORD" --latency

# Check container logs for errors
docker compose logs --tail=50 go-routing-engine
docker compose logs --tail=50 nodejs-api
docker compose logs --tail=50 nginx-proxy
```

### 8.3 Update Stripe Webhook URL
- [ ] In Stripe Dashboard → Developers → Webhooks
- [ ] Update webhook endpoint to: `https://api.vertexcommand.com/api/stripe/webhook`
- [ ] Verify webhook events are received

### 8.4 Update TradingView Alert URLs
- [ ] Update any TradingView alerts to point to: `https://api.vertexcommand.com/webhook/tradingview`

---

## Phase 9: Rollback Procedure

If something goes wrong, follow these steps to revert:

### 9.1 Quick Rollback to Replit
```bash
# 1. Stop all Docker services
cd /home/deploy/vertex-command/infra
docker compose down

# 2. Revert DNS records to point back to Replit
#    - Update A records to Replit's IP
#    - Or revert CNAME to Replit deployment URL

# 3. Re-enable Replit deployment
#    - Push to main branch or redeploy via Replit dashboard

# 4. Verify Replit app is serving traffic
curl -I https://your-replit-domain.replit.app

# 5. Update Stripe webhook URL back to Replit
# 6. Update TradingView alert URLs back to Replit
```

### 9.2 Container-Level Rollback
```bash
# Roll back a single service to previous image
docker compose up -d --no-deps --build <service-name>

# View previous image tags
docker images --format "{{.Repository}}:{{.Tag}} {{.CreatedAt}}" | sort -k2
```

### 9.3 Data Recovery
```bash
# Redis data is persisted in the redis-data volume
# PostgreSQL should have regular backups configured separately

# Restore Redis from AOF
docker compose exec redis redis-cli -a "$REDIS_PASSWORD" BGREWRITEAOF
```

---

## Maintenance Commands

### Viewing Logs
```bash
# All services
docker compose logs -f

# Specific service
docker compose logs -f go-routing-engine
docker compose logs -f nodejs-api

# Last 100 lines
docker compose logs --tail=100 nginx-proxy
```

### Restarting Services
```bash
# Single service
docker compose restart go-routing-engine

# All services
docker compose restart

# Rebuild and restart
docker compose up -d --build go-routing-engine
```

### Updating Application
```bash
cd /home/deploy/vertex-command
git pull origin main
cd infra
docker compose build --no-cache
docker compose up -d
```

### SSL Certificate Renewal
Certbot container handles automatic renewal. Manual renewal:
```bash
docker compose run --rm certbot renew
docker compose restart nginx-proxy
```
