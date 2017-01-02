# Mu Dependencies

This is a short note describing how MuPackage dependencies are managed and resolved.  This design has been inspired by
many existing package managers, and is a mashup of the approaches taken in Go, NPM/Yarn, and Docker.

## Packages

The unit of dependency in Mu is a [MuPackage](mupack.md).

Each has a `Mu.yaml` (or `.json`) manifest, which lists, among other things, that package's own set of dependencies.

Each may also carry arbitrary assets, such as a `Dockerfile` and associated source code; serverless source code
representing lambdas and API implementations; and so on.

The dependency management system is opinionated about how directories are laid out, however most MetaMus will project
MuPackage dependencies into the native package management system using proxy packages that the MetaMu compilers
understand how to recognize.  The details of how a language does this is outside of the scope of this document.

## References

Each package is referred to using a URL-like scheme, facilitating multiple package management distribution schemes.
For example, the URL `https://hub.mu.com/aws/ec2#^1.0.6` references the `aws/ec2` package on MuHub's built-in package
manager, and askes specifically for version `1.0.6` or higher using semantic versioning resolution.

Specifically, the reference has up to four parts: a protocol, base URL, name, and version:

    PackName = [ Protocol ] [ BaseURL ] NamePart [ Version ]
    Protocol = "http://" | "https://" | "ssh://" | ...
    BaseURL  = URL* . (URL | .)* "/"
    URL      = (* valid URL characters *)
    NamePart = (Identifier "/")* Identifier
    Version  = "#" (* valid version numbers *)

Although there are four parts, three of them are optional, because because Mu uses these defaults:

* `https://` is the default protocol.
* `hub.mu.com/` is the default base URL.
* `latest` is the default version number (a.k.a., "tip").

Although we're concerned with package references right now, we'll see soon that the same reference scheme is also used
to address elements exported from a package, like a module, class, function, or variable.  For example, to reference the
`VM` class from a MuIL token, we might say `https://hub.mu.com/aws/ec2/VM#^1.0.6`.  Most likely, we would leave
off the protocol, base URL, and version in the token, and leave it to the MuPackage to bind to a specific version.

The way these URLs are resolved to physical MuPackages is discussed later on.

## Versions

Each physical incarnation of a MuPackage can be tagged with one or more versions.  How this tagging process happens is
left to the specific package provider.  Each version can either be a semantic version number or arbitrary string tag.

For example, the Git provider allows dependency on a specific Git SHA hash.  For example,
`https://github.com/mu/aws/ec2#1895753f53a63c055e7cae81ebe4ea5d5805584f` depends on a MuPackage published in a GitHub
repo at commit `1895753`.  Alternatively, Git tags can be used to give MuPackages friendly names.  So, for example,
`https://github.com/mu/aws/ec2#beta1` uses on the arbitrary tag `beta1`; the same scheme can be used to denote semantic
versions simply by using Git tags, for instance `https://github.com/mu/aws/ec2#^1.0.6`.

If the reference uses a semantic version range, the toolchain is given some "wiggle room" in how it resolves the
reference (in [the usual ways](https://yarnpkg.com/en/docs/dependency-versions)).  If the reference uses a non-range
semantic version, Git commit hash, or Git tag, the reference is said to be "pinned" to a specific version.

A compiled MuPackage always contains the set of specific versions it was compiled against and can be optionally pinned
by the author.  Alternatively, the semantic version ranges can be left in, for more flexibility in dealing with diamond
dependencies in consumer code, at the risk of behavioral changes that are discovered only on the client machine.

## Package Resolution

Now let us see how references are resolved to physical MuPackages.

MuPackages may be found in multiple places, and, as in most dependency management schemes, some locations are preferred
over others.  This is to ease the task of local development while also providing rigorous dependency management.

Roughly speaking, these locations are are searched, in order:

1. The current workspace, for intra-workspace but inter-package dependencies.
2. The current workspace's `.mu/packs/` directory.
3. The global Workspace's `.mu/packs/` directory.
4. The Mu runtime libraries: `$MUROOT/lib/packs/` (default `/usr/local/mu/lib/packs`).

In each location, Mu prefers a fully qualified hit if it exists -- containing both the base of the reference plus the
name -- however, it also accept name-only hits.  This allows developers to organize their workspace without worrying
about where their MuPackages will be published.  Most of the Mu tools, however, prefer fully qualified paths.

To be more precise, given a reference `r` and a workspace root `w`, we look in these locations, in order:

1. `w/base(r)/name(r)`
2. `w/name(r)`
3. `w/.mu/packs/base(r)/name(r)`
4. `w/.mu/packs/name(r)`
5. `~/.mu/packs/base(r)/name(r)`
6. `~/.mu/packs/name(r)`
7. `$MUROOT/bin/packs/base(r)/name(r)`
8. `$MUROOT/bin/packs/name(r)`

To illustrate this process, let us imagine we are looking up the package `https://hub.mu.com/aws/ec2`.

In the illustration, let us imagine we're the author of the package, and so it is in our workspace.  We have things
organized so that it can be easily found, eliminating the need for us to frequently publish changes during development:

    <Workspace>
    |   .mu/
    |   |   workspace.yaml
    |   aws/
    |   |   ec2/
    |   |   |   Mu.yaml
    |   |   |   ...other assets...

The `workspace.yaml` file may optionally specify a "namespace" property, as in:

    namespace: aws

In this case, the following simpler directory structure would also do the trick:

    <Workspace>
    |   .mu/
    |   |   workspace.yaml
    |   ec2/
    |   |   Mu.yaml
    |   |   ...other assets...

It is possible to simplify this even further by specifying the namespace as `aws/ec2`, leading to:

    <Workspace>
    |   .mu/
    |   |   workspace.yaml
    |   Mu.yaml
    |   ...other assets...

Notice that we didn't have to mention the `hub.mu.com/` part in our workspace, although we can if we choose to.

In the second illustration, let us imagine we have used `mu get` to download the dependency from a package manager:

    $ mu get https://hub.mu.com/aws/ec2

In this case, our local workspace's package directory will have been populated with a copy of `aws/ec2`:

    <Workspace>
    |   .mu/
    |   |   packs/
    |   |   |   hub.mu.com/
    |   |   |   |   aws/
    |   |   |   |   |   ec2/
    |   |   |   |   |   |   Mu.yaml
    |   |   |   |   |   |   ...other assets...

Notice that in this case, the full base part `hub.mu.com/` is part of the path, since we downloaded it from that URL.

Now in the third and final illustration, let us imagine that we've installed a global copy of the package.  This might
have been thanks to use using `mu get`'s `--global` (or `-g`) flag:

    $ mu get --global https://hub.mu.com/aws/ec2

The directory structure will look identical to the second example, except that it is rooted in `~/`:

    ~
    |   .mu/
    |   |   packs/
    |   |   |   hub.mu.com/
    |   |   |   |   aws/
    |   |   |   |   |   ec2/
    |   |   |   |   |   |   Mu.yaml
    |   |   |   |   |   |   ...other assets...

## Fetching

TODO(joe): describe package fetching protocols.

TODO(joe): on-demand compilation (for easier Git fetching).

TODO(joe): describe how semantic versioning resolution works.

TODO(joe): describe how all of this interacts with Git repos (locally; e.g., git pull).

