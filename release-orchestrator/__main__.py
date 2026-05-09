"""
Pulumi-on-AWS program for the pulumi/pulumi release orchestrator.

Resources:
- Lambda layer with the shared `pulumi_release` library
- One Lambda per activity
- Lambda for the WebhookRouter
- DynamoDB table for pending callbacks (TTL on expires_at)
- DynamoDB table for release holds
- Step Functions standard workflow with the orchestrator state machine
- API Gateway HTTP API endpoint for GitHub webhooks (-> WebhookRouter)
- EventBridge schedule for weekly releases (Wed 14:00 UTC)
- Secrets Manager secret for the GitHub App private key + installation id
- IAM roles, scoped per principle of least privilege

Outputs:
- Webhook URL to register on the GitHub App
- State machine ARN
- Secrets Manager ARN
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

import pulumi
import pulumi_aws as aws

# Make the in-repo src/ importable so we can call into the state_machine builder.
sys.path.insert(0, str(Path(__file__).parent / "src"))
from state_machine import builder  # noqa: E402


REPO_ROOT = Path(__file__).resolve().parent
LAMBDA_ROOT = REPO_ROOT / "src" / "lambdas"
LIB_ROOT = REPO_ROOT / "src" / "pulumi_release"

config = pulumi.Config()
github_cfg = pulumi.Config("github")

ACTIVITIES = [
    # (name, handler module, timeout_seconds, memory_mb)
    ("SurveyUpstream",         "survey_upstream",          60,  256),
    ("ComputeBumpVersions",    "compute_bump_versions",    30,  128),
    ("OpenChangelogPR",        "open_changelog_pr",        60,  256),
    ("PushChangelogHcl",       "push_changelog_hcl",       60,  256),
    ("OpenBumpPR",             "open_bump_pr",             120, 512),
    ("OpenFreezePR",           "open_freeze_pr",           120, 512),
    ("PublishRelease",         "publish_release",          30,  128),
    ("ApprovePostReleasePR",   "approve_post_release_pr",  30,  128),
    ("ApproveDownstreamBump",  "approve_downstream_bump",  30,  128),
    ("WaitForGitHubEvent",     "wait_for_github_event",    15,  128),
]


# ---- Shared library Lambda layer ----

def _build_layer() -> aws.lambda_.LayerVersion:
    """Package src/pulumi_release/ + dependencies as a Lambda layer."""
    workdir = Path(tempfile.mkdtemp(prefix="release-orch-layer-"))
    python_dir = workdir / "python"
    python_dir.mkdir()
    # Copy the library
    shutil.copytree(LIB_ROOT, python_dir / "pulumi_release")
    # Install runtime deps into the layer (boto3 is provided by the runtime,
    # but we still need cryptography and requests).
    subprocess.run(
        ["pip", "install", "--target", str(python_dir),
         "--platform", "manylinux2014_x86_64",
         "--only-binary", ":all:",
         "--python-version", "3.12",
         "cryptography>=42", "requests>=2.32"],
        check=True,
    )
    return aws.lambda_.LayerVersion(
        "pulumi-release-layer",
        layer_name="pulumi-release-layer",
        code=pulumi.AssetArchive({".": pulumi.FileArchive(str(workdir))}),
        compatible_runtimes=["python3.12"],
        compatible_architectures=["x86_64"],
    )


# ---- Secrets ----

github_secret = aws.secretsmanager.Secret(
    "github-app",
    name="release-orchestrator/github-app",
    description="Pulumi release orchestrator GitHub App credentials",
)

aws.secretsmanager.SecretVersion(
    "github-app-version",
    secret_id=github_secret.id,
    secret_string=pulumi.Output.json_dumps({
        "app_id": github_cfg.require("app_id"),
        "installation_id": github_cfg.require("installation_id"),
        "private_key": github_cfg.require_secret("private_key"),
    }),
)


# ---- DynamoDB tables ----

callbacks_table = aws.dynamodb.Table(
    "release-orchestrator-callbacks",
    name="release-orchestrator-callbacks",
    billing_mode="PAY_PER_REQUEST",
    hash_key="predicate_hash",
    range_key="task_token",
    attributes=[
        {"name": "predicate_hash", "type": "S"},
        {"name": "task_token", "type": "S"},
    ],
    ttl={"attribute_name": "expires_at", "enabled": True},
)

holds_table = aws.dynamodb.Table(
    "release-orchestrator-holds",
    name="release-orchestrator-holds",
    billing_mode="PAY_PER_REQUEST",
    hash_key="hold_id",
    attributes=[{"name": "hold_id", "type": "S"}],
    ttl={"attribute_name": "expires_at", "enabled": True},
)


# ---- IAM ----

lambda_assume = json.dumps({
    "Version": "2012-10-17",
    "Statement": [{
        "Effect": "Allow",
        "Principal": {"Service": "lambda.amazonaws.com"},
        "Action": "sts:AssumeRole",
    }],
})

lambda_role = aws.iam.Role(
    "release-orchestrator-lambda-role",
    assume_role_policy=lambda_assume,
    managed_policy_arns=["arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"],
)

lambda_inline = aws.iam.RolePolicy(
    "release-orchestrator-lambda-inline",
    role=lambda_role.id,
    policy=pulumi.Output.all(
        callbacks_table.arn, holds_table.arn, github_secret.arn,
    ).apply(lambda args: json.dumps({
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:Query",
                           "dynamodb:DeleteItem", "dynamodb:UpdateItem", "dynamodb:Scan"],
                "Resource": [args[0], f"{args[0]}/index/*", args[1]],
            },
            {
                "Effect": "Allow",
                "Action": ["secretsmanager:GetSecretValue"],
                "Resource": args[2],
            },
            {
                "Effect": "Allow",
                "Action": ["states:SendTaskSuccess", "states:SendTaskFailure",
                           "states:SendTaskHeartbeat"],
                "Resource": "*",  # tokens aren't ARNs; SF doesn't support narrower scoping here
            },
        ],
    })),
)

sfn_assume = json.dumps({
    "Version": "2012-10-17",
    "Statement": [{
        "Effect": "Allow",
        "Principal": {"Service": "states.amazonaws.com"},
        "Action": "sts:AssumeRole",
    }],
})

sfn_role = aws.iam.Role(
    "release-orchestrator-sfn-role",
    assume_role_policy=sfn_assume,
)


# ---- Lambdas ----

layer = _build_layer()


def _build_lambda_zip(handler_module: str) -> pulumi.AssetArchive:
    """Single-file zip per Lambda. Module imports from the shared layer."""
    src = LAMBDA_ROOT / handler_module / "handler.py"
    if not src.exists():
        raise FileNotFoundError(f"missing handler: {src}")
    return pulumi.AssetArchive({"handler.py": pulumi.FileAsset(str(src))})


lambda_arns: dict[str, pulumi.Output[str]] = {}
lambda_objs: dict[str, aws.lambda_.Function] = {}

for activity_name, module, timeout, memory in ACTIVITIES:
    fn = aws.lambda_.Function(
        f"release-orch-{module}",
        name=f"release-orch-{module}",
        runtime="python3.12",
        handler="handler.handle",
        role=lambda_role.arn,
        timeout=timeout,
        memory_size=memory,
        layers=[layer.arn],
        code=_build_lambda_zip(module),
        environment={
            "Variables": pulumi.Output.all(
                callbacks_table.name, holds_table.name, github_secret.arn,
            ).apply(lambda args: {
                "CALLBACKS_TABLE": args[0],
                "HOLDS_TABLE": args[1],
                "GITHUB_APP_SECRET_ARN": args[2],
            }),
        },
    )
    lambda_arns[activity_name] = fn.arn
    lambda_objs[activity_name] = fn


# Webhook router: separate Lambda, not part of the state machine.
webhook_router = aws.lambda_.Function(
    "release-orch-webhook-router",
    name="release-orch-webhook-router",
    runtime="python3.12",
    handler="handler.handle",
    role=lambda_role.arn,
    timeout=15,
    memory_size=256,
    layers=[layer.arn],
    code=pulumi.AssetArchive({"handler.py": pulumi.FileAsset(
        str(LAMBDA_ROOT / "webhook_router" / "handler.py"))}),
    environment={
        "Variables": pulumi.Output.all(
            callbacks_table.name, github_secret.arn,
        ).apply(lambda args: {
            "CALLBACKS_TABLE": args[0],
            "GITHUB_APP_SECRET_ARN": args[1],
        }),
    },
)


# ---- State Machine ----

state_machine_definition = pulumi.Output.all(
    **{name: arn for name, arn in lambda_arns.items()}
).apply(lambda arns: builder.to_json(arns))

# Allow the SF role to invoke the Lambdas
aws.iam.RolePolicy(
    "release-orchestrator-sfn-invoke",
    role=sfn_role.id,
    policy=pulumi.Output.all(*[fn.arn for fn in lambda_objs.values()]).apply(
        lambda arns: json.dumps({
            "Version": "2012-10-17",
            "Statement": [{
                "Effect": "Allow",
                "Action": "lambda:InvokeFunction",
                "Resource": list(arns),
            }],
        })
    ),
)

state_machine = aws.sfn.StateMachine(
    "release-orchestrator",
    name="release-orchestrator",
    role_arn=sfn_role.arn,
    definition=state_machine_definition,
    type="STANDARD",
    logging_configuration={
        "include_execution_data": True,
        "level": "ALL",
        "log_destination": pulumi.Output.concat(
            aws.cloudwatch.LogGroup(
                "release-orchestrator-logs",
                name="/aws/states/release-orchestrator",
                retention_in_days=90,
            ).arn, ":*"),
    },
)


# ---- API Gateway for GitHub webhooks ----

api = aws.apigatewayv2.Api(
    "release-orchestrator-webhook",
    protocol_type="HTTP",
)

integration = aws.apigatewayv2.Integration(
    "release-orchestrator-webhook-integration",
    api_id=api.id,
    integration_type="AWS_PROXY",
    integration_uri=webhook_router.invoke_arn,
    payload_format_version="2.0",
)

aws.apigatewayv2.Route(
    "release-orchestrator-webhook-route",
    api_id=api.id,
    route_key="POST /webhook",
    target=pulumi.Output.concat("integrations/", integration.id),
)

stage = aws.apigatewayv2.Stage(
    "release-orchestrator-webhook-stage",
    api_id=api.id,
    name="$default",
    auto_deploy=True,
)

aws.lambda_.Permission(
    "release-orchestrator-webhook-perm",
    action="lambda:InvokeFunction",
    function=webhook_router.name,
    principal="apigateway.amazonaws.com",
    source_arn=pulumi.Output.concat(api.execution_arn, "/*/*"),
)


# ---- EventBridge weekly schedule ----

scheduler_role = aws.iam.Role(
    "release-orchestrator-scheduler-role",
    assume_role_policy=json.dumps({
        "Version": "2012-10-17",
        "Statement": [{
            "Effect": "Allow",
            "Principal": {"Service": "scheduler.amazonaws.com"},
            "Action": "sts:AssumeRole",
        }],
    }),
)

aws.iam.RolePolicy(
    "release-orchestrator-scheduler-inline",
    role=scheduler_role.id,
    policy=state_machine.arn.apply(lambda arn: json.dumps({
        "Version": "2012-10-17",
        "Statement": [{
            "Effect": "Allow",
            "Action": "states:StartExecution",
            "Resource": arn,
        }],
    })),
)

aws.scheduler.Schedule(
    "release-orchestrator-weekly",
    name="release-orchestrator-weekly",
    schedule_expression="cron(0 14 ? * WED *)",   # Wednesdays 14:00 UTC
    schedule_expression_timezone="UTC",
    flexible_time_window={"mode": "OFF"},
    target={
        "arn": state_machine.arn,
        "role_arn": scheduler_role.arn,
        "input": "{}",
    },
)


# ---- Outputs ----

pulumi.export("webhook_url", pulumi.Output.concat(api.api_endpoint, "/webhook"))
pulumi.export("state_machine_arn", state_machine.arn)
pulumi.export("github_secret_arn", github_secret.arn)
pulumi.export("callbacks_table_name", callbacks_table.name)
