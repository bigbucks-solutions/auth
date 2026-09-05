#!/usr/bin/env bash
set -euo pipefail

command -v stripe >/dev/null 2>&1 || {
    echo "stripe CLI is required: https://docs.stripe.com/stripe-cli" >&2
    exit 1
}
command -v jq >/dev/null 2>&1 || {
    echo "jq is required" >&2
    exit 1
}

OUTPUT=${OUTPUT:-subscriptions.generated.json}
TRIAL_PERIOD_DAYS=${TRIAL_PERIOD_DAYS:-7}
CONFIGURE_PORTAL=${CONFIGURE_PORTAL:-true}

STARTER_MONTHLY_BASE_AED=${STARTER_MONTHLY_BASE_AED:-5000}
STARTER_MONTHLY_SEAT_AED=${STARTER_MONTHLY_SEAT_AED:-2000}
STARTER_YEARLY_BASE_AED=${STARTER_YEARLY_BASE_AED:-49900}
STARTER_YEARLY_SEAT_AED=${STARTER_YEARLY_SEAT_AED:-19900}
GROWTH_MONTHLY_BASE_AED=${GROWTH_MONTHLY_BASE_AED:-9000}
GROWTH_MONTHLY_SEAT_AED=${GROWTH_MONTHLY_SEAT_AED:-3500}
GROWTH_YEARLY_BASE_AED=${GROWTH_YEARLY_BASE_AED:-89900}
GROWTH_YEARLY_SEAT_AED=${GROWTH_YEARLY_SEAT_AED:-34900}

find_or_create_product() {
    local tier=$1
    local name=$2
    local product_id

    product_id=$(stripe products list --limit 100 |
        jq -r --arg tier "$tier" '[.data[] | select(.metadata.bigbucks_tier == $tier and .active == true) | .id] | first // empty')
    if [[ -z "$product_id" ]]; then
        product_id=$(stripe products create \
            --name "$name" \
            -d "metadata[bigbucks_tier]=$tier" |
            jq -r '.id')
        echo "Created $name product: $product_id" >&2
    else
        echo "Reusing $name product: $product_id" >&2
    fi
    printf '%s' "$product_id"
}

find_or_create_price() {
    local product_id=$1
    local lookup_key=$2
    local interval=$3
    local base_amount=$4
    local seat_amount=$5
    local price_id

    price_id=$(stripe prices list --limit 100 |
        jq -r --arg key "$lookup_key" '[.data[] | select(.lookup_key == $key and .active == true) | .id] | first // empty')
    if [[ -z "$price_id" ]]; then
        price_id=$(stripe prices create \
            --product "$product_id" \
            --currency aed \
            -d "recurring[interval]=$interval" \
            -d "billing_scheme=tiered" \
            -d "tiers_mode=graduated" \
            -d "tiers[0][up_to]=1" \
            -d "tiers[0][flat_amount]=$base_amount" \
            -d "tiers[0][unit_amount]=0" \
            -d "tiers[1][up_to]=inf" \
            -d "tiers[1][unit_amount]=$seat_amount" \
            -d "lookup_key=$lookup_key" |
            jq -r '.id')
        echo "Created $lookup_key: $price_id" >&2
    else
        echo "Reusing $lookup_key: $price_id" >&2
    fi
    printf '%s' "$price_id"
}

starter_product=$(find_or_create_product starter "BigBucks Starter")
growth_product=$(find_or_create_product growth "BigBucks Growth")

starter_monthly=$(find_or_create_price "$starter_product" bigbucks_starter_monthly month "$STARTER_MONTHLY_BASE_AED" "$STARTER_MONTHLY_SEAT_AED")
starter_yearly=$(find_or_create_price "$starter_product" bigbucks_starter_yearly year "$STARTER_YEARLY_BASE_AED" "$STARTER_YEARLY_SEAT_AED")
growth_monthly=$(find_or_create_price "$growth_product" bigbucks_growth_monthly month "$GROWTH_MONTHLY_BASE_AED" "$GROWTH_MONTHLY_SEAT_AED")
growth_yearly=$(find_or_create_price "$growth_product" bigbucks_growth_yearly year "$GROWTH_YEARLY_BASE_AED" "$GROWTH_YEARLY_SEAT_AED")

if [[ "$CONFIGURE_PORTAL" == "true" ]]; then
    portal_config=$(stripe billing_portal configurations list --limit 100 |
        jq -r '[.data[] | select(.is_default == true and .active == true) | .id] | first // empty')
    if [[ -z "$portal_config" ]]; then
        echo "No active default Stripe Customer Portal configuration found." >&2
        echo "Open Stripe Dashboard > Settings > Billing > Customer portal once, save it, then rerun this script." >&2
        exit 1
    fi

    stripe billing_portal configurations update "$portal_config" \
        -d "features[subscription_update][enabled]=true" \
        -d "features[subscription_update][default_allowed_updates][0]=price" \
        -d "features[subscription_update][default_allowed_updates][1]=quantity" \
        -d "features[subscription_update][proration_behavior]=create_prorations" \
        -d "features[subscription_update][products][0][product]=$starter_product" \
        -d "features[subscription_update][products][0][prices][0]=$starter_monthly" \
        -d "features[subscription_update][products][0][prices][1]=$starter_yearly" \
        -d "features[subscription_update][products][1][product]=$growth_product" \
        -d "features[subscription_update][products][1][prices][0]=$growth_monthly" \
        -d "features[subscription_update][products][1][prices][1]=$growth_yearly" \
        >/dev/null
    echo "Configured Customer Portal plan and quantity updates: $portal_config" >&2
fi

jq -n \
    --arg starter_monthly "$starter_monthly" \
    --arg starter_yearly "$starter_yearly" \
    --arg growth_monthly "$growth_monthly" \
    --arg growth_yearly "$growth_yearly" \
    --arg trial_days "$TRIAL_PERIOD_DAYS" \
    '{
        enabled: true,
        mode: "enforce",
        provider: "stripe",
        options: {
            trialPeriodDays: $trial_days
        },
        currencies: {
            default: "aed",
            byCountry: {AE: "aed"}
        },
        features: {
            cloud_access: {label: "Cloud access & sync"},
            advanced_reporting: {label: "Advanced reporting", order: 1}
        },
        limits: {
            invoices: {label: "Invoices"},
            skus: {label: "Products (SKUs)", order: 1}
        },
        catalog: {
            ($starter_monthly): {
                name: "Starter", tier: "starter", licensesPerUnit: 1,
                features: ["cloud_access"], monthlyQuotas: {invoices: 5000}, caps: {skus: 5000}
            },
            ($starter_yearly): {
                name: "Starter", tier: "starter", licensesPerUnit: 1,
                features: ["cloud_access"], monthlyQuotas: {invoices: 5000}, caps: {skus: 5000}
            },
            ($growth_monthly): {
                name: "Growth", tier: "growth", order: 1, licensesPerUnit: 1,
                features: ["cloud_access", "advanced_reporting"], monthlyQuotas: {invoices: 10000}, caps: {skus: 10000}
            },
            ($growth_yearly): {
                name: "Growth", tier: "growth", order: 1, licensesPerUnit: 1,
                features: ["cloud_access", "advanced_reporting"], monthlyQuotas: {invoices: 10000}, caps: {skus: 10000}
            }
        }
    }' > "$OUTPUT"

printf 'Wrote %s\n' "$OUTPUT"
printf 'Supply Stripe credentials separately with STRIPE_SECRET_KEY and STRIPE_WEBHOOK_SECRET.\n'
