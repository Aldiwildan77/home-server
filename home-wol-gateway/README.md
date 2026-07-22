# Home Gateway Technical Requirements (MVP)

## Overview

The Raspberry Pi is the **Home Controller** and the **single source of truth**. It does **not** need direct Layer 2 visibility to every subnet. Instead, it delegates discovery and Wake-on-LAN to the gateway responsible for each network.

---

## High-Level Architecture

```text
                         Internet
                             │
                      (Optional Cloud)
                             │
                       Raspberry Pi
                     Home Controller
                             │
        ┌────────────────────┴────────────────────┐
        │                                         │
 Local Discovery                          Registered Gateways
 (same subnet)                            (nested networks)
        │                                         │
        │                             ┌───────────┴───────────┐
        │                             │                       │
   MikroTik Plugin             MikroTik Gateway         ESP32 Gateway
                                     │                       │
                                Discovery + WoL        Discovery + WoL
                                     │                       │
                             Local Broadcast         Local Broadcast
```

---

# Raspberry Pi Responsibilities

The Raspberry Pi owns:

* Device inventory
* MAC ↔ IP mapping
* Friendly names
* Device tags
* Wake-on-LAN policies
* Gateway registry
* Network topology
* REST API
* Authentication
* Optional cloud synchronization

The Raspberry Pi **never broadcasts WoL packets outside its own subnet**.

---

# Gateway Responsibilities

Every gateway (MikroTik, ESP32, OpenWrt, Linux, etc.) implements two capabilities.

## 1. Discovery Provider

Reports:

* MAC
* IP
* Hostname (optional)
* Online status

Example:

```json
{
  "gateway_id": "mikrotik-livingroom",
  "devices": [
    {
      "mac": "AA:BB:CC:DD:EE:FF",
      "ip": "192.168.50.10"
    }
  ]
}
```

---

## 2. Wake Provider

Receives:

```json
{
  "mac": "AA:BB:CC:DD:EE:FF"
}
```

Executes:

```
Magic Packet
```

Only inside its local broadcast domain.

---

# Device Inventory

```yaml
devices:

- mac: AA:BB:CC:DD:EE:FF
  ip: 192.168.50.10
  name: NAS
  gateway_id: mikrotik-livingroom
  wol: true

- mac: 11:22:33:44:55:66
  ip: 192.168.3.20
  name: Desktop
  gateway_id: local
  wol: true
```

Every device belongs to exactly **one gateway**.

---

# Gateway Registration

Each gateway registers once.

```json
{
  "gateway_id": "mikrotik-livingroom",
  "parent": "raspi-home",
  "subnets": [
    "192.168.50.0/24"
  ],
  "capabilities": [
    "discovery",
    "wol"
  ]
}
```

---

# Wake-on-LAN Flow

```
User
 │
 │ Wake NAS
 ▼
Raspberry Pi
 │
 │ Lookup MAC
 ▼
Inventory
 │
 │ gateway_id = mikrotik-livingroom
 ▼
Gateway Manager
 │
 │ POST /wake
 ▼
MikroTik Gateway
 │
 │ Broadcast Magic Packet
 ▼
NAS
```

---

# Nested Network Support

```
Raspberry Pi
      │
Gateway A
      │
Gateway B
      │
Gateway C
      │
Target Device
```

The Raspberry Pi stores only:

```yaml
gateway_id: gateway-c
```

It does **not** calculate routing paths or router hops.

The owning gateway is responsible for waking devices on its local subnet.

---

# Gateway Interface

```go
type Gateway interface {
    ID() string

    Discover(ctx context.Context) ([]Device, error)

    Wake(ctx context.Context, mac net.HardwareAddr) error

    Heartbeat(ctx context.Context) error
}
```

Implementations:

* Raspberry Pi Local Scanner
* MikroTik
* ESP32
* OpenWrt
* Linux Agent

---

# Discovery Sources

| Source       | Discovery | Wake | Notes                |
| ------------ | --------- | ---- | -------------------- |
| Raspberry Pi | ✅         | ✅    | Local subnet only    |
| MikroTik     | ✅         | ✅    | RouterOS script/API  |
| ESP32        | ✅         | ✅    | Local subnet gateway |
| OpenWrt      | ✅         | ✅    | Local agent          |
| Linux        | ✅         | ✅    | Local agent          |

---

# Design Principles

* Raspberry Pi is the single source of truth.
* Every device has exactly one **Owning Gateway**.
* Discovery and policy are separate concerns.
* Wake-on-LAN is always executed by the Owning Gateway.
* The controller is vendor-independent.
* Nested routers are supported without changing the controller logic.
* New gateway types can be added by implementing the `Gateway` interface.
