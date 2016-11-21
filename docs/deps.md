# Mu Dependencies

This is a short note describing how dependencies are managed and resolved.  This design has been inspired by many
existing package managers, and is a mashup of the approaches taken in Go, NPM/Yarn, and Docker.

The unit of dependency in Mu is a Stack.  Each Stack is largely described by its `Mu.yaml` (or `.json`) file, although a
Stack may also carry arbitrary assets also (e.g., a program's `Dockerfile` and associated source files).

## Stack References

Each Stack reference (StackRef) combines a URL where the Stack may be downloaded from plus the desired version.

Each StackRef URL has a protocol, a base, and a name part; for example, in the Stack `https://hub.mu.com/aws/s3/bucket`,
`https://` is the protocol, `hub.mu.com/` is the base, and `aws/s3/bucket` is the name.  It is most common to simply see
the name, because Mu treats `https://hub.mu.com/` as the default protocol and base, and will use it when not stated
explicitly.  The way these URLs map to workspace paths during resolution is discussed later on.

Note that StackRef URLs may reference multiple Stacks at once.  To do so, the triple dot is appended to the URL to
be treated as a unit.  For example, `https://hub.mu.com/aws/...` refers to the entirety of the available AWS Stacks.

Each StackRef version may be either a specific Git SHA hash *or* a semantic version.  If a specific hash, the reference
is said to be "pinned" to a precise version of that Stack.  If a semantic version, the toolchain is given some "wiggle
room" in how it resolves the reference (in [the usual ways](https://yarnpkg.com/en/docs/dependency-versions)).  Git tags
are used to specify semantic versions.

It is possible, and indeed relatively common, for developers to use semantic versions in their Stacks but to pin
references by generating a so-called "lock file."  This performs the usual dependency resolution process and saves the
output so that subsequent version updates are explicit, evident, and intentional.

## Stack Resolution

Now let us see how StackRefs are resolved to content.  Dependency Stacks may be found in multiple places, and, as in
most dependency management schemes, some locations are preferred over others.

Roughly speaking, these locations are are searched, in order:

1. The current Workspace, for intra-Workspace but inter-Stack dependencies.
2. The current Workspace's `.mu/stacks/` directory.
3. The global Workspace's `.mu/stacks/` directory.
4. The Mu installation location's `$MUROOT/bin/stacks/` directory (default `/usr/local/mu`).

To be more precise, given a StackRef `r` and a workspace root `w`, we look in these locations:

1. `w/name(r)`
2. `w/.mu/stacks/base(r)/name(r)`
3. `~/.mu/stacks/base(r)/name(r)`
4. `$MUROOT/bin/stacks/base(r)/name(r)`

To illustrate this process, let us imagine we are looking up `https://hub.mu.com/aws/s3/bucket`.

In the first case, we are the author of the `aws/s3/bucket` Stack.  We therefore organize our Workspace so that it can
be found easily during resolution, eliminating the need for us to publish changes during development time:

    <Workspace>
    |   .mu/
    |   |   workspace.yaml
    |   aws/
    |   |   s3/
    |   |   |   bucket/
    |   |   |   |   Mu.yaml

The `workspace.yaml` file may optionally specify a namespace property, as in:

    namespace: aws

in which case the following simpler directory structure would also do the trick:

    <Workspace>
    |   .mu/
    |   |   workspace.yaml
    |   s3/
    |   |   bucket/
    |   |   |   Mu.yaml

In the second case, we would have probably used package management functionality like `mu get` to download dependencies.
For example, perhaps we ran:

    $ mu get https://hub.mu.com/aws/s3/bucket

in which case our local Workspace will have been populated with a copy of this Stack:

    <Workspace>
    |   .mu/
    |   |   services/
    |   |   |   hub.mu.com/
    |   |   |   |   aws/
    |   |   |   |   |   s3/
    |   |   |   |   |   |   bucket/
    |   |   |   |   |   |   |   Mu.yaml

The StackRef resolution process will find this and use it, provided, of course, that it is the right version.

In the third case, a globally available copy of the Stack has been downloaded, and placed in the `~/.mu/stacks/`
directory.  This might have been thanks to running `mu get` with the `--global` (or `-g`) flag:

    $ mu get -g https://hub.mu.com/aws/s3/bucket

The directory structure looks identical, except that instead of `<Workspace>`, it is rooted in `~/`.

## Stack Fetching

TODO(joe): describe how fetching stacks works, at a protocol level.

TODO(joe): describe how semantic versioning resolution works.

TODO(joe): describe how all of this interacts with Git repos.

