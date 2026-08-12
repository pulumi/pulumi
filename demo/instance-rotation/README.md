# Fleet AMI rotation

Instance patching and AMI rotation ([design doc scenario 4](../../docs/design/workflows.md))
built on `pulumi.Workflow`, replacing real EC2 instances cluster by cluster.

## The graph

```
cluster-a ──▶ cluster-b ──▶ cluster-c
```

- Each **cluster is one workflow node** whose program pins the cluster's instances
  (`t4g.nano`, us-west-2) to the AMI the rotation cursor carries. Reconciling a cluster with
  a new AMI **replaces** its instances — the rotation is nothing but ordinary Pulumi
  replacement semantics, applied one cluster at a time.
- Each **edge gate** passes only when every instance the rotation just replaced reports both
  EC2 status checks (`system` and `instance`) as `ok` — plain Go against the EC2 API, with
  the instance ids read from the cursor's data (they flowed there from the node's exports).
  Fresh instances take a few minutes to pass checks, so the rotation naturally bakes.
- The **entry** admits one rotation cursor at cluster-a per distinct `ami` value. A rotation
  is strictly serial by construction: a cursor occupies exactly one node, and the chain is
  the only path.

The rotation is pausable and resumable for free: a cursor parked mid-fleet is just data in
the workflow's state, and the next `pulumi up` — five minutes or five days later — picks up
exactly where it left off.

## Running it

There is no daemon: the workflow only advances during `pulumi up`.

> **Backend caveat:** use a DIY backend (`pulumi login --local` or an object store). The
> Pulumi Cloud backend does not round-trip the prototype's `owner` resource field yet
> (the service re-serializes state through its vendored apitype, silently dropping the
> field), which breaks node sub-state tracking.

Pick two real AMIs to rotate between (any two bootable arm64 images work; the standard and
minimal Amazon Linux 2023 images are convenient):

```sh
AMI1=$(aws ssm get-parameter --region us-west-2 \
  --name /aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64 \
  --query Parameter.Value --output text)
AMI2=$(aws ssm get-parameter --region us-west-2 \
  --name /aws/service/ami-amazon-linux-latest/al2023-ami-minimal-kernel-default-arm64 \
  --query Parameter.Value --output text)
```

```sh
pulumi login --local
pulumi stack init dev
pulumi config set ami $AMI1
# Provide AWS credentials however you normally do; with an ESC environment, wrap each
# command in `pulumi env run <org>/providers/aws -- ...`.

pulumi up   # admits the rotation at cluster-a (placement only; nothing deploys yet)
pulumi up   # reconciles cluster-a: its instance launches on AMI1
pulumi up   # gate polls: parks until cluster-a's status checks pass (~2-3 min), then
...         # replaces cluster-b, and so on down the fleet
```

Once the fleet is on AMI1, start a rotation:

```sh
pulumi config set ami $AMI2
pulumi up   # admits a new rotation cursor at cluster-a
pulumi up   # cluster-a's instance is REPLACED on AMI2; the fleet's other clusters are untouched
...         # keep running up; each cluster is replaced only after the previous one is healthy
```

## What this proves

- Node reconciliation drives real *replacements*, not just creates: the same
  `pulumi up` rows you would see in any stack (`replacing`, `deleting original`) run inside
  a workflow node's slice of the main state.
- Strict serialization comes from the graph shape, not from locks or orchestration code.
- Long-lived, interruptible operations: the cursor persists in state between ups, so a
  rotation can be paused for days and resumed exactly.
- Health gates are arbitrary Go — here, EC2 status checks — evaluated once per up with no
  side effects.

## Cost and teardown

Three `t4g.nano` instances ≈ $0.013/hour total while the demo is up.

```sh
pulumi destroy   # sweeps every cluster's instances; no workflow-specific teardown
```
