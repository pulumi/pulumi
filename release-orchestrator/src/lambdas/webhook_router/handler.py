"""
WebhookRouter Lambda.

Sits behind an API Gateway HTTP API. Receives GitHub webhook deliveries,
validates the X-Hub-Signature-256 HMAC, computes the predicate hash, looks
up matching task tokens in the callbacks table, and calls SendTaskSuccess
on each.

Environment:
    CALLBACKS_TABLE         -- DynamoDB table name
    GITHUB_APP_SECRET_ARN   -- Secrets Manager ARN; secret JSON contains
                               'webhook_secret' for HMAC validation

Returns 200 always (acknowledged); webhook delivery semantics expect a
prompt 200. Detailed errors are logged to CloudWatch but not returned.
"""

from __future__ import annotations

import hashlib
import hmac
import json
import logging
import os
from typing import Any

import boto3

from pulumi_release import events


log = logging.getLogger()
log.setLevel(logging.INFO)

_dynamo = boto3.resource("dynamodb")
_sfn = boto3.client("stepfunctions")
_sm = boto3.client("secretsmanager")

_WEBHOOK_SECRET = None


def _webhook_secret() -> bytes:
    global _WEBHOOK_SECRET
    if _WEBHOOK_SECRET is None:
        raw = _sm.get_secret_value(SecretId=os.environ["GITHUB_APP_SECRET_ARN"])["SecretString"]
        d = json.loads(raw)
        _WEBHOOK_SECRET = d["webhook_secret"].encode()
    return _WEBHOOK_SECRET


def _verify_signature(body: bytes, signature_header: str | None) -> bool:
    if not signature_header or not signature_header.startswith("sha256="):
        return False
    expected = "sha256=" + hmac.new(_webhook_secret(), body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, signature_header)


def _resolve_token(token: str, predicate_hash: str, payload: dict[str, Any]) -> None:
    """Call SendTaskSuccess and clean up the DDB row."""
    try:
        _sfn.send_task_success(taskToken=token, output=json.dumps(payload))
    except _sfn.exceptions.TaskTimedOut:
        log.warning("token already timed out: %s", predicate_hash)
    except _sfn.exceptions.TaskDoesNotExist:
        log.warning("token unknown (already resolved?): %s", predicate_hash)
    finally:
        _dynamo.Table(os.environ["CALLBACKS_TABLE"]).delete_item(
            Key={"predicate_hash": predicate_hash, "task_token": token},
        )


def _resolve_all(predicate_hash: str, payload: dict[str, Any]) -> int:
    """Look up all task tokens for `predicate_hash` and resolve them."""
    table = _dynamo.Table(os.environ["CALLBACKS_TABLE"])
    items = table.query(
        KeyConditionExpression="predicate_hash = :h",
        ExpressionAttributeValues={":h": predicate_hash},
    )["Items"]
    for it in items:
        _resolve_token(it["task_token"], predicate_hash, payload)
    return len(items)


def _payload_summary(event_type: str, body: dict) -> dict:
    """Extract just the fields downstream activities want from the event."""
    if event_type == "pull_request":
        pr = body.get("pull_request", {})
        return {
            "pr_number": pr.get("number") or body.get("number"),
            "title": pr.get("title"),
            "merged": pr.get("merged"),
            "head_sha": pr.get("merge_commit_sha") or pr.get("head", {}).get("sha"),
        }
    if event_type == "release":
        rel = body.get("release", {})
        return {"tag": rel.get("tag_name"), "html_url": rel.get("html_url")}
    if event_type == "create":
        return {"ref": body.get("ref"), "ref_type": body.get("ref_type")}
    if event_type == "workflow_run":
        run = body.get("workflow_run", {})
        return {
            "name": run.get("name"),
            "conclusion": run.get("conclusion"),
            "head_sha": run.get("head_sha"),
            "html_url": run.get("html_url"),
        }
    return {}


def handle(api_event: dict, context) -> dict:
    headers = {k.lower(): v for k, v in (api_event.get("headers") or {}).items()}
    body_text = api_event.get("body") or ""
    if api_event.get("isBase64Encoded"):
        import base64
        body_bytes = base64.b64decode(body_text)
    else:
        body_bytes = body_text.encode()

    if not _verify_signature(body_bytes, headers.get("x-hub-signature-256")):
        log.warning("rejected webhook: bad signature")
        return {"statusCode": 401, "body": "bad signature"}

    event_type = headers.get("x-github-event", "")
    body = json.loads(body_bytes)

    primary = events.predicate_hash_from_event(body, event_type)
    if not primary:
        return {"statusCode": 200, "body": "ignored"}

    payload = _payload_summary(event_type, body)
    n = _resolve_all(primary, payload)

    # Try alternate hashes (title-based, version-based, etc.)
    for alt in events.alternate_hashes_from_event(body, event_type):
        n += _resolve_all(alt, payload)

    return {"statusCode": 200, "body": f"resolved {n} waiter(s)"}
