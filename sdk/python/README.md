# Pulumi Python SDK

The Pulumi Python SDK (pulumi) is the core package used when writing Pulumi programs in Python. It contains everything that you’ll need in order to interact with Pulumi resource providers and express infrastructure using Python code. Pulumi resource providers all depend on this library and express their resources in terms of the types defined in this module.

The Pulumi Python SDK requires [Python version 3.7 or greater](https://www.python.org/downloads/) through official python installer

note:
pip is required to install dependencies. If you installed Python from source, with an installer from [python.org](https://python.org/), or via [Homebrew](https://brew.sh/) you should already have pip. If Python is installed using your OS package manager, you may have to install pip separately, see [Installing pip/setuptools/wheel with Linux Package Managers](https://packaging.python.org/guides/installing-using-linux-tools/). For example, on Debian/Ubuntu you must run sudo apt install python3-venv python3-pip.

## Getting Started

The fastest way to get up and running is to choose from one of the following Getting Started guides:
-[aws](https://www.pulumi.com/docs/get-started/aws/?language=python)
-[microsoft azure](https://www.pulumi.com/docs/get-started/azure/?language=python)
-[google cloud](https://www.pulumi.com/docs/get-started/gcp/?language=python)
-[kubernetes](https://www.pulumi.com/docs/get-started/kubernetes/?language=python)

## Pulumi Programming Model

The Pulumi programming model defines the core concepts you will use when creating infrastructure as code programs using Pulumi. Architecture & Concepts describes these concepts with examples available in Python. These concepts are made available to you in the Pulumi SDK.

The Pulumi SDK is available to Python developers as a Pip package distributed on [PyPI](https://www.pulumi.com/docs/intro/languages/python/#pypi-packages) . To learn more, [refer to the Pulumi SDK Reference Guide](https://www.pulumi.com/docs/reference/pkg/python/pulumi/).

The Pulumi programming model includes a core concept of Input and Output values, which are used to track how outputs of one resource flow in as inputs to another resource. This concept is important to understand when getting started with Python and Pulumi, and the [Inputs and Outputs] (https://www.pulumi.com/docs/intro/concepts/inputs-outputs/)documentation is recommended to get a feel for how to work with this core part of Pulumi in common cases.


## [The Pulumi Python Resource Model](https://www.pulumi.com/docs/reference/pkg/python/pulumi/#the-pulumi-python-resource-model-1)

Like most languages usable with Pulumi, Pulumi represents cloud resources as classes and Python programs can instantiate those classes. All classes that can be instantiated to produce actual resources derive from the pulumi.Resource class.