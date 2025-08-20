# clear-thought

Tracks reasoning artifacts and exposes tools for inspecting an agent's thinking.

## Tools

### `sessioncontext`
Return counts and recent entries for thoughts, mental models, and debugging sessions, along with remaining thought capacity.

Agents can call this when reasoning becomes convoluted to get a quick status update.

**Arguments**

| Name | Type | Description |
|------|------|-------------|
| `limit` | `int` | Maximum number of recent items to return. Defaults to 5. |

**Output fields**

| Field | Description |
|-------|-------------|
| `thought_count` | Number of tracked thoughts. |
| `recent_thoughts` | Most recent thought snippets (newest first). |
| `mental_model_count` | Number of stored mental models. |
| `recent_mental_models` | Recent mental models (newest first). |
| `debug_session_count` | Number of debugging sessions recorded. |
| `recent_debug_sessions` | Recent debugging session descriptions (newest first). |
| `remaining_thought_capacity` | Thoughts left before reaching the limit. |

