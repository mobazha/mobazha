# Self-hosting Mobazha Node

This guide builds and runs the current Mobazha Node release candidate from
source. Stable signed binaries and final production packages are not available
yet, so use testnet for evaluation.

## Requirements

- Go 1.26.4
- Git
- A supported macOS or Linux environment

## Build the node

```bash
git clone https://github.com/mobazha/mobazha.git
cd mobazha
go build -tags goolm -o mobazha .
```

The `goolm` build uses the default pure-Go crypto implementation and does not
require the optional native `libolm` dependency.

## Initialize and start on testnet

```bash
./mobazha init --testnet
./mobazha start --testnet --open
```

The first command initializes the default testnet data directory. The second
starts the node and opens the embedded Web UI. The UI and HTTP API listen on
`http://127.0.0.1:5102` by default, with API routes under `/v1/`.

Keep the generated recovery material private. Never paste seed phrases,
private keys, API tokens, or production credentials into an issue or support
request.

## Check the node

In another terminal, run:

```bash
./mobazha status --testnet
./mobazha doctor --testnet
```

For machine-readable diagnostics:

```bash
./mobazha doctor --testnet --json
```

## Use a custom data directory

Use the same `--datadir` value for initialization, startup, diagnostics, and
backups:

```bash
./mobazha init --testnet --datadir /path/to/mobazha-data
./mobazha start --testnet --datadir /path/to/mobazha-data --open
```

## Install the background service

After confirming that the foreground process starts correctly:

```bash
./mobazha service install
./mobazha service status
```

Manage it with:

```bash
./mobazha service stop
./mobazha service start
```

## Create a backup

Stop write activity before taking an operational backup, then run:

```bash
./mobazha backup --testnet --output mobazha-backup.tar.gz
```

For a custom data directory, also pass `--datadir`.

## Next steps

- [Connect Mobazha Unified during development](https://github.com/mobazha/mobazha-unified/blob/main/docs/getting-started/CONNECT_TO_NODE.md)
- [Connect an AI client or agent](../concepts/AI_AND_AGENTS.md)
- [Understand the architecture](../concepts/ARCHITECTURE.md)
- [Troubleshoot a local node](./TROUBLESHOOTING.md)
