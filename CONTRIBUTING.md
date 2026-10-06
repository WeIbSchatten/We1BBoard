# Contributing

## Dev

```bash
cd frontend && npm ci && npm run build
go build -o we1bboard ./cmd/we1bboard
./we1bboard run
```

## Release

1. Push to `main`
2. Create and push a tag: `git tag v1.0.0 && git push origin v1.0.0`
3. GitHub Actions builds linux amd64/arm64 archives and publishes a Release

## Server update

```bash
we1bboard update
we1bboard legacy    # pin version
we1bboard rollback  # previous binary
```

## Install (VPS)

```bash
bash <(curl -Ls https://raw.githubusercontent.com/WeIbSchatten/We1BBoard/main/install.sh)
```
