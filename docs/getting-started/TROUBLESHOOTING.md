# Troubleshooting a Local Node

## Start with diagnostics

```bash
./mobazha status --testnet
./mobazha doctor --testnet
```

Export a diagnostic bundle only after reviewing it for information you do not
want to share:

```bash
./mobazha doctor --testnet --export mobazha-diagnostics.tar.gz
```

Never publish credentials, API tokens, seed phrases, private keys, customer
data, private RPC URLs, or unreviewed diagnostic bundles.

## The Web UI does not open

- Confirm the node process is still running.
- Check `./mobazha status --testnet`.
- Open `http://127.0.0.1:5102` directly.
- Check whether another process already uses the configured gateway port.
- When using a custom data directory, pass the same `--datadir` to every
  command.

## The frontend cannot reach the node

- Confirm the API URL is `http://127.0.0.1:5102` unless you changed it.
- Keep the Node and Unified testnet/deployment configuration consistent.
- Do not expose the administrative API publicly while troubleshooting.
- Review browser CORS errors only after confirming the backend is reachable.

## Payments do not appear

Payment availability is the intersection of the node's runtime capabilities,
seller configuration, provider health, and the current checkout session. A
frontend cannot enable a payment method that the node did not advertise.

## Report a problem

Search existing GitHub issues before opening a new one. Include the Mobazha
commit or version, operating system, exact command, expected result, and a
sanitized error message. Report suspected vulnerabilities privately according
to [SECURITY.md](../../SECURITY.md).
