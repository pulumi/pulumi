"""
WaitForGitHubEvent Lambda.

Invoked from the state machine with `.waitForTaskToken`. Records a row in
DynamoDB:
    PK: predicate_hash       (stable across producer/consumer)
    SK: task_token           (Step Functions task token)
    predicate (full JSON, for debugging)
    expires_at (TTL)

The state-machine execution parks until the WebhookRouter resolves the
token via SendTaskSuccess.

Input shape (delivered by SF):
    {
      "predicate": {                 # see pulumi_release/events.py
        "repo": "pulumi/pulumi",
        "event": "pull_request.merged",
        "pr_number": 22833
      },
      "taskToken": "AAA..."
    }
"""

from __future__ import annotations

import json
import os
import time

import boto3

from pulumi_release import events


_dynamo = boto3.resource("dynamodb")


def handle(event: dict, context) -> dict:
    predicate = event["predicate"]
    token = event["taskToken"]

    # Resolve any "$.field"-suffix indirection that ASL passed through. ASL
    # JSONPath substitution writes the resolved value, but if a predicate
    # field still contains "$." it means the upstream activity didn't
    # populate it. Fail fast.
    for k, v in predicate.items():
        if isinstance(v, str) and v.startswith("$."):
            raise ValueError(f"unresolved predicate field {k}={v!r}")

    h = events.predicate_hash_from_predicate(predicate)
    table = _dynamo.Table(os.environ["CALLBACKS_TABLE"])
    table.put_item(Item={
        "predicate_hash": h,
        "task_token": token,
        "predicate": json.dumps(predicate),
        "created_at": int(time.time()),
        "expires_at": int(time.time()) + 7 * 24 * 3600,    # 7-day TTL safety
    })
    # Return immediately. SF parks the execution waiting on the token.
    return {}
