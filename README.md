# auth [![CI](https://github.com/bigbucks-solutions/auth/actions/workflows/ci.yml/badge.svg)](https://github.com/bigbucks-solutions/auth/actions/workflows/ci.yml)
Authentication and authorization backend service written in go. Cloud native design and ideally suited for side car patterns.

More Details here [https://bigbucks-solutions.github.io/auth/](https://bigbucks-solutions.github.io/auth/)

## Local Development Setup

### Prerequisites
- Docker and Docker Compose
- Go 1.23.2 or later
- OpenSSL (for generating ECS256 key pairs)
- Atlas (for database migrations)
- Pre-commit hooks (optional)

### Setup Steps

1. Clone the repository:

```bash
git clone https://github.com/bigbucks-solutions/auth.git
cd auth
```


2. Install pre-commit hooks (optional but recommended):
```bash
make install-pre-commit
```

3. Generate ECS256 key pair:
```bash
make gen-ecs256-pair
```

4. Start local dependencies (PostgreSQL, etc.):
```bash
make run-local-dependencies
```

5. Apply database migrations:
```bash
make migration-apply
```

### Development Commands

- Generate new database migration:
```bash
make migration-generate
```

- Generate API documentation:
```bash
make ci-swaggen
```

- Deploy documentation to GitHub Pages:
```bash
make gh-deploy
```

### Documentation
For detailed API documentation, visit [https://bigbucks-solutions.github.io/auth/](https://bigbucks-solutions.github.io/auth/)
