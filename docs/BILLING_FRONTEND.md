# Billing & Licensing — Frontend Integration Guide

This document is the complete contract for building the billing UI against the
bigbucks auth service. It is written to be handed to an agent or developer with
no other context.

Base path for every endpoint below: `/api/v1`.

---

## 1. The model in one paragraph

An **organization** buys a subscription. The organization — not the user — is the
billing customer; the owner's email only receives receipts. A subscription grants
two things: **entitlement** (may this organization use the app at all) and
**licenses** (how many members it may have). Purchases happen on Stripe-hosted
pages, so you never handle card details. All plan changes are made in the Stripe
billing portal, not in your UI.

**Billing is optional.** A deployment may run with it switched off. Your UI must
handle that: see [`managed_externally`](#3-get-billingsubscription).

---

## 2. Authentication

Every billing endpoint requires the standard session headers:

| Header | Value |
| --- | --- |
| `X-Auth` | The JWT, as for all other authenticated calls |
| `X-Organization-Id` | The ULID of the organization being viewed or purchased for |

`X-Organization-Id` is mandatory. Without it these endpoints return `403`.

The two mutating endpoints additionally require the `billing:*:write` permission.
In practice that means **only the Owner (and roles you have granted equivalent
permission) can start a purchase or open the portal.** Hide those buttons for
everyone else; a member without the permission gets `403`.

---

## 3. `GET /billing/subscription`

The endpoint your billing screen is built from. Call it on page load, and again
after returning from Stripe.

**Response `200`:**

```json
{
  "org_id": "01HZY3K8Q2N4T5V6W7X8Y9Z0AB",
  "entitled": true,
  "status": "active",
  "licenses": 25,
  "licenses_used": 18,
  "licenses_available": 7,
  "over_limit": false,
  "features": ["advanced_reporting", "cloud_access"],
  "plans": [
    {
      "price_id": "price_1QxTeamMonthly",
      "name": "Team",
      "quantity": 25,
      "licenses": 25,
      "status": "active",
      "cancel_at_period_end": false,
      "current_period_end": "2026-10-05T09:41:00Z"
    }
  ],
  "current_period_end": "2026-10-05T09:41:00Z",
  "cancel_at_period_end": false,
  "managed_externally": true
}
```

### Field reference

| Field | Meaning / how to use it |
| --- | --- |
| `entitled` | `false` → the organization has no active subscription. Show a paywall. |
| `status` | Raw provider status. `"none"` when nothing is held. See the table below. |
| `licenses` | Total members the plan allows. |
| `licenses_used` | Current members **plus unexpired pending invitations**. |
| `licenses_available` | `licenses - licenses_used`, never negative. `0` → disable "Invite". |
| `over_limit` | `true` after a downgrade left more members than licences. Existing members keep working; adding more is blocked. Show a persistent warning telling the admin to remove members or buy more licences. |
| `features` | Capability strings the plan unlocks. Gate optional UI on these. |
| `plans` | One entry per purchased line, for display. |
| `current_period_end` | Earliest renewal date across active lines. ISO 8601. |
| `cancel_at_period_end` | `true` → subscription lapses at `current_period_end`. Show "Cancels on {date}" with a "Resume" link to the portal. |
| **`managed_externally`** | **`false` → billing is disabled in this deployment. Hide the entire billing UI, and never show licence limits.** In this case `entitled` is `true` and `status` is `"not_managed"`. |

### `status` values

| Status | Meaning | Suggested UI |
| --- | --- | --- |
| `none` | Never subscribed | Paywall / plan picker |
| `trialing` | In trial | Badge with `current_period_end` |
| `active` | Healthy | Normal |
| `past_due` | Payment failed, Stripe is retrying | **Amber banner: "Update your payment method"** linking to the portal. Access continues by default. |
| `unpaid` / `canceled` | Access ended | Paywall |
| `paused` | Paused | Paywall |
| `incomplete` | First payment never completed | Prompt to retry checkout |
| `not_managed` | Billing disabled in this deployment | Hide billing UI |

> `past_due` still grants access (the server default is a grace period). Warn the
> user, do not lock them out.

---

## 3a. `GET /billing/plans` — the pricing page

Everything a pricing or upgrade screen needs, shaped as a **comparison table**.
Use this, not `/billing/catalog`.

`features` and `limits` list every key across the *whole* catalog, so each is a
table row; each plan then reports which of them it includes. A tier that lacks a
feature therefore renders a cross rather than the row vanishing.

**Query parameters:** `?currency=usd` — optional override for a currency
switcher. Ignored if the catalog cannot sell in it. With no override the
currency is derived from the organization's country (`X-Organization-Id`), then
the configured default.

**Response `200`:**

```json
{
  "available": true,
  "currency": "aed",
  "currencies": ["aed", "usd"],
  "currency_locked": false,
  "features": [
    { "key": "cloud_access", "label": "Cloud access & sync", "description": "Use BigBucks from any device…" },
    { "key": "advanced_reporting", "label": "Advanced reporting", "description": "Custom report builder…" },
    { "key": "api_access", "label": "API access", "description": "Programmatic access…" }
  ],
  "limits": [
    { "key": "invoices", "label": "Invoices", "unit": "invoices per month", "period": "month" },
    { "key": "skus", "label": "Products (SKUs)", "unit": "SKUs", "period": "total" }
  ],
  "plans": [
    {
      "tier": "starter",
      "name": "Starter",
      "description": "For small teams getting started.",
      "highlight": false,
      "features": ["cloud_access"],
      "limits": { "invoices": 5000, "skus": 5000 },
      "pricing": {
        "month": {
          "price_id": "price_…", "interval": "month", "default_currency": "aed",
          "amounts": {
            "aed": { "currency": "aed", "first_unit_amount": 5000,  "additional_unit_amount": 2000, "tiered": true },
            "usd": { "currency": "usd", "first_unit_amount": 1400,  "additional_unit_amount": 550,  "tiered": true }
          }
        },
        "year": {
          "price_id": "price_…", "interval": "year", "default_currency": "aed",
          "amounts": {
            "aed": { "currency": "aed", "first_unit_amount": 49900, "additional_unit_amount": 19900, "tiered": true },
            "usd": { "currency": "usd", "first_unit_amount": 13900, "additional_unit_amount": 5500,  "tiered": true }
          }
        }
      }
    },
    { "tier": "growth", "name": "Growth", "highlight": true, "…": "…" }
  ]
}
```

### Rendering rules

- **`available: false`** → billing is disabled. Do not render the page at all.
- **Plans arrive pre-sorted** cheapest first. Render in order; don't re-sort.
- **`highlight: true`** → the "most popular" treatment.
- **Amounts are in the currency's smallest unit** — fils for AED. Divide by 100.
  `5000` is **50.00 AED**, not 5000.
- **`pricing` is keyed by interval** (`month`, `year`). Drive your monthly/annual
  toggle off these keys; a tier may offer only one.
- **Then index by currency:** `plan.pricing[interval].amounts[page.currency]`.
- **`first_unit_amount` vs `additional_unit_amount`** — the plan includes one
  user at the first price, and each extra user costs the second. Render as
  *"50 AED/month, includes 1 user · +20 AED per extra user"*.
  When `tiered` is `false` both are equal; just show the one figure.

### Currency

- **`currency`** is the code to display by default — resolved from the
  organization's country, or `?currency=` if you passed one.
- **`currencies`** is every code the organization may actually buy in, for your
  switcher. A currency only some plans carry is deliberately excluded, so
  switching can never blank a column. If it has one entry, hide the switcher.
- **`currency_locked: true`** → the organization has already been invoiced and
  the provider has pinned it to one currency. `currencies` holds only that one.
  **Hide the switcher** and, if useful, explain: *"Billing currency is fixed to
  AED for your account."* Sending anything else to checkout returns `400`.
- Selecting from the switcher: re-request with `?currency=`, or just re-index
  `amounts[chosen]` client-side — both work, since every currency is returned.
- **Pass the same currency to checkout.** `POST /billing/checkout-session`
  accepts `"currency"`; omitting it charges the price's default, which may not
  be what you displayed. Sending one the price doesn't carry returns `400`.

```js
const amount = plan.pricing[interval].amounts[selectedCurrency];
// → { currency: "usd", first_unit_amount: 1400, additional_unit_amount: 550 }
```
- **Feature ticks:** `page.features` are the rows, `plan.features` (an array of
  keys) decides tick or cross.
- **Limit values:** look up `plan.limits[key]`. **`-1` means unlimited.** A key
  *missing* from `plan.limits` means that tier does not offer it at all.
- **`limits[].period`** — `"month"` resets each month (say "5,000 / month"),
  `"total"` is a standing cap (say "5,000 SKUs"). An annual subscriber still gets
  the monthly allowance monthly, not multiplied by twelve.

### Degraded mode

If the billing provider is unreachable, plans still return with features, limits
and `price_id`, but `amounts` is an **empty object** and `currencies` is empty.
Detect it with `Object.keys(detail.amounts).length === 0` and show "Contact us"
or a spinner in the price slot rather than "0 AED". Checkout still works, because
`price_id` survives.

### Labels

`label` and `description` come from backend config. If you need i18n, treat
`key` as the stable identifier and keep your own translation map, falling back to
the supplied `label`. Keys never change; labels are copy and may.

---

## 4. `GET /billing/catalog`

The raw catalog, keyed by Stripe price id. **Prefer `/billing/plans`** for
anything user-facing — this endpoint carries no amounts, no display labels and no
tier grouping. It remains useful for debugging what the backend thinks a given
price grants.

**Response `200`:**

```json
{
  "price_1QxTeamMonthly": {
    "name": "Team",
    "features": ["cloud_access"],
    "licensesPerUnit": 1,
    "includedLicenses": 0
  },
  "price_1QxBusinessMonthly": {
    "name": "Business",
    "features": ["cloud_access", "advanced_reporting"],
    "licensesPerUnit": 1,
    "includedLicenses": 5
  }
}
```

Licences granted by a plan = `includedLicenses + licensesPerUnit × quantity`.
So the Business example at quantity 10 gives `5 + 1×10 = 15`.

An **empty object `{}`** means billing is disabled. Same handling as
`managed_externally: false`.

---

## 5. `POST /billing/checkout-session` — buying

Requires `billing:*:write`.

**Request:**

```json
{
  "price_id": "price_1QxTeamMonthly",
  "quantity": 25,
  "currency": "aed",
  "success_url": "/settings/billing?checkout=success",
  "cancel_url": "/settings/billing?checkout=cancelled"
}
```

| Field | Required | Notes |
| --- | --- | --- |
| `price_id` | yes | From the catalog endpoint. |
| `quantity` | no | Number of licences. Defaults to `1`. Capped server-side. |
| `currency` | no | Must match what the page displayed. Omitting it charges the price's default. `400` if the price does not carry it, or if the organization is already locked to a different one. |
| `success_url` | no | **Path** on your UI origin. |
| `cancel_url` | no | **Path** on your UI origin. |

> **URLs are validated.** Only paths (`/settings/billing`) or absolute URLs on the
> configured UI origin are accepted. Anything pointing elsewhere is silently
> replaced with a default — this prevents the authenticated redirect being used as
> an open redirect. Always send a path.

**Response `200`:**

```json
{ "id": "cs_test_a1B2c3", "url": "https://checkout.stripe.com/c/pay/cs_test_a1B2c3" }
```

**Then redirect the browser to `url`:**

```js
const res = await fetch('/api/v1/billing/checkout-session', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'X-Auth': token,
    'X-Organization-Id': orgId,
  },
  body: JSON.stringify({
    price_id: priceId,
    quantity: seats,
    currency: selectedCurrency,
    success_url: '/settings/billing?checkout=success',
    cancel_url: '/settings/billing?checkout=cancelled',
  }),
});
const { url } = await res.json();
window.location.assign(url); // full navigation, not fetch — Stripe hosts this page
```

The buyer can adjust the licence count on Stripe's page (enabled by default), so
`quantity` is a starting value, not a ceiling.

### Critical: do not trust the success redirect

Landing on `success_url` means *Stripe finished*, **not** that the backend has
recorded the subscription — that arrives moments later over a webhook. On return:

1. Show an optimistic "Finalising your subscription…" state.
2. Poll `GET /billing/subscription` every ~2s until `entitled` becomes `true`.
3. Give up after ~30s with "This is taking longer than usual — refresh shortly."

Never write the plan into your own state from the redirect parameters.

---

## 6. The billing page — managing an existing subscription

Everything an admin needs after purchase. **You build the page; Stripe hosts the
forms.** Each action is one POST that returns a URL to redirect to.

### `POST /billing/portal-session`

Requires `billing:*:write`.

```json
{ "flow": "cancel", "return_url": "/settings/billing" }
```

| `flow` | Lands the admin on | Needs an active subscription |
| --- | --- | --- |
| *(omitted)* | Portal home — invoices, plan, payment method, cancel | no |
| `"update_plan"` | **Change plan *and* seat count** — one screen does both | yes → `409` |
| `"cancel"` | Cancellation, with any retention offer you configured | yes → `409` |
| `"payment_method"` | Card entry | no |

Response is the same `{ "id", "url" }` as checkout — redirect to `url`.

> **Buying additional seats is `update_plan`.** Stripe's update screen shows the
> quantity selector next to the plan list, so there is no separate "add seats"
> flow. Label your button "Manage licences" and send `update_plan`.

Seat and plan changes are **prorated automatically** by Stripe and arrive back as
a webhook. On return, re-poll `GET /billing/subscription` — the same
[race as checkout](#critical-do-not-trust-the-success-redirect) applies.

### `GET /billing/invoices`

Requires `billing:*:read`. Renders billing history **inside your page**, so the
admin sees recent invoices without being bounced to Stripe.

`?limit=` — default 12, max 100.

```json
[
  {
    "id": "in_1Qx…",
    "number": "BB-0042",
    "status": "paid",
    "currency": "aed",
    "total": 13000,
    "amount_paid": 13000,
    "created": "2026-09-05T09:41:00Z",
    "period_start": "2026-09-05T09:41:00Z",
    "period_end": "2026-10-05T09:41:00Z",
    "hosted_url": "https://invoice.stripe.com/i/…",
    "pdf_url": "https://pay.stripe.com/invoice/…/pdf"
  }
]
```

- Amounts are in the smallest unit — `13000` is **130.00 AED**.
- `status`: `paid`, `open`, `void`, `uncollectible`. Drafts are excluded.
- `open` means unpaid — link `hosted_url` so they can pay it.
- `pdf_url` for a download button.
- An org that never purchased returns `[]`, not an error.

### Suggested page

```
Billing
├── Plan: Growth (annual) · Renews 5 Oct 2026
│   [Change plan or licences] → portal flow "update_plan"
│   [Update payment method]   → portal flow "payment_method"
│
├── Licences  18 of 25 used            ← GET /billing/subscription
│   [Add licences] ──────────────────→ portal flow "update_plan"
│
├── Invoices                            ← GET /billing/invoices
│   BB-0042  5 Sep 2026  130.00 AED  Paid  [PDF]
│   BB-0041  5 Aug 2026  130.00 AED  Paid  [PDF]
│   [View all in billing portal] ─────→ portal, no flow
│
└── [Cancel subscription] ────────────→ portal flow "cancel"
```

### One thing you cannot prevent

The portal lets an admin **reduce seats below the current member count** —
Stripe has no idea how many members you have. The webhook lands, licences drop,
and `GET /billing/subscription` returns `over_limit: true`. Existing members keep
working; adding more is blocked. Show the persistent warning and prompt them to
either remove members or buy licences back.

---

## 7. Licence enforcement on member endpoints

Licences are consumed by **members plus pending invitations**. Two existing
endpoints now enforce this:

| Endpoint | Behaviour when out of licences |
| --- | --- |
| `POST /invitations` | `409 Conflict` |
| `POST /roles/bind-user` | `409 Conflict` |

And when the organization has no active subscription at all: **`402 Payment
Required`**.

Handle both:

```js
if (res.status === 409) {
  showDialog('No licences available. Add more to invite this person.', {
    action: 'Manage licences', onClick: openBillingPortal,
  });
} else if (res.status === 402) {
  showDialog('Your subscription is inactive.', {
    action: 'View plans', onClick: () => navigate('/settings/billing'),
  });
}
```

> Because pending invitations reserve a licence, the administrator is told at
> **invite time** rather than the invitee hitting a wall at accept time. Revoking
> a pending invitation frees its licence immediately.

Gated feature routes return `402` with a JSON body:

```json
{ "error": "organization plan does not include this feature: advanced_reporting",
  "reason": "subscription_required" }
```

---

## 8. Status code summary

| Code | Meaning | Action |
| --- | --- | --- |
| `200` | OK | — |
| `400` | Malformed body or unknown `price_id` | Fix the request |
| `401` | Missing/invalid session | Re-authenticate |
| `403` | No `X-Organization-Id`, or lacks `billing:*:write` | Hide the control |
| `402` | No active subscription, or feature not in plan | Paywall / upsell |
| `409` | No licences available | Upsell licences |
| `409` | Portal flow needs an active subscription, or no licences left | Send them to checkout / upsell |
| `501` | Billing not enabled in this deployment | Hide billing UI |
| `502` | Provider unreachable | "Try again shortly" |

---

## 9. Conditional banners

Drive these off `GET /billing/subscription`, on every billing-related screen:

| Condition | Treatment |
| --- | --- |
| `over_limit` | **Red**, persistent: "You have 3 more members than licences." → `update_plan` |
| `status === "past_due"` | **Amber**: "Payment failed — update your card." → `payment_method` |
| `cancel_at_period_end` | **Amber**: "Cancels on 5 Oct." → `update_plan` to resume |
| `!entitled` | Paywall with the plan picker from `GET /billing/plans` |
| `!managed_externally` | Hide the billing UI entirely |

**Build order:** `GET /billing/subscription` and the `managed_externally` /
`entitled` branches first; then the `409`/`402` handlers on invite; then the
pricing page and checkout; then the billing page — the portal buttons are one
redirect each, so they are the cheapest part.

---

## 10. Backend configuration (context only)

Not your concern to set, but useful to know when something misbehaves:

- Enabled via `subscriptions.enabled` / `subscriptions.mode` in `config.json`.
- `mode: "observe"` evaluates every check and logs what *would* have been denied
  while allowing everything. If limits appear not to be enforced, this is why.
- Stripe credentials come from `STRIPE_SECRET_KEY` and `STRIPE_WEBHOOK_SECRET`.
- Webhook endpoint: `POST /api/v1/billing/stripe/webhook`, verified by signature.
  Locally: `stripe listen --forward-to localhost:8000/api/v1/billing/stripe/webhook`.
