# clear-thought

A lightweight session memory service built with [MCP-Go](https://github.com/mark3labs/mcp-go). It tracks sequential thoughts, mental models, and debugging approaches for agents.

## Tools

- `sequentialthinking` – record a thought with branching and revision metadata.
- `retractthought` – remove the most recent thought from session memory.
- `mentalmodel` – log a mental model used to analyze a problem.
- `debuggingapproach` – capture a systematic debugging session.

## Retract thought use cases

Typical scenarios where `retractthought` is helpful:

- Undo a mistaken or obsolete thought before proceeding.
- Free capacity when the session approaches its thought limit.
- Backtrack after exploring an unproductive line of reasoning.

