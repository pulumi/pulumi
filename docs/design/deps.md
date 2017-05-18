# Lumi Dependencies

This is a short note describing how LumiPack dependencies are managed and resolved.  This design has been inspired by
many existing package managers, and is a mashup of the approaches taken in Go, NPM/Yarn, and Docker.

## Packages

The unit of dependency in Lumi is a package, encoded in the [LumiPack](packages.md) format.

Each has a manifest file, which lists, among other things, that package's own set of dependencies.  It is largely
derived from the source program's `Lumi.json` (or `.yaml`) file, but is named `Lumipack.json` (or `.yaml`), and
contains additional metadata inserted by the respective LumiLang compiler.  This includes LumiIL artifacts.

Each package may also carry arbitrary assets, such as a `Dockerfile`, associated source code, shredded serverless
source code representing lambdas and RPC or web API implementations, and so on.  This is described elsewhere.

The dependency management system is opinionated about how directories are laid out, however most LumiLangs will project
LumiPack dependencies into the native package management system using proxy packages that the LumiLang compilers
understand how to recognize.  The details of how a language does this is outside of the scope of this document.

### Dependency References

Each package manifest is required to declare its dependencies.  This occurs in a `dependencies` section with each entry
having a key name equal to the simple name of the package, and value equal to its complete package reference URL:

    "name": "acmecorp/package",
    "dependencies": {
        "lumi": "...",
        "lumijs": "...",
        "aws": "...",
        "acmecorp/utils": "..."
    },
    ...

Each package reference is a URL in order to facilitate multiple package management distribution schemes.

For example, the URL `https://lumihub.com/aws#^1.0.6` references the `aws` package on LumiHub's built-in package
manager, and asks specifically for version `1.0.6` or higher using semantic versioning resolution.  Note that package
names may have multiple parts, delimited by `/`, as part of the URL; for example `https://lumihub.com/a/b/c`.

Specifically, the reference has up to four parts: a protocol, base URL, name, and version:

    PackName = [ [ Protocol ] [ BaseURL ] [ NamePart ] "#" ] Version
    Protocol = "http://" | "https://" | "ssh://" | "git://" | ...
    BaseURL  = URL* "." (URL | ".")* "/"
    URL      = (* valid URL characters *)
    NamePart = (Identifier "/")* Identifier
    Version  = (* valid version numbers *)

Although there are four parts, all but the version are optional, because because Lumi uses these defaults:

* `https://` is the default protocol.
* `lumihub.com/` is the default base URL.
* The package name from the key is the default name.

Note that the `#` preceding the version is only required if the protocol, base URL, and/or name parts are provided.

### Package Member Tokens

Although we're concerned with package references right now, it's worth noting that references to elements within a
package use LumiIL tokens.  These tokens have a package part that must match a key in the enclosing package's manifest.
For example, `aws:ec2/instance:Instance` refers to the class `Instance`, in the module `ec2/instance`, in the package
`aws`.  It is this package part that must be matched to a package in the dependency list.

## Versions

Each physical LumiPack can be tagged with one or more versions.  How this tagging process happens is left to the
specific package provider.  Each version can either be a semantic version number or arbitrary string tag.

It is possible to request the "latest" version instead of a specific one.  This is convenient for development scenarios
but can be dangerous in production scenarios, because dependency updates may imply resource changes.  To specify the
latest available package, use either `latest` or the shortcut `*`.  Lumi will always attempt to bind the to latest.

For example, the Git provider allows dependency on a specific Git SHA hash.  So,
`git://github.com/lumi/aws#1895753f53a63c055e7cae81ebe4ea5d5805584f` refers to a package published in a GitHub
repo `lumi/aws` at commit `1895753`.  Alternatively, Git tags can be used to give packages friendly names.  So, for
example, `git://github.com/lumi/aws#beta1` refers to a package published in a GitHub repo `lumi/aws` at the tag
`beta1`.  The same scheme can be used to denote semantic versions simply by using numeric semantic version Git tags, for
instance `git://github.com/lumi/aws#^1.0.0-beta1` refers to a version of the package of at least `1.0.0-beta1`.

### Flexible versus Pinned Versions

