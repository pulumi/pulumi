# Coconut Dependencies

This is a short note describing how CocoPack dependencies are managed and resolved.  This design has been inspired by
many existing package managers, and is a mashup of the approaches taken in Go, NPM/Yarn, and Docker.

## Packages

The unit of dependency in Coconut is a package, encoded in the [CocoPack](nutpack.md) format.

Each has a `Coconut.yaml` (or `.json`) manifest, which lists, among other things, that package's own set of dependencies.

Each may also carry arbitrary assets, such as a `Dockerfile` and associated source code; serverless source code
representing lambdas and API implementations; and so on.

The dependency management system is opinionated about how directories are laid out, however most CocoLangs will project
CocoPack dependencies into the native package management system using proxy packages that the CocoLang compilers
understand how to recognize.  The details of how a language does this is outside of the scope of this document.

## References

Each package is referenced using a URL-like scheme, facilitating multiple package management distribution schemes.
For example, the URL `https://cocohub.com/aws#^1.0.6` references the `aws` package on CocoHub's built-in package
manager, and askes specifically for version `1.0.6` or higher using semantic versioning resolution.  Note that package
names may have multiple parts, delimited by `/`, as part of the URL; for example `https://cocohub.com/a/b/c`.

Specifically, the reference has up to four parts: a protocol, base URL, name, and version:

    PackName = [ Protocol ] [ BaseURL ] NamePart [ Version ]
    Protocol = "http://" | "https://" | "ssh://" | ...
    BaseURL  = URL* "." (URL | ".")* "/"
    URL      = (* valid URL characters *)
    NamePart = (Identifier "/")* Identifier
    Version  = "#" (* valid version numbers *)

Although there are four parts, three of them are optional, because because Coconut uses these defaults:

* `https://` is the default protocol.
* `cocohub.com/` is the default base URL.
* `latest` is the default version number (a.k.a., "tip").

Although we're concerned with package references right now, we'll see soon that a similar reference scheme is used
to address elements exported from a package, like a module, class, function, or variable.  The package part of the
reference uses the above grammar, however members inside of it are preceded by a `:`.  Furthermore, such references do
not have version numbers.  These references are not strictly URLs and must be interpreted by the Coconut toolchain:

    MemberName = [ Protocol ] [ BaseURL ] NamePart MemberPart
    MemberPart = ":" NamePart

For example, to reference the `VM` class from a CocoIL token -- assuming we have a dependency declared on
`https://cocohub.com/aws#^1.0.6` as shown above -- we would most likely say `aws:ec2/VM`.  A fully qualified, but
versionless, reference is also permitted, as in `https://cocohub.com/aws:ec2/VM`, although this is less conventional.
The self-referential package plus module identifier `.` can be used to reference members in the current package.

The way these URLs are resolved to physical CocoPacks is discussed later on in this document.

## Versions

Each physical CocoPack can be tagged with one or more versions.  How this tagging process happens is left to the
specific package provider.  Each version can either be a semantic version number or arbitrary string tag.

For example, the Git provider allows dependency on a specific Git SHA hash.  For example,
`https://github.com/coconut/aws/ec2#1895753f53a63c055e7cae81ebe4ea5d5805584f` depends on a CocoPack published in a GitHub
repo at commit `1895753`.  Alternatively, Git tags can be used to give CocoPacks friendly names.  So, for example,
`https://github.com/coconut/aws/ec2#beta1` uses on the arbitrary tag `beta1`; the same scheme can be used to denote
semantic versions simply by using Git tags, for instance `https://github.com/coconut/aws/ec2#^1.0.6`.

### Flexible vs. Pinned

