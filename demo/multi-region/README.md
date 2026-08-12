# Multi-region wave rollout

A hands-off multi-region rollout ([design doc scenario 3](../../docs/design/workflows.md))
built on `pulumi.Workflow`, deploying a real Lambda-backed service to six AWS regions.

## The graph

```
wave1 ──────────▶ wave2 ──────────▶ wave3
us-west-2         us-east-1         eu-central-1
                  eu-west-1         ap-southeast-2
                                    us-east-2
```

- Each **wave is one workflow node**. Its program is a mini Pulumi program that deploys the
  service — an IAM role, and per region a Lambda function, a public function URL, and a
  CloudWatch alarm on the function's `Errors` metric — using an explicit per-region provider.
- Each **edge gate** passes only when the release has baked in its current wave
  (`bakeSeconds`, default 120) **and** none of that wave's error alarms are in `ALARM`.
  The gate is plain Go running in the workflow program: it reads the alarm names from the
  cursor's data (they flowed there from the wave's stack exports) and calls the CloudWatch
  API directly.
- The **entry** admits one release cursor at wave1 per distinct `version`. Changing
  `version` starts a new release from wave1; per workflow semantics, if it catches up to an
  older release still on the conveyor, the older cursor is superseded.

Every wave's resources live in the main stack's state, owned by the workflow and nested
under it in `pulumi up`'s display and `pulumi stack export`.

## Running it

There is no daemon: the workflow only advances during `pulumi up`. Run it repeatedly and
watch releases walk the conveyor.

> **Backend caveat:** use a DIY backend (`pulumi login --local` or an object store). The
> Pulumi Cloud backend does not round-trip the prototype's `owner` resource field yet
> (the service re-serializes state through its vendored apitype, silently dropping the
> field), which breaks node sub-state tracking.

```sh
pulumi login --local
pulumi stack init dev
pulumi config set bakeSeconds 60        # optional; default 120

# Provide AWS credentials however you normally do; with an ESC environment, wrap each
# command in `pulumi env run`:
pulumi env run <org>/providers/aws -- pulumi up

pulumi up   # admits release v1 at wave1 (placement only; nothing deploys yet)
pulumi up   # reconciles wave1: v1's Lambda + alarm deploy to us-west-2
pulumi up   # gate polls: before bake elapses, v1 parks; after, it enters wave2
...         # keep running up until v1 reaches wave3
```

To see two releases on the conveyor at once, change the version mid-rollout:

```sh
pulumi config set version v2
pulumi up   # admits v2 at wave1 while v1 continues ahead of it
```

Each wave exports its function URLs into the cursor's data; `curl` one to see the running
version (`wf-demo v1 from us-east-1`).

## What this proves

- Nodes are real Pulumi programs deploying real cloud resources across six regions, with
  per-node sub-state reconciled on every up.
- Gates are arbitrary Go: bake clocks plus live CloudWatch alarm reads, polled once per up
  with no side effects.
- Multiple releases ride the conveyor concurrently; a newer release overtaking an older one
  supersedes it.
- Parking costs nothing: a release waiting on bake or on a firing alarm is just a row of
  data in the workflow's state.

## Cost and teardown

Lambda, function URLs, and alarms are free or fractions of a cent at demo volume.

```sh
pulumi destroy   # sweeps every wave's resources; no workflow-specific teardown
```
