# pulumi

Lazily install the [Pulumi CLI](https://www.pulumi.com) via npx.

```sh
npx pulumi up
```

> **Note:** This is NOT the Pulumi SDK. If you're writing a Pulumi program you
> want [`@pulumi/pulumi`](https://www.npmjs.com/package/@pulumi/pulumi)
> instead. See the [language SDK
> docs](https://www.pulumi.com/docs/iac/languages-sdks/) for all supported
> runtimes.

## How it works

On first use, the package downloads the Pulumi CLI binary for your platform from
[get.pulumi.com](https://get.pulumi.com) and caches it under `~/.pulumi/versions/`.
Subsequent invocations use the cached binary directly.

## Version pinning

The npm package version matches the Pulumi CLI version exactly. Pin to a
specific release by specifying the version:

```sh
npx pulumi@3.323.0 up
```

## Cache location

Binaries are cached at `~/.pulumi/versions/<version>/bin/`, shared with the
[Automation API](https://www.pulumi.com/docs/iac/packages/pulumi-automation/) so
both never download the same version twice. Set `PULUMI_HOME` to use a different
base directory:

```sh
PULUMI_HOME=/path/to/pulumi npx pulumi up
```

To clear the cache:

```sh
rm -rf ~/.pulumi/versions
```