If the reference uses a semantic version range, the toolchain is given some wiggle room in how it resolves the
reference (in [the usual ways](https://yarnpkg.com/en/docs/dependency-versions)).  Such a reference is said to be
"flexible."  If the reference uses a non-range semantic version, Git commit hash, or Git tag, on the other hand, the
reference is said to be "pinned" to a specific version and can never bind to anything else.

At development-time, flexible versions are nice, because you're often getting the latest-and-greatest that a library
has to offer, without having to spend a great deal of time manually managing version numbers.  Flexible versions are
also nice for libraries, as the package manager can resolve multiple close, but different, semantic versions of a given
library to a single physical incarnation of it.   But when it comes to managing a production system, flexible versions
can cause problems, since upgrading to a new version may change a topology unexpectedly and/or at an inopportune time.

It is up to you, the package author, to decide whether to use flexible or pinned versions.  The recommended practice is,
however, for package manifests to specify flexible semantic version ranges.  This ensures development-time is flexible.
These package manifests should be published as-is, permitting more versioning flexibility on the consumer side.
Blueprints, however, should be pinned to specific versions, both in version control, and in the default developer
workflow.  This pinning is important to ensure that deployments are repeatable, and is encouraged by the command line
tools; in particular, generating a plan automatically first generates a so-called "pinfile."

### Pinning

A pinfile exists alongside a package's manifest to lock all versions to specific pinned versions; it is called
`Cocodeps.yaml` (or `Cocodeps.json`).  Using a separate pinfile enables an independent pinning step without modifying
the package manifest, which can continue using flexible versions if appropriate.  This file contains the entire
transitive closure of a package, its dependencies, their dependencies, and so on, pinned to specific versions.

All CLI commands respect the pinfile when it exists.  There are CLI commands to manage this pinfile too.  For example:

* `coco pin` will generate a new pinfile.
* `coco upgrade` will upgrade all packages to new versions where available.
* And so on.

To encourage the recommended workflow above, a few other behaviors are worth noting.

First, all deployment CLI commands will generate a pinfile if it doesn't exist (`coco plan`, `coco apply`, etc).  This
ensures that blueprints are generally pinned to versions during, between, and after deployment activities.  Such
pinfiles should be checked into blueprint source control repositories and versioned intentionally.  The option
`--no-pin` suppresses the automatic generation of pinfiles fo rthe deployment commands.

Second, publishing a library package, by default, omits the pinfile.  This encourages publication of libraries that are
not pinned to specific versions.  If that is desired, the pinned versions belong in the package manifest.

## Package Resolution

Now let us see how references are resolved to physical CocoPacks.

CocoPacks may be found in multiple places, and, as in most dependency management schemes, some locations are preferred
over others.  This is to ease the task of local development while also providing rigorous dependency management.

Roughly speaking, these locations are are searched, in order:

1. The current workspace, for intra-workspace but inter-package dependencies.
2. The current workspace's `.coconut/packs/` directory.
3. The global Workspace's `.coconut/packs/` directory.
4. The Coconut runtime libraries: `$COCOROOT/lib/packs/` (default `/usr/local/coconut/lib/packs`).

In each location, Coconut prefers a fully qualified hit if it exists -- containing both the base of the reference plus
the name -- however, it also accept name-only hits.  This allows developers to organize their workspace without worrying
about where their CocoPacks will be published.  Most of the Coconut tools, however, prefer fully qualified paths.

To be more precise, given a reference `r` and a workspace root `w`, we look in these locations, in order:

1. `w/base(r)/name(r)`
2. `w/name(r)`
3. `w/.coconut/packs/base(r)/name(r)`
4. `w/.coconut/packs/name(r)`
5. `~/.coconut/packs/base(r)/name(r)`
6. `~/.coconut/packs/name(r)`
7. `$COCOROOT/lib/base(r)/name(r)`
8. `$COCOROOT/lib/name(r)`

To illustrate this process, let us imagine we are looking up the package `https://cocohub.com/aws/ec2`.

In the illustration, let us imagine we're the author of the package, and so it is in our workspace.  We have things
organized so that it can be easily found, eliminating the need for us to frequently publish changes during development:

    <Workspace>
    |   .coconut/
    |   |   workspace.yaml
    |   aws/
    |   |   ec2/
    |   |   |   Coconut.yaml
    |   |   |   ...other assets...

The `workspace.yaml` file may optionally specify a "namespace" property, as in:

    namespace: aws

In this case, the following simpler directory structure would also do the trick:

    <Workspace>
    |   .coconut/
    |   |   workspace.yaml
    |   ec2/
    |   |   Coconut.yaml
    |   |   ...other assets...

It is possible to simplify this even further by specifying the namespace as `aws/ec2`, leading to:

    <Workspace>
    |   .coconut/
    |   |   workspace.yaml
    |   Coconut.yaml
    |   ...other assets...

Notice that we didn't have to mention the `cocohub.com/` part in our workspace, although we can if we choose to.

In the second illustration, let us imagine we have used `coco get` to download the dependency from a package manager:

    $ coco get https://cocohub.com/aws/ec2

In this case, our local workspace's package directory will have been populated with a copy of `aws/ec2`:

    <Workspace>
    |   .coconut/
    |   |   packs/
    |   |   |   cocohub.com/
    |   |   |   |   aws/
    |   |   |   |   |   ec2/
    |   |   |   |   |   |   Coconut.yaml
    |   |   |   |   |   |   ...other assets...

Notice that in this case, the full base part `cocohub.com/` is part of the path, since we downloaded it from that URL.

Now in the third and final illustration, let us imagine that we've installed a global copy of the package.  This might
have been thanks to use using `coco get`'s `--global` (or `-g`) flag:

    $ coco get --global https://cocohub.com/aws/ec2

The directory structure will look identical to the second example, except that it is rooted in `~/`:

    ~
    |   .coconut/
    |   |   packs/
    |   |   |   cocohub.com/
    |   |   |   |   aws/
    |   |   |   |   |   ec2/
    |   |   |   |   |   |   Coconut.yaml
    |   |   |   |   |   |   ...other assets...

## Fetching

TODO(joe): describe package fetching protocols.

TODO(joe): on-demand compilation (for easier Git fetching).

TODO(joe): describe how semantic versioning resolution works.

TODO(joe): describe how all of this interacts with Git repos (locally; e.g., git pull).

