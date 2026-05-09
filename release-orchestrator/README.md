# pulumi/pulumi release orchestrator

AWS Step Functions workflow that drives the end-to-end pulumi/pulumi
release: detect upstream language host changes, release them as needed,
open the bump PR, freeze, publish, approve the post-release PR, and
merge downstream auto-bumps. See `2026-05-08-release-step-functions-design.md`
for the design rationale.

This is intended to live in a separate `pulumi/release-orchestrator` repo
once the design is ratified. It currently sits alongside `pulumi/pulumi`
for review.

## Layout

```
.
|-- Pulumi.yaml                           Pulumi project for AWS infra
|-- __main__.py                           Pulumi program (defines all infra)
|-- src/
|   |-- pulumi_release/                   Shared library (Lambda layer)
|   |   |-- __init__.py
|   |   |-- gh.py                         GitHub App auth + REST/GraphQL helpers
|   |   |-- versions.py                   Version inference (lifted from scripts/)
|   |   |-- events.py                     Event predicate hashing
|   |-- lambdas/                          Lambda handlers (one zip each)
|   |   |-- survey_upstream/
|   |   |-- compute_bump_versions/
|   |   |-- open_changelog_pr/
|   |   |-- push_changelog_hcl/
|   |   |-- open_bump_pr/
|   |   |-- open_freeze_pr/
|   |   |-- publish_release/
|   |   |-- approve_post_release_pr/
|   |   |-- approve_downstream_bump/
|   |   |-- wait_for_github_event/
|   |   |-- webhook_router/
|   |-- state_machine/
|       |-- builder.py                    Generates ASL JSON for the workflow
|-- tests/
|   |-- unit/                             pytest suites for the shared library
|   |-- integration/                      Step Functions Local + mocked Lambda
|-- scripts/
    |-- test-local.sh                     Spin up SFN Local in Docker, run tests
```

## Local development

```sh
uv sync                          # installs the shared lib + dev deps
uv run pytest tests/unit         # unit tests
./scripts/test-local.sh          # SFN Local + mocked Lambdas (integration)
```

## Testing the state machine

Three layers:

1. **Step Functions Local** (`amazon/aws-stepfunctions-local` Docker
   image). The integration tests register the state machine, then call
   `StartSyncExecution` with synthetic inputs. Lambda integrations are
   replaced by entries in `tests/integration/MockConfigFile.json`,
   which lets each task return a sequence of mocked responses (covers
   retry / error scenarios). Examples in `tests/integration/`.
2. **Dry-run mode in real Lambdas**. Each activity reads `DRY_RUN=true`
   from its env or top-level input and returns what it *would* do
   without calling GitHub. Use to exercise the full state machine in a
   non-prod AWS account before letting it touch production.
3. **Unit tests** on the shared library and each activity's pure
   functions, using `pytest` + `responses` (HTTP mocking) + `moto`
   (AWS mocking).

## Deploying

```sh
pulumi stack init dev            # or prod
pulumi config set github:app_id <app id>
pulumi config set --secret github:private_key < path/to/private-key.pem
pulumi up
```

The Pulumi program builds Lambda zips, registers them, defines the state
machine, sets up EventBridge + DynamoDB + Secrets Manager, and prints the
webhook URL to register on the GitHub App.

## Triggering

- **Default**: weekly EventBridge schedule (Wed 14:00 UTC).
- **Manual**: `aws stepfunctions start-execution --state-machine-arn <arn> --input '{}'`.
- **Slack**: `/release` slash command -> API Gateway -> Lambda ->
  `StartExecution`. Wire up via the Slack app config.

## Operator overrides

- Stop in flight: `aws stepfunctions stop-execution --execution-arn <arn>`.
- Skip a wait: `aws stepfunctions send-task-success --task-token <token>`.
  Tokens are visible in the execution event log.
- Hold the next release: write a row to the
  `release-orchestrator-holds` DynamoDB table (or set the hold via the
  Slack `/release-hold` command). The `OpenFreezePR` activity checks
  this at start.
