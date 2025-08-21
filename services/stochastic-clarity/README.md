# stochastic-clarity

A unified server combining structured reasoning tools with stochastic decision-making algorithms.

## Tools

- `sequentialthinking` – process a stream of thoughts with branching and revision support. Responses include a `hint` with the next expected thought number.
- `getbranch` – retrieve the sequence of thoughts for a specific branch.
- `mentalmodel` – record the use of a mental model to analyze a problem.
- `debuggingapproach` – record a systematic debugging session.
- `getthoughts` – list stored thoughts. Optional `offset` and `limit` parameters paginate results.
- `getmentalmodels` – list recorded mental models. Supports `offset` and `limit`.
- `getdebuggingsessions` – list recorded debugging sessions. Supports `offset` and `limit`.
- `sessioncontext` – summary of counts and recent entries with remaining thought capacity. Helpful for a quick status update when reasoning becomes convoluted.
- `resetsession` – clear all stored thoughts, mental models, and debugging sessions to discard prior context and restore full thought capacity.
- `retractthought` – remove the most recent thought when it becomes irrelevant or incorrect.
- `stochasticalgorithm` – apply stochastic algorithms such as MDPs or MCTS to decision problems.
- `algorithmspec` – list supported stochastic algorithms and required parameters.
- `stochasticexamples` – sample requests for stochastic algorithms.
- `stochasticclarityexamples` – sample requests covering all tools.

## Typical Use Cases

- Backtrack a mistaken line of reasoning by retracting the latest thought and freeing capacity for new ideas.
- Capture and review mental models or debugging approaches used during problem solving.
- Obtain a quick snapshot of session context when reasoning becomes complex or needs summarization.
- Reset the session entirely when previous context becomes irrelevant and a fresh start is required.
