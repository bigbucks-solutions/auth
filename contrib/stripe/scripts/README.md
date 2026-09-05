# Stripe catalog provisioning

`provision-default-catalog.sh` creates or reuses the default BigBucks Starter
and Growth products and their monthly/yearly graduated prices. It also updates
the active default Stripe Customer Portal configuration so customers can switch
plans and change licence quantity with prorations.

## Requirements

- [Stripe CLI](https://docs.stripe.com/stripe-cli), authenticated with `stripe login` or `STRIPE_API_KEY`
- `jq`

The CLI account and mode determine where resources are created. Use a test-mode key for development and QA, and a live-mode key for production.

## Run

```bash
./contrib/stripe/scripts/provision-default-catalog.sh
```

The script writes `subscriptions.generated.json` in the current directory. Price lookup keys and product metadata make repeated runs reuse existing active resources.

The Stripe Customer Portal must have been saved at least once in the Dashboard
so an active default configuration exists. To provision only the catalog and
leave portal settings unchanged, run with `CONFIGURE_PORTAL=false`.

Default AED amounts are in fils:

| Tier | Interval | First seat | Additional seat |
| --- | --- | ---: | ---: |
| Starter | month | 5000 | 2000 |
| Starter | year | 49900 | 19900 |
| Growth | month | 9000 | 3500 |
| Growth | year | 89900 | 34900 |

Override an amount or output path when needed:

```bash
GROWTH_MONTHLY_BASE_AED=9500 \
OUTPUT=subscriptions.qa.json \
./contrib/stripe/scripts/provision-default-catalog.sh
```

The generated file deliberately excludes credentials. For a local file, add `secretKey` and `webhookSecret` under `options`, or provide `STRIPE_SECRET_KEY` and `STRIPE_WEBHOOK_SECRET`. For QA and production, inject the completed JSON through `SUBSCRIPTIONS_CONFIG_JSON`.

The seven-day trial is created by Checkout, not on the Stripe Price. Override it with `TRIAL_PERIOD_DAYS`; set it to `0` to disable trials in the generated configuration.
