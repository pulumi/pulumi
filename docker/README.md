# Pulumi Docker images

This image is an alternative to the [Pulumi docker image](https://hub.docker.com/r/pulumi/pulumi)
The `pulumi/pulumi` image is quite large because it has to bundle all the SDKs that Pulumi supports:

  - Go
  - Python
  - NodeJS
  - DotNet

This container is a slimmer container for the specific SDK. It contains the `pulumi` binary, the `pulumi` language runtime
for that SDK and any additional necessary language components..

## Images

We build a matrix of images for differing Pulumi language SDKs and operating systems. The OS base images we use are:

  - registry.access.redhat.com/ubi8/ubi-minimal (ubi)
  - debian:buster-slim (debian)

### Base Image

The base image just contains the pulumi binaries and language runtimes, but _not_ the SDK runtimes. If you use the base
image, you'll have to install Go/Python/Dotnet/NodeJS yourself. The image format is:

```
pulumi/pulumi-base:<PULUMI_VERSION>-<OS>
```

The default image without the OS is based on Debian Buster, and can be used like so:

```
pulumi/pulumi-base:<PULUMI_VERSION>
```

### SDK Images

Images with the SDK runtimes are generated in the following format:

```
pulumi/pulumi-<PULUMI_SDK>:<PULUMI_VERSION>-<OS>
```

The default image without the OS suffix is based on Debian Buster, and can be used like so:

```
pulumi/pulumi-<PULUMI_SDK>:<PULUMI_VERSION>
pulumi/pulumi-<PULUMI_SDK>:latest
```

### Image Size

Each of the images are much smaller than the combined Pulumi container. They are in the region of approx 150MB (compressed size)
depending on the operating system it has been built on

### Operating Systems

We currently build images based on both [Debian Buster](https://wiki.debian.org/DebianBuster) and with the [RedHat Universal Base Image](https://www.redhat.com/en/blog/introducing-red-hat-universal-base-image)/

### UBI Images

The UBI images use `microdnf` as a package manager, not yum. See [this](https://github.com/rpm-software-management/microdnf) page for more information.

## Usage

In order to try and keep the images flexible and try to meet as many use cases as possible, none of these images have `CMD` or entrypoint set, so you'll need to specify the commands you want to run, for example:

```
docker run -e PULUMI_ACCESS_TOKEN=<TOKEN> -v "$(pwd)":/pulumi/projects $IMG /bin/bash -c "npm ci && pulumi preview -s <stackname>"
```

## Considerations

These images _do not_ include additional tools you might want to use when running a pulumi provider. For example, if 
you're using the [pulumi-kubernetes](https://github.com/pulumi/pulumi-kubernetes) with [Helm](https://helm.sh/), you'll 
need to use these images as a base image, or install the `helm` command as part of your CI setup.
