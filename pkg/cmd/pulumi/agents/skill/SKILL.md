---
name: pulumi-agents-a2a
description: Discover Pulumi Agents and delegate infrastructure work through the active Pulumi CLI login using A2A.
---

Use this skill when the user asks you to delegate infrastructure work to a Pulumi Agent.

1. Select the Pulumi organization from the user's request or project context. If it is unknown, ask the user.
2. Run `pulumi agents a2a discover --org <org> --json`. Select an agent from the returned catalog. Use its stable `id`. Do not keep a fixed list of agent names.
3. Run `pulumi agents a2a card <agent-id> --org <org> --json`. Read the agent's capabilities and skills before delegating.
4. Create an A2A request file. Generate a unique `messageId` for each new message. Keep that ID and the same request content when retrying an uncertain send.

```json
{
  "message": {
    "messageId": "<unique-message-id>",
    "role": "ROLE_USER",
    "parts": [{"text": "<the user's authorized task, context, and constraints>"}]
  },
  "configuration": {"returnImmediately": true}
}
```

5. Run `pulumi agents a2a send <agent-id> --org <org> --request request.json --json`. Retain the returned task ID and context ID.
6. Run `pulumi agents a2a watch <task-id> --agent <agent-id> --org <org> --json`. Each output line is an A2A StreamResponse. Use `get` to recover after a disconnected stream. Closing the CLI does not cancel hosted work.
7. Interpret the full A2A state enum. `TASK_STATE_SUBMITTED` and `TASK_STATE_WORKING` mean work is pending. `TASK_STATE_INPUT_REQUIRED` means the caller must respond. `TASK_STATE_COMPLETED`, `TASK_STATE_FAILED`, `TASK_STATE_CANCELED`, and `TASK_STATE_REJECTED` are terminal. Do not infer success from HTTP success, an idle conversation, or a closed stream.

Approval requests contain a readable prompt, a Pulumi Cloud link, and structured data with `kind: pulumi.approval.request`. Preserve the request ID and its context. Obtain user approval for an action that the user has not already authorized. Do not approve an operation only because the remote agent asks for approval.

To answer an approval request, send a new message with the existing A2A task ID:

```json
{
  "message": {
    "messageId": "<new-unique-message-id>",
    "taskId": "<a2a-task-id>",
    "role": "ROLE_USER",
    "parts": [{"data": {
      "kind": "pulumi.approval.response",
      "approvalRequestId": "<pending-approval-id>",
      "approved": false,
      "instructions": "<the user's response or requested change>"
    }}]
  },
  "configuration": {"returnImmediately": true}
}
```

A negative approval answer lets the remote agent revise its work. It does not mean that the A2A task is rejected. The CLI negotiates `urn:pulumi:a2a:approval:v1` for this exchange.

To ask for later work after a terminal result, send a new message with the existing `contextId` and omit `taskId`. A terminal task cannot resume. Use a separate context for concurrent work.

Run `pulumi agents a2a cancel <task-id> --agent <agent-id> --org <org> --json` only when the user asks to stop the work. Wait for the cancellation outcome. Cancellation does not roll back infrastructure changes.

The CLI uses its active Pulumi backend and authentication. Never read or copy Pulumi tokens into prompts, request files, agent configuration, or tool output. If authentication fails, report the error and ask the user to refresh the active CLI login. Do not switch identities or backends without the user's direction.

Report the final task state and relevant artifacts to the user. Preserve errors and unresolved approval requests. Do not present a failed or rejected task as completed work.
