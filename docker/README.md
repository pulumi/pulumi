# Pulumi Docker images

The [Pulumi docker image](https://hub.docker.com/r/pulumi/pulumi) is quite large because it has to bundle all the SDKs we support:

  - Go
  - Python
  - NodeJS
  - DotNet

We do offer SDK specific containers. They contain the `pulumi` binary, the `pulumi` language runtime
for that SDK and any additional language quirks.

## Images

We build a matrix of images for differing Pulumi language SDKs and operating systems. The OS base images we use are:

  - registry.access.redhat.com/ubi8/ubi-minimal (ubi)
  - debian:buster-slim (debian)
  - alpine:3.12.0 (alpine)

### Base Image

The base image just contains the pulumi binaries and language runtimes, but _not_ the SDK runtimes. If you use the base
image, you'll have to install Go/Python/Dotnet/NodeJS yourself. The image format is:

```
pulumi/pulumi-base:<PULUMI_VERSION>-<OS>
```

### SDK Images

Images with the SDK runtimes are generated in the following format:

```
pulumi/pulumi-<PULUM_SDK>:<PULUMI_VERSION>-<OS>
```

### Image Size

Each of the images are much smaller than the combined Pulumi container. They are in the region of approx 150MB (compressed size)
depending on the operating system it has been built on

## Usage

None of these images have `CMD` or entrypoint set, so you'll need to specify the commands you want to run, for example:

```
docker run -e PULUMI_ACCESS_TOKEN=<TOKEN> -v "$(pwd)":/pulumi/projects $IMG /bin/bash -c "npm ci && pulumi preview -s <stackname>"
```

## Considerations

These images _do not_ include additional tools you might want to use when running a pulumi provider. For example, if 
you're using the [pulumi-kubernetes](https://github.com/pulumi/pulumi-kubernetes) with [Helm](https://helm.sh/), you'll 
need to use these images as a base image, or install the `helm` command as part of your CI setup.
