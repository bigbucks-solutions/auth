---
layout: home
title: Usage
nav_order: 2
---

# Docker Deployment

## Prerequisites

- Docker Engine 20.10+
- Docker Compose V2+

Generate EC keys:

```bash
openssl ecparam -genkey -name prime256v1 -noout -out ec_private.pem && openssl ec -in ec_private.pem -pubout -out ec_public.pem
```

## Configuration

Create `config.docker.json`:

```json
{
    "key": "your-secret-key-here",
    "port": "9000",
    "alg": "ES256",
    "privateKey": "ec_private.pem",
    "publicKey": "ec_public.pem",
    "dBUsername": "authuser",
    "dBPassword": "your-db-password",
    "dBName": "authdb",
    "dBHost": "postgres",
    "dBPort": "5432",
    "dBSSLMode": "disable",
    "redisAddress": "redis:6379",
    "redisPassword": "",
    "redisDB": "0",
    "redisUsername": "",
    "logLevel": "debug",
    "smtpHost": "smtp.gmail.com",
    "smtpPort": "587",
    "smtpUsername": "your-email@example.com",
    "smtpPassword": "your-smtp-password",
    "smtpFrom": "noreply@example.com",
  "emailVerificationTTLSeconds": 600,
  "emailVerificationMaxAttempts": 5,
  "emailVerificationResendSeconds": 60,
  "emailVerificationHourlySendLimit": 5,
    "extraPermResources": ["orders", "reports"],
    "baseHost": "https://auth-api.example.com",
    "uiHost": "https://app.example.com"
}
```

  `uiHost` is the frontend origin used for invitation and password-reset links. It falls back to `baseHost` when omitted for backward compatibility. Do not include a trailing slash.

  Set `UI_HOST` to override this value in deployments, for example `UI_HOST=https://app.example.com`. The effective host is logged once when the service starts.

  Set `EMAIL_VERIFICATION_SECRET` to an independently generated secret of at least 32 characters. The service refuses to start without it. Generate one with `openssl rand -base64 32`; do not commit the generated value. Optional policy overrides are `EMAIL_VERIFICATION_TTL_SECONDS`, `EMAIL_VERIFICATION_MAX_ATTEMPTS`, `EMAIL_VERIFICATION_RESEND_SECONDS`, and `EMAIL_VERIFICATION_HOURLY_SEND_LIMIT`.

## GitHub Actions Configuration

The repository uses GitHub Actions. Store the HMAC secret as an Actions secret and store non-sensitive policy values as Actions variables.

### Repository-Level Configuration

In GitHub, open **Settings > Secrets and variables > Actions**.

Create this repository secret:

| Secret | Required | Value |
| --- | --- | --- |
| `EMAIL_VERIFICATION_SECRET` | Yes | A stable random value of at least 32 characters |

Create these repository variables only when overriding the application defaults:

| Variable | Default | Purpose |
| --- | ---: | --- |
| `EMAIL_VERIFICATION_TTL_SECONDS` | `600` | Code lifetime |
| `EMAIL_VERIFICATION_MAX_ATTEMPTS` | `5` | Failed attempts allowed per code |
| `EMAIL_VERIFICATION_RESEND_SECONDS` | `60` | Minimum delay before resend |
| `EMAIL_VERIFICATION_HOURLY_SEND_LIMIT` | `5` | Maximum sends per email and source IP per hour |

The secret must remain stable across deployments. Changing it invalidates every outstanding verification code.

Configure the values with GitHub CLI:

```bash
openssl rand -base64 32 | gh secret set EMAIL_VERIFICATION_SECRET

gh variable set EMAIL_VERIFICATION_TTL_SECONDS --body "600"
gh variable set EMAIL_VERIFICATION_MAX_ATTEMPTS --body "5"
gh variable set EMAIL_VERIFICATION_RESEND_SECONDS --body "60"
gh variable set EMAIL_VERIFICATION_HOURLY_SEND_LIMIT --body "5"
```

Do not print the secret, write it to `$GITHUB_OUTPUT`, place it in an artifact, or pass it as a Docker build argument. The image does not need this secret while building; inject it only when running the service.

### Environment-Level Configuration

For separate staging and production values, create GitHub environments named `staging` and `production`, then add the same secret and variables under each environment. Reference the environment from the deployment job:

