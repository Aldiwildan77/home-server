# Deploying home-wol-gateway

Two pieces: the **backend** (Go binary, one per machine/subnet) and the
**frontend** (static site, build once, host anywhere on your LAN).

## 1. Generate secrets

```bash
openssl rand -hex 32   # -> security.psk (same on every node)
openssl rand -hex 32   # -> security.api_token (same on every HTTP-enabled node)
```

Keep these two values handy, you'll paste them into configs below.

## 2. Backend — get the binary

Easiest: download a prebuilt release (Linux amd64/arm64/armv6/armv7 — covers
generic Linux and every Raspberry Pi model):

```bash
curl -sSL https://raw.githubusercontent.com/Aldiwildan77/home-server/master/home-wol-gateway/backend/install.sh | sh
```

Installs to `/usr/local/bin/home-wol-gateway` (set `INSTALL_DIR=...` to put
it somewhere else, `HWG_VERSION=home-wol-gateway-vX.Y.Z` to pin a version
instead of latest).

Or build from source on each machine that'll run a node:

```bash
cd home-wol-gateway/backend
go build -o home-wol-gateway .
```

Or with Docker instead:

```bash
cd home-wol-gateway/backend
docker build -t home-wol-gateway .
```

## 3. Backend — configure

Copy the example and edit it:

```bash
cp config.example.yaml config.yaml
```

**First node** (the one your browser will talk to — needs HTTP on):

```yaml
node:
  id: raspi-home
  listen_udp_addr: ":9090"
  listen_http_addr: ":8080"
db:
  path: "./data/inventory.db"
security:
  psk: "<paste psk>"
  api_token: "<paste api_token>"
```

**Every other node** (a subnet gateway/agent — UDP only):

```yaml
node:
  id: gateway-livingroom
  listen_udp_addr: ":9090"
  peers: [] # see note below
security:
  psk: "<paste same psk>"
```

Leave `peers: []` empty if this node is on the **same subnet** as another
node already in the mesh — as long as both use the same `listen_udp_addr`
port, they find each other automatically. Only set `peers: ["<ip>:9090"]`
to bridge into a *different* subnet (broadcast can't cross a router).

## 4. Backend — run

Quick test:

```bash
./home-wol-gateway
```

For real use, install as a systemd service so it restarts if it ever
crashes:

```bash
sudo mkdir -p /etc/home-wol-gateway
sudo cp home-wol-gateway /usr/local/bin/
sudo cp config.yaml /etc/home-wol-gateway/
sudo cp home-wol-gateway.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now home-wol-gateway
sudo systemctl status home-wol-gateway
```

Or with Docker:

```bash
docker run -d --name home-wol-gateway \
  --restart unless-stopped \
  --network host \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -v home-wol-data:/app/data \
  home-wol-gateway
```

(`--network host` is the simplest way to get real UDP broadcast + LAN
discovery working — Docker's default bridge network gets in the way of
both.)

## 5. Frontend — build and host

```bash
cd home-wol-gateway/frontend
npm install
npm run build
```

This produces `dist/` — a plain static site. Serve it with anything:

```bash
npx serve dist          # quick and dirty
# or drop dist/ behind nginx, Caddy, or any static file host on your LAN
```

## 6. Connect

Open the frontend in a browser, enter:

- **Gateway address**: `http://<first node's IP>:8080`
- **API token**: the `api_token` you generated in step 1

Done. Add more nodes anytime by repeating steps 2–4 on another machine.
Same subnet as an existing node → leave `peers` empty. Different subnet →
point `peers` at any one node already in the mesh, doesn't have to be the
first node.
