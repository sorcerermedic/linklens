# Contributing

Thanks for your interest in linklens!

## Setup

Requirements: Go 1.22+, Node.js 20+, Docker (optional).

```bash
cd web && npm ci && cd ..
make run          # builds the dashboard and starts the server on :8080 with in-memory backends
```

Copy `.env.example` to `.env` (or export the variables) to point the server at PostgreSQL/Redis.

## Running checks

```bash
make lint                     # gofmt, go vet, eslint, tsc
make test                     # Go (race detector) + vitest, no external services
docker compose up -d postgres # then:
INTEGRATION=1 make test-integration
make bench                    # redirect benchmark
```

Every pull request must keep `make lint` and `make test` green.

## Commit convention

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add QR code endpoint
fix(ratelimit): refill tokens with millisecond precision
docs: explain cache invalidation
test(api): cover 410 for expired links
```

## Pull request flow

1. Fork and create a branch from `main` (`feat/short-description`).
2. Keep changes focused; add or update tests alongside code.
3. Run `make lint test` locally.
4. Open a PR describing the motivation and the change; CI must pass before review.
5. Squash-merge once approved.
