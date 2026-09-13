# Entitlements over gRPC

Other backend services use the `Entitlements` gRPC service to read an
organization's subscription state, plan features, licences and limits, and then
apply their own business rules. The auth service answers from its local billing
projection, so calls never reach Stripe.

The service returns **read-only snapshots**. There is no "check" RPC: the calling
service owns the usage counts, so it compares them with the snapshot itself using
the [rules below](#applying-the-rules).

The contract is `grpc-auth/entitlements.proto` (proto package
`entitlements.v1`). Generate a client from it in your service's language.

## Connecting

| Setting | Environment variable | Default | Purpose |
| --- | --- | --- | --- |
| `grpcAddress` | `GRPC_ADDRESS` | `127.0.0.1:8080` | Listen address. The default is reachable only from the same host; use `0.0.0.0:8080` in a container. |
| `grpcServiceKeys` | `GRPC_SERVICE_KEYS` | none | Keys for service callers, as `name=key,other=key`. |

The server does not terminate TLS, and service keys and user tokens are bearer
credentials. Expose the port only on a private network, or behind a mesh or proxy
that provides TLS.

### Authentication

Send one of these as gRPC metadata:

| Metadata | Caller | May query |
| --- | --- | --- |
| `x-service-key: <key>` | A backend service with no user in context | Any organization |
| `authorization: <user JWT>` | A user, forwarded by a service acting for them | Only organizations the user belongs to |

Generate one key per service, at least 32 characters:

```bash
openssl rand -base64 48
```

Only SHA-256 digests of keys are held, and they are compared in constant time.
Service keys are accepted only by the `Entitlements` service; the `Auth` service
still requires a user token. To rotate a key, add the new key under a second
name, deploy the calling service with it, then remove the old entry.

## RPCs

| RPC | Use it to |
| --- | --- |
| `GetEntitlements` | Read the full snapshot for one organization. Cache it. |
| `BatchGetEntitlements` | Read up to 100 organizations at once, for background jobs. Service keys only. |

### States

Branch on `state`. `provider_status` is Stripe's raw vocabulary, for display only.

| `state` | `entitled` | Meaning |
| --- | --- | --- |
| `NOT_MANAGED` | true | Billing is disabled in this deployment. Apply no limits. |
| `NONE` | false | Never subscribed, or the first payment never completed. |
| `TRIALING` | true | Trial running; `trial_ends_at` is set. |
| `ACTIVE` | true | Paid and current. |
| `PAST_DUE` | true | A renewal payment failed and is being retried. Keep working. |
| `CANCELED` | false | Cancelled; `ended_at` is set. |
| `EXPIRED` | false | Lapsed for another reason (unpaid, paused, not renewed); `ended_at` is set. |

A period that ends without a renewal being observed keeps granting for
`periodEndGraceHours` (48 by default), so a delayed webhook never locks out a
paying organization.

### Limits

`limits` holds every limit key the plan grants. A key missing from the map is not
offered on the plan at all, which is different from a limit of zero.

| `kind` | What you count | Resets |
| --- | --- | --- |
| `CAP` | Records currently stored, such as SKUs | Never; deleting a record frees capacity |
| `MONTHLY_QUOTA` | Records created within `[period_start, period_end)`, such as invoices | Monthly, on the subscription's billing-cycle day |

Monthly quotas reset monthly on yearly plans too. Always take the window from
the snapshot rather than computing calendar months. When several subscription
lines grant the same key, their allowances add up, and `unlimited` on any line
wins. When `unlimited` is true, `limit` is zero and carries no meaning.

## Applying the rules

Every client must implement these rules exactly, so that all services make the
same decision from the same snapshot.

**Feature** `feature` is allowed when, checked in order:

1. `managed` is false → **allow**.
2. `entitled` is false → **deny**, reason `not_entitled`.
3. `feature` is not in `features` → **deny**, reason `feature_not_in_plan`.
4. Otherwise → **allow**.

**Limit** for creating `requested` more of `key`, given `used` counted by the
caller, checked in order (treat a negative `used` or `requested` as 0):

1. `managed` is false → **allow**.
2. `entitled` is false → **deny**, reason `not_entitled`.
3. `key` is not in `limits` → **deny**, reason `limit_not_in_plan`.
4. `unlimited` is true → **allow**.
5. `used + requested <= limit` → **allow**; otherwise **deny**, reason
   `limit_exceeded`.

**Enforcement mode.** When `enforced` is false (observe mode), evaluate the rules
as above. If the result is a denial, log the reason and **allow** anyway. This
matches what the auth service's own REST checks do in observe mode, and lets you
measure the impact of a limit before enforcing it.

### Shared test cases

Clients in every language should pass this table.

| # | `managed` | `enforced` | `entitled` | `limits[key]` | `used` | `requested` | Result |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | false | false | true | missing | 10⁹ | 1 | allow |
| 2 | true | true | false | 100 | 0 | 1 | deny `not_entitled` |
| 3 | true | true | true | missing | 0 | 1 | deny `limit_not_in_plan` |
| 4 | true | true | true | unlimited | 10⁹ | 1 | allow |
| 5 | true | true | true | 100 | 99 | 1 | allow |
| 6 | true | true | true | 100 | 100 | 1 | deny `limit_exceeded` |
| 7 | true | true | true | 100 | 150 | 0 | deny `limit_exceeded` |
| 8 | true | true | true | 100 | −5 | 100 | allow (used treated as 0) |
| 9 | true | false | true | 100 | 100 | 1 | allow, log `limit_exceeded` |

| # | `managed` | `enforced` | `entitled` | `features` | Feature | Result |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | false | false | true | `[]` | `reports` | allow |
| 2 | true | true | false | `[reports]` | `reports` | deny `not_entitled` |
| 3 | true | true | true | `[cloud]` | `reports` | deny `feature_not_in_plan` |
| 4 | true | true | true | `[reports]` | `reports` | allow |
| 5 | true | false | true | `[cloud]` | `reports` | allow, log `feature_not_in_plan` |

### Example (Go)

```go
type Decision struct {
	Allowed bool
	Reason  string // empty when the rules allow
}

func CheckLimit(snapshot *authpb.OrgEntitlements, key string, used, requested int64) Decision {
	decision := evaluateLimit(snapshot, key, max(used, 0), max(requested, 0))
	if !decision.Allowed && !snapshot.Enforced {
		log.Printf("entitlements: would deny %s for org %s: %s", key, snapshot.OrgId, decision.Reason)
		decision.Allowed = true
	}
	return decision
}

func evaluateLimit(snapshot *authpb.OrgEntitlements, key string, used, requested int64) Decision {
	switch limit, offered := snapshot.Limits[key]; {
	case !snapshot.Managed:
		return Decision{Allowed: true}
	case !snapshot.Entitled:
		return Decision{Reason: "not_entitled"}
	case !offered:
		return Decision{Reason: "limit_not_in_plan"}
	case limit.Unlimited, used+requested <= limit.Limit:
		return Decision{Allowed: true}
	default:
		return Decision{Reason: "limit_exceeded"}
	}
}
```

For a monthly quota, count within the snapshot's window before deciding:

```go
snapshot := cache.Get(ctx, orgID) // GetEntitlements, cached briefly
if quota, offered := snapshot.Limits["invoices"]; offered && !quota.Unlimited {
	used := countInvoices(orgID, quota.PeriodStart.AsTime(), quota.PeriodEnd.AsTime())
	if decision := CheckLimit(snapshot, "invoices", used, 1); !decision.Allowed {
		return planLimitError(decision.Reason)
	}
}
```

Skip counting when the key is unlimited or not offered; the rules decide both
without a count. When the key is not offered, still call `CheckLimit` so the
denial and its reason are consistent.

These checks take no locks, so two concurrent creates can both pass with one unit
left. Where an exact cap matters, re-count inside the transaction that creates
the record, or accept a small overshoot.

## Caching and failures

- Cache a snapshot per organization for 30–60 seconds; `resolved_at` tells you
  its age. A plan change reaches you within that window.
- Resolve at most once per request, not once per record.
- Use a short deadline, such as 500 ms.
- On `UNAVAILABLE` or `DEADLINE_EXCEEDED`, keep using the last snapshot for a few
  minutes. Once it is too old, decide per operation whether to fail open or
  closed; blocking reads during a billing outage is rarely right.

| gRPC code | Cause |
| --- | --- |
| `UNAUTHENTICATED` | Missing, unknown or invalid key or token. |
| `PERMISSION_DENIED` | User not a member of `org_id`, a service key on an `Auth` RPC, or a user calling `BatchGetEntitlements`. |
| `INVALID_ARGUMENT` | Missing `org_id`; empty, blank or more than 100 batch ids. |
| `INTERNAL` | The auth service could not read billing state. Retry, or use a cached snapshot. |

## Regenerating code

Inside this repository:

```bash
protoc --go_out=. --go-grpc_out=. grpc-auth/entitlements.proto
```

In another Go module, override the import path when generating:

```bash
protoc --go_out=. --go_opt=Mgrpc-auth/entitlements.proto=example.com/yourservice/authpb \
  --go-grpc_out=. --go-grpc_opt=Mgrpc-auth/entitlements.proto=example.com/yourservice/authpb \
  grpc-auth/entitlements.proto
```
