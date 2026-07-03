# AI and Agent Integrations

Mobazha Node exposes authenticated commerce capabilities to AI clients and
automation through the Model Context Protocol (MCP). The public endpoint uses
Streamable HTTP under `/v1/mcp`.

## What agents can work with

The available tool catalog depends on the connected Node, its runtime policy,
the authenticated token scopes, and enabled capabilities. Tool groups can
include catalog and listing operations, collections, seller and buyer orders,
notifications, receiving accounts, and fulfillment or sourcing integrations.

An AI client must not infer that a tool is available from source code alone.
It should discover the tools exposed by the connected Node and handle missing
or newly added tools safely.

## Connect a client

Start the Node, then list detected client integrations:

```bash
./mobazha mcp list
```

Create an appropriately scoped API token through the available administrative
AI/agent settings for your deployment, then connect a detected client:

```bash
./mobazha mcp connect --token <api-token>
```

The CLI also provides `disconnect` and a stdio bridge for clients that cannot
connect to Streamable HTTP directly:

```bash
./mobazha mcp bridge --help
./mobazha mcp disconnect --help
```

Never commit, paste into an issue, or embed an API token in a screenshot.

## Security model

- Production client connections require authentication.
- Token scopes determine which tools can be discovered and called.
- Runtime policy can restrict the tool catalog for a deployment.
- Commerce invariants remain enforced by Node application services.
- Tool calls are auditable and do not bypass payment verification, settlement
  gates, or key-custody boundaries.
- Agents do not receive raw seed phrases or private keys.

The UI for creating tokens and managing agents can differ by deployment and is
still evolving during the release-candidate period. Treat the Node's published
MCP capability and tool list as authoritative.
