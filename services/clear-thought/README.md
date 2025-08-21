# clear-thought

A clarity-oriented microservice built with [MCP-Go](https://github.com/mark3labs/mcp-go). It focuses on summarization and structural outlining.

## Tools

### `ct_summarize`
Summarize a block of text.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `text` | string | yes | Content to summarize. |
| `style` | string | no | Tone of summary (e.g., "bullet"). |

**Example payload**

```json
{
  "tool": "ct_summarize",
  "text": "long passage",
  "style": "bullet"
}
```

### `ct_outline`
Generate an outline from notes.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `text` | string | yes | Source material. |
| `depth` | number | no | Outline depth (default 2). |

**Example payload**

```json
{
  "tool": "ct_outline",
  "text": "notes about project milestones",
  "depth": 3
}
```

## Usage

Run the service over stdio:

```bash
go run ./services/clear-thought/main.go --model openai:gpt-4o-mini
```

The program uses `server.ServeStdio` to exchange JSON over stdin/stdout.

## Configuration

- `--model` flag or `CT_MODEL` environment variable sets the model.
- `--max-tokens` flag or `CT_MAX_TOKENS` environment variable sets the response size limit.
