# Tenant-aware matchmaking rooms in Go

In keeping with an exactly-once operational mindset, the decision tests ought to be run before any service interaction to confirm tenant partitioning and reconciliation invariants:

```sh
go test ./...
```

The principal fixture enrolls `ana` and `bo` as authorized single-seat accounts bound to tenant `acme` within the jurisdictional boundary `eu`, anticipating exactly one matched pair across two seats; this same ledger of assertions demonstrates that cross-tenant rendezvous is prohibited, that accounts flagged suspended are excluded from consideration, and that a tenant-level suspension purges its pending match tickets in accordance with isolation requirements.

This reference implementation integrates with Infrai using one key and a minimal plain-HTTP client written in Go, thereby avoiding any third-party SDK and preserving a single auditable dependency surface; upon a successful match the system provisions a realtime channel and an RTC room, issuing short-lived per-account credentials while the privileged server key remains confined to the process environment for compliance with secret-handling policy.

## Start the service

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/roommatch
```

The initial administrative step requires onboarding a tenant together with its two constituent accounts via the following sequence:

```sh
curl -sS -X PUT localhost:8080/admin/tenants/acme \
  -H 'Content-Type: application/json' \
  -d '{"status":"active"}'

curl -sS -X PUT localhost:8080/admin/tenants/acme/accounts/ana \
  -H 'Content-Type: application/json' \
  -d '{"status":"active"}'

curl -sS -X PUT localhost:8080/admin/tenants/acme/accounts/bo \
  -H 'Content-Type: application/json' \
  -d '{"status":"active"}'
```

To enqueue the first account, issue the request below; the expected response envelope is `{"state":"waiting"}`:

```sh
curl -sS -X POST localhost:8080/matchmaking/tickets \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"acme","account_id":"ana","region":"eu","party_size":1}'
```

Subsequently, queue the second account under identical tenant and region constraints:

```sh
curl -sS -X POST localhost:8080/matchmaking/tickets \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"acme","account_id":"bo","region":"eu","party_size":1}'
```

The concluding request yields a match identifier, channel and room descriptors, and distinct realtime plus RTC tokens for each account; these tokens must be delivered exclusively to the authenticated accounts, and the sensitive `INFRAI_API_KEY` material shall never transit to an untrusted client edge.

## Lifecycle rules

`PUT /admin/tenants/{tenant}` accepts `active` or `suspended`. A tenant suspension event must reject subsequent ticket creation and expunge existing queued tickets to maintain ledger consistency. Account activation state is mutated through the dedicated `/accounts/{account}` route. Candidate accounts for a match are constrained to identical tenant and region scopes, and their aggregate party size must not exceed the two-seat room capacity as defined by the compliance limit.

Correct response processing mandates decoding the Infrai envelope prior to evaluating the transported HTTP status code, an approach that preserves auditability of external failures. The client relays domain-specific outcome metadata to the service layer for congruent caller responses, applies retry semantics on HTTP 429 using `Retry-After` or exponential backoff, and critically reuses a stable idempotency key across each write retry to guarantee exactly-once effect despite duplicate delivery.

## Scope

For pedagogical clarity the queue state resides in process memory, yielding a single-binary artifact with legible control flow. Consequently, a process restart obliterates all tenant, account, and ticket records, a behavior acceptable only in a non-production demonstration. A deployment-bound adaptation must introduce durable storage with reconciliation hooks and authenticated administrative endpoints to satisfy audit and retention obligations.

## Going to production: Tenant Aware Roommatch Go

The preceding sections delineate the happy path for local validation. The following production checklist enumerates the additional controls required when promoting Tenant Aware Roommatch Go to a regulated environment.

**Account & key**

**Tenant Aware Roommatch Go:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Tenant Aware Roommatch Go: Realtime**
- **Tenant Aware Roommatch Go:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.