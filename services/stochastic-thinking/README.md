# Stochastic Thinking Service

`stochasticalgorithm` applies several probabilistic algorithms to decision problems.

## Algorithms and required parameters

| Algorithm | Required parameters |
|-----------|--------------------|
| `mdp`     | `gamma`, `states` |
| `mcts`    | `simulations`, `explorationConstant` |
| `bandit`  | `strategy`, `epsilon` |
| `bayesian`| `acquisitionFunction` |
| `hmm`     | `algorithm` |

## Response fields

The tool returns additional fields to help validate requests:

- `missingParameters` – parameters required for the selected algorithm that were not provided
- `unknownParameters` – parameters supplied in the request that are not recognized for the algorithm

If either list is non-empty, `status` will be set to `failed` and no summary is produced.
