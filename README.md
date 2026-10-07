# inventory

A demo service for the [service registry POC](https://github.com/mcafeelabs/services).

`POST /restock?sku=` publishes `stock.changed` on NATS with the request's baggage. `GET /stock` shows the current levels.

## Files

- `service.yaml`: what inventory provides and consumes. This repo is the source of truth.
- `config-values.schema.yaml`: JSON Schema for its config. Fields marked `x-ref` only accept the listed reference kinds.
- `main.go`: built on [`servicekit`](https://github.com/mcafeelabs/platform/tree/main/pkg/servicekit). The kit reads `CONFIG_FILE` and forwards the `sandbox` baggage on every HTTP and NATS hop.

## CI

1. Runs `go test` and `svcreg lint-service` (manifest and schema checked against the chart's `capabilities.yaml`).
2. Pushes the image `ghcr.io/mcafeelabs/inventory`: `0.1.<run>` on main (Kargo promotes semver tags) and `pr-<n>` on same-repo PRs (for sandboxes).
3. On main, opens a PR to `mcafeelabs/services` with `service.yaml`, the schema and the new image tag for directly delivered environments. That PR auto-merges once its checks pass.

   This step needs a `SERVICES_REPO_TOKEN` secret with contents and pull-request write access to `mcafeelabs/services`. Without it, the step is skipped with a notice.

## Run locally

```sh
go test ./...
SERVICE_NAME=inventory CONFIG_FILE=config.yaml go run .
```
