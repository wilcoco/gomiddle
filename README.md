# gomiddle

Go middleware server for factory equipment integration. Current scope: reads
silo weights from a Modbus TCP PLC and serves them over an HTTP JSON API.
Planned: Mitsubishi MC-protocol injection PLCs, Odoo ERP API, Oracle/PostgreSQL.

## Project layout

```
cmd/server/        Entry point (main.go) — wiring and lifecycle only
internal/config/   Environment-variable configuration
internal/silo/     Modbus TCP poller for the 6 silo weight registers
internal/api/      HTTP server, routes, JSON responses
```

`internal/` packages cannot be imported by other repositories — Go enforces
this, which keeps the middleware's internals private by construction.

## Running

```sh
cp .env.example .env      # then edit values; .env is git-ignored
set -a; source .env; set +a

go run ./cmd/server
```

Without a PLC on your network, use mock mode:

```sh
MOCK_PLC=true go run ./cmd/server
```

## API

| Method | Path        | Description                                  |
| ------ | ----------- | -------------------------------------------- |
| GET    | /healthz    | Liveness check                               |
| GET    | /api/silos  | Latest silo weights (503 if the PLC is down) |

```sh
curl -s localhost:8080/api/silos | jq
```

Example response:

```json
{
  "readings": [
    {"silo": 1, "raw": 1234, "tons": 12.34},
    {"silo": 2, "raw": -56,  "tons": -0.56}
  ],
  "updated_at": "2026-07-20T13:30:00+09:00"
}
```

## Data interpretation

Each holding register (FC3, addresses 0–5) is a signed 16-bit value;
`value / 100` is the silo weight in tons. Negative values are possible.

## Build

```sh
go build -o bin/server ./cmd/server
go vet ./...
```
