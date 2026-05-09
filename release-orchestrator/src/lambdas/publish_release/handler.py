"""
PublishRelease Lambda.

Finds the draft release matching `version` on pulumi/pulumi, validates it,
and PATCHes it to publish: draft=false, prerelease=false, make_latest=true,
discussion_category_name=Announcements.

Idempotent: re-running on an already-published release returns the
existing release_id / html_url without modifying anything.

Input: { "version": "v3.235.0" }   (or "3.235.0"; v is optional)
Output: { "release_id": 12345, "html_url": "https://github.com/..." }
"""

from __future__ import annotations

import os

from pulumi_release import gh, versions


REPO = "pulumi/pulumi"
DISCUSSION_CATEGORY = "Announcements"


def _find_release(tag: str) -> dict | None:
    # /releases/tags/<tag> resolves only published; we need to scan to find drafts.
    try:
        return gh.get(f"repos/{REPO}/releases/tags/{tag}").json()
    except Exception:
        pass
    # Fall back to listing.
    items = gh.get(f"repos/{REPO}/releases?per_page=50").json()
    for it in items:
        if it.get("tag_name") == tag:
            return it
    return None


def handle(event: dict, context) -> dict:
    version = event["version"]
    tag = versions.with_v(version)

    rel = _find_release(tag)
    if rel is None:
        raise RuntimeError(f"PublishRelease: no release found for {tag}")

    if not rel.get("draft"):
        # Already published; just return the existing record.
        return {
            "release_id": rel["id"],
            "html_url": rel["html_url"],
            "already_published": True,
        }

    if os.environ.get("DRY_RUN") == "true" or event.get("dry_run"):
        return {"release_id": rel["id"], "html_url": rel["html_url"], "dry_run": True}

    fields = {
        "draft": False,
        "prerelease": False,
        "make_latest": "true",
        "discussion_category_name": DISCUSSION_CATEGORY,
    }
    updated = gh.patch(f"repos/{REPO}/releases/{rel['id']}", json_body=fields).json()
    if updated.get("draft"):
        raise RuntimeError(f"PublishRelease: PATCH returned draft=true; {updated}")
    return {"release_id": updated["id"], "html_url": updated["html_url"]}
