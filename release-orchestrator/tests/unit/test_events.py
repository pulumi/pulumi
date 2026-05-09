from pulumi_release import events


def test_pr_merged_predicate_hash():
    h = events.predicate_hash_from_predicate({
        "repo": "pulumi/pulumi",
        "event": "pull_request.merged",
        "pr_number": 22833,
    })
    assert h == "pr-merged:pulumi/pulumi:22833"


def test_pr_merged_event_hash():
    h = events.predicate_hash_from_event(
        {
            "action": "closed",
            "number": 22833,
            "pull_request": {"merged": True, "number": 22833, "title": "..."},
            "repository": {"full_name": "pulumi/pulumi"},
        },
        event_type="pull_request",
    )
    assert h == "pr-merged:pulumi/pulumi:22833"


def test_release_published_round_trip():
    p = events.predicate_hash_from_predicate({
        "repo": "pulumi/pulumi-yaml",
        "event": "release.published",
        "tag": "v1.33.0",
    })
    e = events.predicate_hash_from_event(
        {
            "action": "published",
            "release": {"tag_name": "v1.33.0"},
            "repository": {"full_name": "pulumi/pulumi-yaml"},
        },
        event_type="release",
    )
    assert p == e == "release-published:pulumi/pulumi-yaml:v1.33.0"


def test_tag_created_round_trip():
    p = events.predicate_hash_from_predicate({
        "repo": "pulumi/pulumi",
        "event": "create",
        "ref_type": "tag",
        "ref": "pkg/v3.236.0",
    })
    e = events.predicate_hash_from_event(
        {"ref": "pkg/v3.236.0", "ref_type": "tag",
         "repository": {"full_name": "pulumi/pulumi"}},
        event_type="create",
    )
    assert p == e == "tag-created:pulumi/pulumi:pkg/v3.236.0"


def test_workflow_run_completed_by_sha():
    p = events.predicate_hash_from_predicate({
        "repo": "pulumi/pulumi",
        "event": "workflow_run.completed",
        "workflow": "build-test",
        "head_sha": "abc123",
    })
    e = events.predicate_hash_from_event(
        {
            "action": "completed",
            "workflow_run": {"name": "build-test", "head_sha": "abc123"},
            "repository": {"full_name": "pulumi/pulumi"},
        },
        event_type="workflow_run",
    )
    assert p == e == "workflow-completed:pulumi/pulumi:build-test:abc123"


def test_unknown_event_returns_none():
    assert events.predicate_hash_from_event(
        {"action": "labeled", "repository": {"full_name": "pulumi/pulumi"}},
        event_type="pull_request",
    ) is None
