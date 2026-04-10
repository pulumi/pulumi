# Providers and Languages

Languages and providers run as their own executables, so debugging isn't as
simple as attaching to the engine.

Instead you'll have to launch them in a debugger and provide the information
for connecting to them to the engine via environmental variables.

## Providers

To debug a provider, first start the provider executable (e.g. `pulumi-resource-aws`)
in your debugger of choice and have it listen on a known port.

Then set the `PULUMI_DEBUG_PROVIDERS` environment variable to tell the engine to
connect to that port instead of launching the provider itself. The format is
`<provider-name>:<port>`, and multiple providers can be separated by commas:

```sh
PULUMI_DEBUG_PROVIDERS="aws:12345" pulumi preview
```

For example, `PULUMI_DEBUG_PROVIDERS=aws:12345,gcp:678` will result in the
engine attaching to port 12345 for the `aws` provider and port 678 for the `gcp`
provider.

## Languages

Languages work very similarly to [providers](#providers).

Instead of the `PULUMI_DEBUG_PROVIDERS` environment variable, you use
`PULUMI_DEBUG_LANGUAGES` like so:

```sh
PULUMI_DEBUG_LANGUAGES="go:12345" pulumi preview
```

After opening your language executable (e.g. `pulumi-language-go`) in your debugger of choice.