If the reference uses a semantic version range, the toolchain is given some wiggle room in how it resolves the
reference (in [the usual ways](https://yarnpkg.com/en/docs/dependency-versions)).  Such a reference is said to be
"flexible."  If the reference uses a non-range semantic version, Git commit hash, or non-semantic version range Git tag,
on the other hand, the reference is said to be "pinned" to a specific version and can never bind to anything else.

At development-time, flexible versions are nice, because you're often getting the latest-and-greatest that a library
has to offer, without having to spend a great deal of time manually managing version numbers.  Flexible versions are
also nice for libraries, as the package manager can resolve multiple close, but different, semantic versions of a given
library to a single physical incarnation of it.   But when it comes to managing a production system, flexible versions
can cause problems, since upgrading to a new version may change a topology unexpectedly and/or at an inopportune time.

It is up to you, the package author, to decide whether to use flexible or pinned versions.  The recommended practice,
however, is to use flexible semantic version ranges for libraries, and pinned versions for executables.  This permits
flexibility on consumers of library packages where diamonds are more likely and where pinning might prevent the
transparent resolution of these diamonds.  The recommended practice for executables, however, is to pin them.  This
pinning is important to ensure that deployments are repeatable, and is encouraged by the command line tools.

No matter what, the `Lumi.json` (or `.yaml`) file should be checked in as-is, regardless of whether pinned or not.

### Pinning

To pin versions, you can simply specify concrete versions in your package manifest.

Alternatively, you can use command line tools to manage pinned versions:

* `lumi pack pin` will pin all packages to a specific version.
* `lumi pack pin <dep>` will pin the specific `<dep>` package to a specific version.
* `lumi pack upgrade` will upgrade all packages to new versions when available.
* `lumi pack upgrade <dep>` will upgrade the specific `<dep>` package to a new version when available.

To encourage the recommended workflow, the `lumi deploy` command will automatically pin references.  This ensures
executables are pinned to versions during, between, and after deployments.  The resulting manifest should be checked
into source control and versioned using the above commands.  The option `--no-pin` suppresses automatic pinning.

## Package Resolution

Now let us see how references are resolved to physical LumiPacks on the local filesystem.

LumiPacks may be found in multiple places, and, as in most dependency management schemes, some locations are preferred
over others.  This is to ease the task of local development while also providing rigorous dependency management.

Roughly speaking, these locations are are searched, in order:

1. The current workspace, for intra-workspace but inter-package dependencies.
2. The current workspace's `.lumi/packs/` directory (for locally installed packages).
3. The global workspace's `.lumi/packs/` directory (for machine-wide installed packages).
4. The Lumi runtime libraries: `$LUMIROOT/lib/packs/` (default `/usr/local/lumi/lib/packs`).

In each location, Lumi prefers a fully qualified match if it exists -- containing both the base of the reference plus
the name -- however, it also accept name-only matches.  This allows developers to organize their workspace without
worrying about where packages will get published.  Most of the Lumi tools, however, prefer fully qualified paths.

To be more precise, given a reference `r` and a workspace root `w`, we search these locations, in order:

1. `w/base(r)/name(r)`
2. `w/name(r)`
3. `w/.lumi/packs/base(r)/name(r)`
4. `w/.lumi/packs/name(r)`
5. `~/.lumi/packs/base(r)/name(r)`
6. `~/.lumi/packs/name(r)`
7. `$LUMIROOT/lib/base(r)/name(r)`
8. `$LUMIROOT/lib/name(r)`

To illustrate this process, let us imagine we are looking up the package `https://lumihub.com/aws/utils`.

In the illustration, let us imagine we're the author of the package, and so it is in our workspace.  We have things
organized so that it can be easily found, eliminating the need for us to frequently publish changes during development:

    <Workspace>
    |   .lumi/
    |   |   workspace.yaml
    |   aws/
    |   |   utils/
    |   |   |   Lumi.yaml
    |   |   |   ...other assets...

The `workspace.yaml` file may optionally specify a "namespace" property, as in:

    namespace: aws

In this case, the following simpler directory structure would also do the trick:

    <Workspace>
    |   .lumi/
    |   |   workspace.yaml
    |   utils/
    |   |   Lumi.yaml
    |   |   ...other assets...

It is possible to simplify this even further by specifying the namespace as `aws/utils`, leading to:

    <Workspace>
    |   .lumi/
    |   |   workspace.yaml
    |   Lumi.yaml
    |   ...other assets...

Notice that we didn't have to mention the `lumihub.com/` part in our workspace, although we can if we choose to.

In the second illustration, let us imagine we have used `lumi get` to download the dependency from a package manager:

    $ lumi get https://lumihub.com/aws/utils

In this case, our local workspace's package directory will have been populated with a copy of `aws/utils`:

    <Workspace>
    |   .lumi/
    |   |   packs/
    |   |   |   lumihub.com/
    |   |   |   |   aws/
    |   |   |   |   |   utils/
    |   |   |   |   |   |   Lumi.yaml
    |   |   |   |   |   |   ...other assets...

Notice that in this case, the full base part `lumihub.com/` is part of the path, since we downloaded it from that URL.

Now in the third and final illustration, let us imagine that we've installed a global copy of the package.  This might
have been thanks to use using `lumi get`'s `--global` (or `-g`) flag:

    $ lumi get --global https://lumihub.com/aws/utils

The directory structure will look identical to the second example, except that it is rooted in `~/`:

    ~
    |   .lumi/
    |   |   packs/
    |   |   |   lumihub.com/
    |   |   |   |   aws/
    |   |   |   |   |   utils/
    |   |   |   |   |   |   Lumi.yaml
    |   |   |   |   |   |   ...other assets...

## Fetching

TODO(joe): describe package fetching protocols.

TODO(joe): on-demand compilation (for easier Git fetching).

TODO(joe): describe how semantic versioning resolution works.

TODO(joe): describe how all of this interacts with Git repos (locally; e.g., git pull); Go-like.

