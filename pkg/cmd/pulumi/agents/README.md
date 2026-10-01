# Pulumi Agents A2A client

`pulumi agents a2a` uses the active Pulumi CLI backend and login to call a hosted A2A 1.0 service. The backend must provide the Pulumi Agents A2A routes. The command does not start a local agent server.

```sh
pulumi agents a2a discover --org acme --json
pulumi agents a2a card <agent-id> --org acme --json
pulumi agents a2a send <agent-id> --org acme --request request.json --json
pulumi agents a2a get <task-id> --agent <agent-id> --org acme --json
pulumi agents a2a list --agent <agent-id> --org acme --page-size 50 --json
pulumi agents a2a watch <task-id> --agent <agent-id> --org acme --json
pulumi agents a2a cancel <task-id> --agent <agent-id> --org acme --json
```

The service catalog supplies agent IDs, including Custom Agents. Agent Cards select the JSON-RPC interface. The CLI accepts only interfaces on the active backend's origin, under its Pulumi Agents API path.

`send --stream` uses `SendStreamingMessage`. `watch` uses `SubscribeToTask`. Each SSE data event becomes one JSON line containing the A2A StreamResponse. `--json` uses compact JSON for other results. Without it, results use indented JSON. Protocol errors retain their code, message, and error data on stderr. A failed task remains a task result, which is distinct from a failed API request.

The request file is an A2A SendMessageRequest. Use `--request -` to read it from stdin. `configuration.returnImmediately` defaults to false under A2A 1.0. The CLI preserves the supplied value. `--return-immediately` explicitly sets it to true.

The hosted service waits for up to one minute on a blocking send or cancellation. A wait timeout includes `taskId` and `contextId` in the protocol error data. Use `get` or `watch` with that task ID to check the outcome. A send timeout does not cancel the task.

`get` and `list` accept `--history-length`, including zero. `list` also accepts `--context`, `--status`, `--status-timestamp-after`, `--include-artifacts`, and `--page-token`. Use the same filters when continuing a page.

## Codex and Claude Code

Export the shared delegation skill to the documented project skill directory for [Codex](https://learn.chatgpt.com/docs/build-skills#where-codex-loads-local-skills) or [Claude Code](https://code.claude.com/docs/en/skills#where-skills-live):

```sh
# Codex
mkdir -p .agents/skills/pulumi-agents-a2a
pulumi agents a2a skill > .agents/skills/pulumi-agents-a2a/SKILL.md

# Claude Code
mkdir -p .claude/skills/pulumi-agents-a2a
pulumi agents a2a skill > .claude/skills/pulumi-agents-a2a/SKILL.md
```

The skill teaches discovery, delegation, task recovery, and approval replies. It contains no credentials and does not require native A2A support in either client. The `skill` command only prints instructions. It does not require a login or organization.
