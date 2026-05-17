# `memscan-mcp`

Source: [`cmd/memscan-mcp/`](https://github.com/oioio-space/maldev/tree/master/cmd/memscan-mcp) ·
godoc: [pkg.go.dev/github.com/oioio-space/maldev/cmd/memscan-mcp](https://pkg.go.dev/github.com/oioio-space/maldev/cmd/memscan-mcp)

## What it does

Command memscan-mcp is a minimal Model Context Protocol adapter that
exposes the memscan-server HTTP API as MCP tools over stdio JSON-RPC 2.0.
Claude Code launches this process, talks to it on stdin/stdout, and the
process relays each tool call to the memscan-server running inside the
Windows VM. Tools available: read_memory, find_pattern, get_module,
get_export. Each auto-attaches/detaches — Claude doesn't juggle sessions.
Wire up via .mcp.json at repo root:
	{
	  "mcpServers": {
	    "memscan": {
	      "command": "go",
	      "args": ["run", "./cmd/memscan-mcp",
	               "--server", "http://192.168.122.122:50300"]
	    }
	  }
	}
Cross-platform (pure Go HTTP client + stdio). Safe to run on Linux host
against a Windows VM memscan-server.

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o memscan-mcp.exe ./cmd/memscan-mcp
```

For platform-native builds, drop the `GOOS` / `GOARCH` prefix.

## Help / flags

Run with `-h` to see the current flag set:

```bash
./memscan-mcp -h
```

## Related

- Reference for the underlying packages: see the [Techniques tree](../techniques/).
- Runnable examples: see [Runnable examples](../examples/runnable.md).
