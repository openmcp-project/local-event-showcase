# ORD + A2A Demo

Kubernetes deployment of the [a2a-ord-demo](https://github.com/open-resource-discovery/a2a-ord-demo) — demonstrating how [Open Resource Discovery (ORD)](https://open-resource-discovery.github.io/) and [Agent-to-Agent (A2A)](https://google.github.io/A2A/) work together: discover agents via ORD metadata, then communicate with them using the A2A protocol.

## Architecture

```
  Spaceship App                                 Commander (super-agent)
  ┌──────────────────────┐                      ┌────────────────────────┐
  │ Solar Explorer       │    ORD discovery      │ "My Spaceship"         │
  │   /solar             │ <──────────────────── │                        │
  │ Repair Technician    │                       │  1. discovers agents   │
  │   /repair            │    A2A (JSON-RPC)     │     via ORD            │
  │ ORD endpoint         │ <──────────────────── │  2. resolves cards     │
  │   /ord/v1/...        │                       │  3. delegates via A2A  │
  └──────────────────────┘                      └────────────────────────┘
```

| Service         | In-cluster port | Local port | Role                                                              |
| --------------- | --------------- | ---------- | ----------------------------------------------------------------- |
| `spaceship-app` | 3000            | 3001       | Hosts 2 A2A agents (Solar Explorer + Repair Technician) and ORD   |
| `super-agent`   | 3000            | 3002       | Commander — discovers agents via ORD, delegates via A2A           |
| `basic-ord`     | 8083            | 3004       | Serves ORD documents via [provider-server](https://github.com/open-resource-discovery/provider-server) |

## Getting Started

Set up port-forwards to access the services locally:

```bash
kubectl port-forward svc/spaceship-app 3001:3000
kubectl port-forward svc/super-agent 3002:3000
kubectl port-forward svc/basic-ord 3004:8083
```

Then open [`demo.http`](demo.http) in VS Code with the [REST Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client) extension for a guided walkthrough:

1. **ORD Discovery** — fetch `.well-known/open-resource-discovery` and ORD documents from the spaceship-app
2. **Super Agent ORD** — discover the commander's own ORD metadata
3. **Agent Cards + A2A Calls** — fetch individual agent cards and invoke them directly (Repair Technician, Solar Explorer)
4. **Commander Delegation** — ask the commander a question and watch it discover and delegate to crew agents automatically
5. **A2A Editor Playground** — try the visual [A2A Editor](https://open-resource-discovery.github.io/a2a-editor/playground) with the local endpoints

## How It Works

1. The **spaceship-app** publishes ORD documents describing its agents and exposes A2A endpoints (`/solar`, `/repair`)
2. The **super-agent** reads ORD from the spaceship-app (`ORD_SOURCE_URL`), discovers available agents and their capabilities
3. When asked a question, the commander resolves which agent to delegate to, fetches its A2A agent card, and sends a JSON-RPC `message/send` request
4. The **basic-ord** service provides a standalone ORD provider for additional metadata

## Upstream

Source code and Docker images: [open-resource-discovery/a2a-ord-demo](https://github.com/open-resource-discovery/a2a-ord-demo)