```yaml
jobs:
  deploy:
    environment: production
    runs-on: ubuntu-latest
    env:
      EMAIL_VERIFICATION_SECRET: ${{ secrets.EMAIL_VERIFICATION_SECRET }}
      EMAIL_VERIFICATION_TTL_SECONDS: ${{ vars.EMAIL_VERIFICATION_TTL_SECONDS }}
      EMAIL_VERIFICATION_MAX_ATTEMPTS: ${{ vars.EMAIL_VERIFICATION_MAX_ATTEMPTS }}
      EMAIL_VERIFICATION_RESEND_SECONDS: ${{ vars.EMAIL_VERIFICATION_RESEND_SECONDS }}
      EMAIL_VERIFICATION_HOURLY_SEND_LIMIT: ${{ vars.EMAIL_VERIFICATION_HOURLY_SEND_LIMIT }}
    steps:
      - uses: actions/checkout@v7
      - name: Start application
        run: docker compose -f docker-compose.standalone.yml up -d
```

`docker-compose.standalone.yml` forwards `EMAIL_VERIFICATION_SECRET` into the `auth` container. To override the optional policy values through CI, add them to that service as well:

```yaml
services:
  auth:
    environment:
      EMAIL_VERIFICATION_SECRET: ${EMAIL_VERIFICATION_SECRET:?set EMAIL_VERIFICATION_SECRET}
      EMAIL_VERIFICATION_TTL_SECONDS: ${EMAIL_VERIFICATION_TTL_SECONDS:-600}
      EMAIL_VERIFICATION_MAX_ATTEMPTS: ${EMAIL_VERIFICATION_MAX_ATTEMPTS:-5}
      EMAIL_VERIFICATION_RESEND_SECONDS: ${EMAIL_VERIFICATION_RESEND_SECONDS:-60}
      EMAIL_VERIFICATION_HOURLY_SEND_LIMIT: ${EMAIL_VERIFICATION_HOURLY_SEND_LIMIT:-5}
```

For a remote Docker host, VM, or deployment script, ensure these variables are exported in the shell that executes `docker compose`; values set only in one Actions step do not automatically persist into another step unless they are declared under job `env` or written to `$GITHUB_ENV`.

### CI Tests

Compilation does not require the production secret. Integration tests use an isolated test-only secret configured by the test fixture. Never expose the production secret to pull-request test jobs, especially pull requests from forks. GitHub intentionally withholds repository and environment secrets from forked pull requests.

If a new runtime smoke-test job starts the real service, use a non-production CI secret:

```yaml
- name: Start auth smoke test
  env:
    EMAIL_VERIFICATION_SECRET: ${{ secrets.CI_EMAIL_VERIFICATION_SECRET }}
  run: docker compose -f docker-compose.test.yml up --abort-on-container-exit
```

Use a different `CI_EMAIL_VERIFICATION_SECRET` from staging and production. Protect production deployment with a GitHub environment approval rule and grant the deployment job only the permissions it needs.

## Docker Compose

```yaml
services:
  postgres:
    image: postgres:18
    environment:
      POSTGRES_DB: authdb
      POSTGRES_USER: authuser
      POSTGRES_PASSWORD: your-db-password
    ports:
      - "6432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U authuser"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:8.4
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

  migration:
    image: ghcr.io/bigbucks-solutions/auth:0.1.5
    depends_on:
      postgres:
        condition: service_healthy
    volumes:
      - ./config.docker.json:/app/config.json:ro
      - ./ec_private.pem:/app/ec_private.pem:ro
      - ./ec_public.pem:/app/ec_public.pem:ro
    command: ["./auth", "-c", "config.json", "migrate", "up"]
    restart: on-failure

  auth:
    image: ghcr.io/bigbucks-solutions/auth:0.1.5
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      migration:
        condition: service_completed_successfully
    ports:
      - "9000:9000"
    volumes:
      - ./config.docker.json:/app/config.json:ro
      - ./ec_private.pem:/app/ec_private.pem:ro
      - ./ec_public.pem:/app/ec_public.pem:ro
    command: ["./auth", "-c", "config.json"]
    restart: unless-stopped

volumes:
  postgres_data:
```

## Usage

Start services:
```bash
docker-compose up -d
```

View logs:
```bash
docker-compose logs -f
```

Stop services:
```bash
docker-compose down
```

## Common Commands

```bash
# Restart auth service
docker-compose restart auth

# Run migration manually
docker-compose run --rm migration ./auth -c config.json migrate up

# Access PostgreSQL
docker-compose exec postgres psql -U authuser -d authdb

# Access Redis
docker-compose exec redis redis-cli
```

```yaml
postgres:
  image: postgres:18
  environment:
    POSTGRES_PASSWORD: ${DB_PASSWORD}
```

Then create a `.env` file:
```
DB_PASSWORD=your_secure_password_here
```
