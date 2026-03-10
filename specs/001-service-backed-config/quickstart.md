# Quickstart: Service-Backed Configuration

Validation scenarios for each user story. Run after implementation to
verify end-to-end behavior.

## Prerequisites

- Pulumi CLI built from `001-service-backed-config` branch
- Logged into Pulumi Cloud (`pulumi login`)
- A Pulumi project directory (any language)

## US1: Create a Service-Backed Stack

```bash
# Create stack with explicit flag
pulumi stack init dev --remote-config

# Verify no local config file
ls Pulumi.dev.yaml  # should not exist

# Verify ESC environment was created
pulumi config env
# Expected: "Environment: <org>/<project>/dev"

# Verify basic config works
pulumi config set foo bar
pulumi config get foo
# Expected: "bar"

# Cleanup
pulumi stack rm dev --yes
```

## US2: Read and Write Config

```bash
pulumi stack init dev --remote-config

# Set plain value
pulumi config set aws:region us-west-2
pulumi config get aws:region
# Expected: "us-west-2"

# Set secret value
pulumi config set --secret dbPassword hunter2
pulumi config get --secret dbPassword
# Expected: "hunter2" (decrypted)

# List config
pulumi config
# Expected: shows source header with ESC env name, lists both values

# Remove value
pulumi config rm aws:region
pulumi config get aws:region
# Expected: error — key not found

# Set multiple
pulumi config set-all --plaintext a=1 --plaintext b=2
pulumi config rm-all a b

# Error: config cp not supported
pulumi config cp --stack other-stack
# Expected: error about service-backed stacks

# Error: --config-file not applicable
pulumi config set foo bar --config-file custom.yaml
# Expected: error about service-backed stacks

pulumi stack rm dev --yes
```

## US3: Deploy with Service-Backed Config

```bash
pulumi stack init dev --remote-config
pulumi config set message "hello from ESC"

# Preview should resolve config from ESC
pulumi preview
# Expected: program sees config value "hello from ESC"

# Conflict detection: create local file
echo 'config: {myproject:conflicting: value}' > Pulumi.dev.yaml
pulumi preview
# Expected: hard error about config conflict

# Metadata-only local file should NOT trigger conflict
echo 'encryptionsalt: v1:abc' > Pulumi.dev.yaml
pulumi preview
# Expected: no conflict error

rm Pulumi.dev.yaml
pulumi stack rm dev --yes
```

## US4: Migrate to Service-Backed Config

```bash
# Setup local stack with config
pulumi stack init dev
pulumi config set foo bar
pulumi config set --secret dbPass s3cret

# Migrate
pulumi config env init --migrate
# Expected: prompts to delete local file

# Verify config preserved
pulumi config get foo
# Expected: "bar"
pulumi config get --secret dbPass
# Expected: "s3cret"

# Verify ESC environment linked
pulumi config env
# Expected: shows ESC environment name

pulumi stack rm dev --yes
```

## US5: Eject from Service-Backed Config

```bash
pulumi stack init dev --remote-config
pulumi config set greeting hello
pulumi config set --secret apiKey abc123

# Eject
pulumi config env eject
# Expected: prompts for confirmation, secrets provider

# Verify local file created
cat Pulumi.dev.yaml
# Expected: contains greeting and encrypted apiKey

# Verify ESC link removed
pulumi config env
# Expected: shows local config file path

pulumi stack rm dev --yes
```

## US6: Pin and Restore

```bash
pulumi stack init dev --remote-config
pulumi config set version v1
pulumi config set version v2
pulumi config set version v3

# Pin to earlier revision
pulumi config pin 1
pulumi config get version
# Expected: "v1"

# Mutation should fail while pinned
pulumi config set version v4
# Expected: error — unpin first

# Unpin
pulumi config pin latest
pulumi config get version
# Expected: "v3" (latest)

# Restore old revision
pulumi config restore 1
pulumi config get version
# Expected: "v1" (new revision with old content)

pulumi stack rm dev --yes
```

## US7: Edit, View, Inspect

```bash
pulumi stack init dev --remote-config
pulumi config set foo bar

# View config source
pulumi config env
# Expected: ESC environment name
pulumi config env --json
# Expected: JSON with source, environment, version fields

# Open in browser (manual verification)
pulumi config web
# Expected: browser opens to ESC console

# Edit in $EDITOR
EDITOR=cat pulumi config edit
# Expected: prints ESC environment YAML

pulumi stack rm dev --yes
```

## US8: Stack Deletion Cleanup

```bash
pulumi stack init dev --remote-config
pulumi config set foo bar

# Delete stack — environment should be cleaned up
pulumi stack rm dev --yes

# Recreating should succeed (environment was soft-deleted)
pulumi stack init dev --remote-config
# Expected: succeeds, no 409 conflict

pulumi stack rm dev --yes
```
