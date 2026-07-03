# Development Guide

## Prepare the repository

```bash
git clone https://github.com/mobazha/mobazha.git
cd mobazha
go build -tags goolm -o mobazha .
```

Go 1.26.4 is the supported toolchain for the current release candidate.

## Run the checks

```bash
make test
go vet -tags goolm ./...
go build -tags goolm ./...
```

If native `libolm` is installed, use `make test-libolm` for the native path.

## Run the node locally

Use testnet while developing payment and checkout flows:

```bash
./mobazha init --testnet
./mobazha start --testnet --open
```

The local gateway is available at `http://127.0.0.1:5102` by default. Public
HTTP APIs use `/v1/`, WebSocket connections use `/ws`, and the MCP endpoint is
available under `/v1/mcp` when enabled by the selected runtime policy.

## Develop the frontend

Mobazha Unified is maintained separately:

```bash
git clone https://github.com/mobazha/mobazha-unified.git
```

Follow its [frontend development guide](https://github.com/mobazha/mobazha-unified/blob/main/docs/getting-started/FRONTEND_DEVELOPMENT.md)
to connect it to this node.

## Before opening a pull request

- Add or update tests for behavior changes.
- Update public documentation when APIs, capabilities, configuration, or
  security assumptions change.
- Run the relevant release-boundary checks listed in the root README.
- Read [CONTRIBUTING.md](../../CONTRIBUTING.md) and sign off commits under the
  [Developer Certificate of Origin](../../DCO.md).
