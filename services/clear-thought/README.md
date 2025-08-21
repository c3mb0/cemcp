# clear-thought

A minimal server for managing sequential thinking, mental models, and debugging approaches.

## Tools

- `sequentialthinking` – process a stream of thoughts with branching and revision support.
- `getbranch` – retrieve the sequence of thoughts for a specific branch.
- `mentalmodel` – record the use of a mental model to analyze a problem.
- `debuggingapproach` – record a systematic debugging session.
- `getthoughts` – list stored thoughts. Optional `offset` and `limit` parameters paginate results.
- `getmentalmodels` – list recorded mental models. Supports `offset` and `limit`.
- `getdebuggingsessions` – list recorded debugging sessions. Supports `offset` and `limit`.
- `sessioncontext` – summary of counts and recent entries with remaining thought capacity. Helpful for a quick status update when reasoning becomes convoluted.
- `resetsession` – clear all stored thoughts, mental models, and debugging sessions to discard prior context and restore full thought capacity.
- `retractthought` – remove the most recent thought when it becomes irrelevant or incorrect.

## Configuration

The server supports optional configuration via command-line flags or environment variables:

- `-max-thoughts` / `CT_MAX_THOUGHTS` – maximum number of thoughts stored per session (default `100`).

Flags take precedence over environment variables.

## Typical Use Cases

- Backtrack a mistaken line of reasoning by retracting the latest thought and freeing capacity for new ideas.
- Capture and review mental models or debugging approaches used during problem solving.
- Obtain a quick snapshot of session context when reasoning becomes complex or needs summarization.
- Reset the session entirely when previous context becomes irrelevant and a fresh start is required.
