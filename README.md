# Wii Sports Club — NEX game server

A replacement NEX game server for *Wii Sports Club*, built on
[Pretendo Network](https://github.com/PretendoNetwork)'s NEX libraries.
Nintendo Network shut down on 8 April 2024; this brings the game's online
modes (matchmaking, leaderboards, StreetPass-style common data) back on a
server you run yourself.

It serves the two PRUDP servers the game expects — an **authentication
server** that issues Kerberos tickets, and a **secure server** carrying
matchmaking, NAT traversal, ranking and DataStore.

---

## The game's identity

| Property | Value |
|---|---|
| Access key | `4d324052` |
| NEX library version | `3.4.7` |

## Ranking

Wii Sports Club's `Ranking::GetCachedTopXRankings` (method `0x0F`) doesn't use
the standard NEX signature for that method — a single request can carry a
list of ranking categories, and structures in this one method carry NEX
structure headers even though the rest of the game's traffic (NEX 3.4.7)
doesn't. `nex/wscranking` implements this game-specific variant as a patch
over the standard Ranking protocol, so every other Ranking method keeps using
the common implementation unmodified.

---

## Architecture

```
                  ┌──────────────────────────────┐
   Wii U ─────────▶ authentication server        │  UDP / PRUDP
                  │   Ticket Granting             │
                  └──────────────┬───────────────┘
                                 │ Kerberos ticket + secure server address
                  ┌──────────────▼───────────────┐
   Wii U ─────────▶ secure server                │  UDP / PRUDP
                  │   Secure Connection, NAT      │
                  │   Traversal, Match Making,    │
                  │   Matchmake Extension,        │
                  │   Ranking, DataStore, Utility  │
                  └──────────────┬───────────────┘
                                 │
                          ┌──────▼──────┐
                          │  PostgreSQL │
                          └─────────────┘

   Wii U ◀────────────── P2P ──────────────▶ Wii U
```

```
main.go, init.go             startup, shutdown, configuration
globals/                     shared server state (accounts, station store, ...)
database/                    Postgres-backed rankings, common data and DataStore objects
nex/
  authentication.go          authentication server bootstrap
  secure.go                  secure server bootstrap
  register_common_*.go       protocol registration and game-specific handler overrides
  wscranking/                Wii Sports Club's non-standard GetCachedTopXRankings
nex-protocols-common-go-patch/  a locally patched copy of Pretendo's nex-protocols-common-go
                                 (see its own LICENSE — still AGPL-3.0), pulled in via a
                                 go.mod replace directive
```

---

## Running it

### Requirements

- Go 1.25+
- PostgreSQL

### Configuration

All variables are read directly from the environment (or a `.env` file next
to the binary).

| Variable | Meaning |
|---|---|
| `PN_POSTGRES_URI` | Postgres connection string. **Required.** |
| `PN_KERBEROS_PASSWORD` | Password for the two internal Quazal server accounts. Defaults to a well-known placeholder — set this for anything beyond local testing. |
| `PN_AUTHENTICATION_SERVER_PORT` | UDP port for the authentication server. **Required.** |
| `PN_SECURE_SERVER_HOST` | Address consoles will use to reach the secure server. Sent to the console verbatim during login, so it must be routable *from the console*. **Required.** |
| `PN_SECURE_SERVER_PORT` | UDP port for the secure server. **Required.** |
| `PN_NEX_TOKEN_AES_KEY` | 64-character hex AES-256 key used to validate NEX tokens. **Required.** |
| `PN_NEX_PASSWORD_SECRET` | At least 32 bytes, hex-encoded. Used to derive a stable per-PID NEX password (`globals.PasswordFromPID`) without storing credentials. **Required.** |

### Build and run

```bash
go build -o wii-sports-club .
./wii-sports-club
```

The console needs an account server that issues it a NEX token for this
server's access key/game server ID before it ever reaches this server —
that part is out of scope here; this is the game server, not the account
server.

---

## Credits

Built on [nex-go](https://github.com/PretendoNetwork/nex-go),
[nex-protocols-go](https://github.com/PretendoNetwork/nex-protocols-go) and
[nex-protocols-common-go](https://github.com/PretendoNetwork/nex-protocols-common-go)
by Pretendo Network.

Not affiliated with Nintendo.
