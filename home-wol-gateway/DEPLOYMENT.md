# Deploying home-wol-gateway

One binary per machine/subnet. On any node where you set `listen_http_addr`,
that same binary also serves the frontend — no separate hosting step needed
for those nodes. UDP-only nodes (most subnet agents) don't need the
frontend at all.

## 1. Get the binary

Prebuilt release (Linux amd64/arm64/armv6/armv7 — covers generic Linux and
every Raspberry Pi model, frontend already embedded):

```bash
curl -sSL https://raw.githubusercontent.com/Aldiwildan77/home-server/master/home-wol-gateway/backend/install.sh | sh
```

Installs to `/usr/local/bin/home-wol-gateway` (set `INSTALL_DIR=...` for
somewhere else, `HWG_VERSION=home-wol-gateway-vX.Y.Z` to pin a version
instead of latest).

Or build from source (`./build.sh` builds the frontend and embeds it too;
plain `go build .` also works but serves an empty `/` — fine for a UDP-only
agent that'll never expose HTTP anyway):

```bash
cd home-wol-gateway/backend
./build.sh              # cross-compiles every target into dist/, frontend embedded
# or just: go build -o home-wol-gateway .
```

Or with Docker (build from the `home-wol-gateway/` root, not `backend/` —
the image needs to see `frontend/` too, to embed it the same way):

```bash
cd home-wol-gateway
docker build -t home-wol-gateway .
```

## 2. Generate a config

```bash
home-wol-gateway -init raspi-home -config /etc/home-wol-gateway/config.yaml
```

Writes a starter config with a freshly generated `psk` and `api_token`
already filled in (refuses to overwrite if the file already exists). Open
it and fill in:

- `node.listen_http_addr` — set to e.g. `":8080"` on the node your browser
  will talk to. Leave empty on a UDP-only subnet agent.
- `node.advertise_http_addr` — that node's own reachable URL, e.g.
  `"http://192.168.1.10:8080"` (only needed if `listen_http_addr` is set).
- `node.peers` — leave `[]` if this node is on the **same subnet** as
  another node already in the mesh (they'll find each other automatically
  via a broadcast beacon, as long as both use the same `listen_udp_addr`
  port). Set `["<ip>:9090"]` only to bridge into a *different* subnet.
- `discovery.subnet` — this node's own CIDR, to actively sweep for idle
  devices instead of only passively reading the ARP table.

Every other node in the mesh needs the **same `psk`** — copy it from the
first config into every subsequent `-init` (or just paste it in by hand).

## 3. Run it

Quick test:

```bash
home-wol-gateway -config /etc/home-wol-gateway/config.yaml
```

For real use, install as a systemd service so it restarts if it ever
crashes:

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin home-wol-gateway
sudo chown -R home-wol-gateway:home-wol-gateway /etc/home-wol-gateway
sudo curl -sSL https://raw.githubusercontent.com/Aldiwildan77/home-server/master/home-wol-gateway/backend/home-wol-gateway.service \
  -o /etc/systemd/system/home-wol-gateway.service
sudo systemctl daemon-reload
sudo systemctl enable --now home-wol-gateway
sudo systemctl status home-wol-gateway
```

(No `-config` flag needed there — the unit's `WorkingDirectory` is already
`/etc/home-wol-gateway`, and with no `-config` given the binary defaults to
`config.yaml` in its working directory.)

Or with Docker (image built in step 1):

```bash
docker run -d --name home-wol-gateway \
  --restart unless-stopped \
  --network host \
  -v /etc/home-wol-gateway:/etc/home-wol-gateway \
  home-wol-gateway -config /etc/home-wol-gateway/config.yaml
```

(`--network host` is the simplest way to get real UDP broadcast + LAN
discovery working — Docker's default bridge network gets in the way of
both.)

## 4. Connect

If a node has `listen_http_addr` set, just open `http://<that node's
IP>:<port>` in a browser — the frontend is already there, and the gateway
address field prefills itself. Paste in the `api_token` from step 2 and
you're connected.

Add more nodes anytime by repeating steps 1–3 on another machine. Same
subnet as an existing node → leave `peers` empty. Different subnet → point
`peers` at any one node already in the mesh, doesn't have to be the first
node.
