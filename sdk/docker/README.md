# pulumi-language-docker

This is a proof of concept that shows how we can distribute plugins as docker images.

We add a `dockerSource` plugin source that knows how to interpret a URL starting with `docker.io`. This enables commands like `pulumi plugin install ...` or `pulumi package add ...` to seamlessly work with docker based plugins.

When installing such a plugin, instead of downloading source code to `~/.pulumi/plugins`, create a `PulumiPlugin.yaml`:

```yaml
runtime:
  name: docker
  options:
    image: docker.io/jpoissonnier/pulumi-docker-plugin-example:0.0.28
```

The language plugin's RunPlugin method reads this file and runs the specified image.

A pulumi command invocation is executed by a set of servers that communicate over GRPC. These all expect to be running on the same machine and to be able to
communicate on `127.0.0.1:${port}`. When a plugin runs inside docker, this is no longer true. Inside docker, the plugin needs to reach out back to the host via `host.docker.internal`. Communication from host to docker can still happen over `127.0.0.1` since we expose ports from the container.

One particularly tricky situation is `Construct`. A `Construct` call includes a "monitor endpoint", the address of the pulumi engine to which calls such as `RegisterResource` are sent.
