# Mu Dependencies

This is a short note describing how dependencies are managed and resolved.  This design has been inspired by many
existing package managers, and is a mashup of the approaches taken in Go, NPM/Yarn, and Docker.

The unit of dependency in Mu is a Stack.  Each Stack is largely described by its `Mu.yaml` (or `.json`) file, although a
Stack may also carry arbitrary assets also (e.g., a program's `Dockerfile` and associated source files).

## Stack References

Each Stack reference (StackRef) combines a URL where the Stack may be downloaded from plus the desired version.

Each StackRef URL has a protocol, a base URL, a name, and a version; for example, in the Stack
`https://hub.mu.com/aws/s3/bucket@^1.0.6`, `https://` is the protocol, `hub.mu.com/` is the base, `aws/s3/bucket` is the
name, and `^1.0.6` is the version.  It is common to simply see the name, because Mu has reasonable defaults:

* `https://` is the default protocol.
* `hub.mu.com/` is the default base URL.
* `latest` is the default version number (meaning "tip").

Each StackRef version may be either a specific Git SHA hash or a semantic version range.  If a specific hash, the
reference is said to be "pinned" to a precise version of that Stack.  If a semantic version, the toolchain is given some
"wiggle room" in how it resolves the reference (in [the usual ways](https://yarnpkg.com/en/docs/dependency-versions)).
Git tags are used to specify semantic versions.  After compilation, all StackRefs will have been pinned.

The way these URLs map to workspace paths during resolution is discussed later on.

## Stack Resolution

Now let us see how StackRefs are resolved to content.  Dependency Stacks may be found in multiple places, and, as in
most dependency management schemes, some locations are preferred over others.

Roughly speaking, these locations are are searched, in order:

1. The current Workspace, for intra-Workspace but inter-Stack dependencies.
2. The current Workspace's `.mu/stacks/` directory.
3. The global Workspace's `.mu/stacks/` directory.
4. The Mu installation location's `$MUROOT/bin/stacks/` directory (default `/usr/local/mu`).

In each location, we prefer a fully qualified hit if it exists -- containing both the base of the reference plus the
name -- however, we also accept name-only hits.  This allows developers to organize their workspace without worrying
about where their Mu Stacks are hosted.  Most of the Mu tools, however, prefer fully qualified paths.

To be more precise, given a StackRef `r` and a workspace root `w`, we look in these locations, in order:

1. `w/base(r)/name(r)`
2. `w/name(r)`
3. `w/.mu/stacks/base(r)/name(r)`
4. `w/.mu/stacks/name(r)`
5. `~/.mu/stacks/base(r)/name(r)`
6. `~/.mu/stacks/name(r)`
7. `$MUROOT/bin/stacks/base(r)/name(r)`
8. `$MUROOT/bin/stacks/name(r)`

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

Notice that we didn't have to mention the `hub.mu.com/` part in our Workspace, although we can if we choose to.

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

Notice that in this case, the full base part `hub.mu.com/` is part of the path, since we downloaded it.  The StackRef
resolution process will find this and use it, provided, of course, that it is the right version.

In the third case, a globally available copy of the Stack has been downloaded, and placed in the `~/.mu/stacks/`
directory.  This might have been thanks to running `mu get` with the `--global` (or `-g`) flag:

    $ mu get -g https://hub.mu.com/aws/s3/bucket

The directory structure looks identical, except that instead of `<Workspace>`, it is rooted in `~/`.

## Stack Fetching

TODO(joe): describe how fetching stacks works, at a protocol level.

TODO(joe): describe how semantic versioning resolution works.

TODO(joe): describe how all of this interacts with Git repos.

