The Kubernetes provider package offers support for all Kubernetes resources and their properties.
Resources are exposed as types from modules based on Kubernetes API groups such as 'apps', 'core',
'rbac', and 'storage', among many others. Additionally, support for deploying Helm charts ('helm')
and YAML files ('yaml') is available in this package. Using this package allows you to
programmatically declare instances of any Kubernetes resources and any supported resource version
using infrastructure as code, which Pulumi then uses to drive the Kubernetes API.

If this is your first time using this package, these two resources may be helpful:

* [Kubernetes Getting Started Guide](https://www.pulumi.com/docs/quickstart/kubernetes/): Get up and running quickly.
* [Kubernetes Pulumi Setup Documentation](https://www.pulumi.com/docs/quickstart/kubernetes/configure/): How to configure Pulumi
    for use with your Kubernetes cluster.

Use the navigation below to see detailed documentation for each of the supported Kubernetes resources.
