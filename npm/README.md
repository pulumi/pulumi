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
[get.pulumi.com](https://get.pulumi.com) and caches it in npm's cache directory
(`~/.npm/_pulumi/` by default). The download is checksum-verified to detect
corruption in transit. Subsequent invocations use the cached binary directly.

If a `pulumi` binary is already on your `PATH` (e.g. installed via Homebrew or
the [install script](https://www.pulumi.com/docs/install/)), that installation
is used instead of downloading a new one.

## Version pinning

The npm package version matches the Pulumi CLI version exactly. Pin to a
specific release by specifying the version:

```sh
npx pulumi@3.323.0 up
```

## Cache location

Binaries are cached at `_pulumi/<version>/` inside npm's configured cache
directory. To change the location, set `npm_config_cache`:

```sh
npm_config_cache=/path/to/cache npx pulumi up
```
