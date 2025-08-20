# Stochastic Thinking Service

`stochasticalgorithm` applies several probabilistic algorithms to decision problems.

## Algorithms and required parameters

Parameters are supplied in an object named after the selected algorithm:

| Algorithm | Required parameters |
|-----------|--------------------|
| `mdp`     | `gamma`, `states` |
| `mcts`    | `simulations`, `explorationConstant` |
| `bandit`  | `strategy`, `epsilon` |
| `bayesian`| `acquisitionFunction` |
| `hmm`     | `algorithm` |

If any required parameter is missing, the request fails with an error before the algorithm runs.

## Response fields

The tool returns the following fields:

- `summary` – brief description of what the algorithm accomplished
- `nextSteps` – suggestion for how to proceed after the algorithm run
- `hasResult` – whether a `result` field was provided in the request

`status` will be set to `success` when all required parameters are present, otherwise `failed`.
