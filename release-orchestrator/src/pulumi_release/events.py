"""
Event predicate hashing.

`WaitForGitHubEvent` activities write `(predicate_hash, task_token)` rows to
DynamoDB. The `WebhookRouter` Lambda computes the same hash from incoming
GitHub webhook events and queries the table to find waiting tokens.

The hash is a deterministic string -- not cryptographic -- intended to
collide *exactly* for matching event/predicate pairs. Both producers
(WaitForGitHubEvent activity, predicate side) and consumers (WebhookRouter,
event side) call into this module so the shapes can't drift.
"""

from __future__ import annotations

from typing import Any


def predicate_hash_from_predicate(predicate: dict[str, Any]) -> str:
    """Hash from a predicate dict, as written by WaitForGitHubEvent."""
    event = predicate["event"]
    repo = predicate.get("repo")
    if event in ("pull_request.merged", "pull_request.opened"):
        if "pr_number" in predicate:
            return f"pr-{event.split('.')[1]}:{repo}:{predicate['pr_number']}"
        if "title" in predicate:
            return f"pr-{event.split('.')[1]}-by-title:{repo}:{predicate['title']}"
        if "title_contains" in predicate:
            return f"pr-{event.split('.')[1]}-title-contains:{repo}:{predicate['title_contains']}"
    if event == "release.published":
        return f"release-published:{repo}:{predicate['tag']}"
    if event == "create":
        # tag creation. ref_type is required.
        return f"tag-created:{repo}:{predicate['ref']}"
    if event == "workflow_run.completed":
        # head_sha keys for build-test waits, version keys for release.yml waits
        wf = predicate.get("workflow", "")
        if "head_sha" in predicate:
            return f"workflow-completed:{repo}:{wf}:{predicate['head_sha']}"
        if "version" in predicate:
            return f"workflow-completed-version:{repo}:{wf}:{predicate['version']}"
    raise ValueError(f"unsupported predicate {predicate!r}")


def predicate_hash_from_event(event: dict[str, Any], event_type: str) -> str | None:
    """
    Hash from an incoming GitHub webhook event payload. Returns None if the
    event isn't one we care about (the router skips it).

    `event_type` is the value of the `X-GitHub-Event` header
    (e.g. 'pull_request', 'release', 'create', 'workflow_run').
    """
    repo = event.get("repository", {}).get("full_name")

    if event_type == "pull_request":
        action = event.get("action")
        pr = event.get("pull_request", {})
        number = event.get("number") or pr.get("number")
        title = pr.get("title", "")

        if action == "closed" and pr.get("merged") is True:
            # number-based wait
            return f"pr-merged:{repo}:{number}"
        if action == "opened":
            # Some waits key on title; the router emits both
            # primary (number) hashes and secondary (title) hashes; the
            # router queries one then the other so a single waiting wait
            # only needs to register one shape.
            return f"pr-opened:{repo}:{number}"
        return None

    if event_type == "release":
        if event.get("action") == "published":
            tag = event.get("release", {}).get("tag_name")
            return f"release-published:{repo}:{tag}"
        return None

    if event_type == "create":
        if event.get("ref_type") == "tag":
            return f"tag-created:{repo}:{event['ref']}"
        return None

    if event_type == "workflow_run":
        if event.get("action") == "completed":
            run = event.get("workflow_run", {})
            return (
                f"workflow-completed:{repo}:{run.get('name')}:{run.get('head_sha')}"
            )
        return None

    return None


def alternate_hashes_from_event(event: dict[str, Any], event_type: str) -> list[str]:
    """
    For events that can be matched by multiple keys (e.g. PR opened by
    number OR by title), return the additional hashes to query alongside
    the primary one. WebhookRouter looks up all of them in DynamoDB.
    """
    if event_type == "pull_request" and event.get("action") == "opened":
        repo = event.get("repository", {}).get("full_name")
        title = event.get("pull_request", {}).get("title", "")
        out = [f"pr-opened-by-title:{repo}:{title}"]
        # title_contains: any waiter whose substring is in title
        # We can't reverse-look-up substrings cheaply; the router scans a
        # bounded recent set instead -- see WebhookRouter.handler.
        return out
    if event_type == "workflow_run" and event.get("action") == "completed":
        # release.yml is keyed by version; we read that out of the run name
        # if it includes the version (release.yml passes 'version' as input).
        return []   # version-based hash needs additional metadata; populated later
    return []
