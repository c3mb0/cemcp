# stochastic-thinking

A brainstorming microservice built with [MCP-Go](https://github.com/mark3labs/mcp-go). It offers lightweight tools for generating stochastic ideas and variations.

## Tools

### `st_brainstorm`
Generate multiple ideas around a topic.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `topic` | string | yes | Subject to explore. |
| `count` | number | no | Number of ideas to return (default 5). |

**Example payload**

```json
{
  "tool": "st_brainstorm",
  "topic": "user engagement features",
  "count": 3
}
```

### `st_variations`
Create variations of a statement using stochastic sampling.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `seed` | string | yes | Starting text. |
| `temperature` | number | no | Sampling temperature (default 1.0). |

**Example payload**

```json
{
  "tool": "st_variations",
  "seed": "launch a new product",
  "temperature": 0.8
}
```

## Usage

Start the service over stdio:

```bash
go run ./services/stochastic-thinking/main.go --model openai:gpt-4
```

The main program calls `server.ServeStdio` and streams JSON over stdin/stdout.

## Configuration

- `--model` flag or `ST_MODEL` environment variable selects the backing language model.
- `--max-tokens` flag or `ST_MAX_TOKENS` environment variable sets the response size limit.
