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
    "baseHost": "http://localhost:3000"
}
```

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
