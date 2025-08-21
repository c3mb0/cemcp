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
- `resetsession` – clear all stored thoughts, mental models, and debugging sessions to discard prior context and report remaining capacity.

