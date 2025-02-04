# Component Provider

The `pulumi.provider.experimental` package allows writing a Pulumi provider in Python with minimal boilerplate, using type annotations to define the schema. The provider can be published to Git or referenced locally.

## Building a Component Provider

Create a new directory that will hold the provider implementation.

First we need to let Pulumi's plugin system know which language runtime should be used to run the provider plugin. Create a a `PulumiPlugin.yaml` file with the contents:

```yaml
runtime: python
```

> [!NOTE]
> Python component providers only support the pip toolchain with a virtual environment named `venv`. Don't add any other runtime options to the `PulumiPlugin.yaml` file.

We also need to create a `requirements.txt` file to list the dependencies of the provider:

```txt
git+ssh://git@github.com/pulumi/pulumi.git@julienp/tutorial#subdirectory=sdk/python
```

TODO: update version to a pre-release with the experimental component provider merged


The resources provided by the provider are are defined as Python classes that subclass `pulumi.ComponentResource`. Create a new file `my-component.py` to hold the component implementation:

```python
from typing import Optional

import pulumi

class MyComponent(pulumi.ComponentResource):
    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__("my-provider:index:MyComponent", name, {}, opts)
        self.register_outputs({})
```

> [!NOTE]
> The type string `my-provider:index:MyComponent` follows the pattern `<package>:<module>:<type>`. The package is your provider's name, module must be `index`. The `type` must match the class name.

Next, create a `__main__.py` file to host the provider:

```python
from pulumi.provider.experimental import Metadata, component_provider_host

if __name__ == "__main__":
    component_provider_host(
        Metadata(name="my-provider", version="1.2.3", display_name="My Component Provider")
    )
```

> [!NOTE]
> The name passed to `Metadata` must exactly match the package name in the type string of your component.

The call to `component_provider_host` will start the provider and listen for RPC requests from the Pulumi engine. Any subclasses of `pulumi.ComponentResource` found in any Python source code files in the provider's directory will be exposed as resources by the provider.

At this point, we can run the provider, but it does not yet do anything useful. To get data in and out of the provider we need to define the inputs and outputs of the component. Inputs are infered from the `args` parameter of the component's constructor. Add a new type `MyComponentArgs` derived from `TypedDict` to represent the arguments, and add it as an argument to the constructor.

```diff
-from typing import Optional
+from typing import Optional, TypedDict

 import pulumi

+class MyComponentArgs(TypedDict):
+    who: Optional[pulumi.Output[str]]
+
 class MyComponent(pulumi.ComponentResource):
-    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
+    def __init__(self, name: str, args: MyComponentArgs, opts: Optional[pulumi.ResourceOptions] = None):
         super().__init__("my-provider:index:MyComponent", name, {}, opts)
         self.register_outputs({})
```

Outputs are defined as attributes on the component class:

```diff
 class MyComponent(pulumi.ComponentResource):
+    greeting: Optional[pulumi.Output[str]]
+
     def __init__(self, name: str, args: MyComponentArgs, opts: Optional[pulumi.ResourceOptions] = None):
         super().__init__("my-provider:index:MyComponent", name, {}, opts)
         self.register_outputs({})
```

To return data from the component, assign a value to the output attribute:

```diff
     def __init__(self, name: str, args: MyComponentArgs, opts: Optional[pulumi.ResourceOptions] = None):
         super().__init__("my-provider:index:MyComponent", name, {}, opts)
+        who = args.get("who") or "Pulumipus"
+        self.greeting = pulumi.Output.concat("Hello, ", who, "!")
         self.register_outputs({})
```

With everything in place, we can now run the provider, and request the schema:

```bash
# Create a venv and install the dependencies
python -m venv venv
source venv/*/activate
pip install -r requirements.txt

# Get the schema
pulumi package get-schema ./
```

```json
{
  "name": "my-provider",
  "displayName": "My Component Provider",
  "version": "1.2.3",
  "meta": {
    "moduleFormat": "(.*)"
  },
  "language": { ... },
  "config": {},
  "provider": {
    "type": "object"
  },
  "resources": {
    "my-provider:index:MyComponent": {
      "properties": {
        "greeting": {
          "type": "string"
        }
      },
      "type": "object",
      "required": [
        "greeting"
      ],
      "inputProperties": {
        "who": {
          "type": "string"
        }
      },
      "isComponent": true
    }
  }
}
```

To use the provider from a local project, create a new directory and add a `Pulumi.yaml` file with a reference to the provider under the `plugins.provider` key:

```bash
mkdir example && cd example
```

```yaml
name: test-provider-tutorial
runtime: yaml
plugins:
  providers:
    - name: my-provider
      path: ../
resources:
  greeter:
    type: my-provider:index:MyComponent
    properties:
      who: Bonnie
outputs:
  greeting: ${greeter.greeting}
```

```bash
pulumi up
pulumi stack output greeting
> Hello, Bonnie!
```

TODO: use the provider from a differnt language with a generated SDK.

## Current Limitations

The current implementation is not yet complete, and has the following limitations:

* The module must always be `index`
* Plain types are not supported
* Dictionary types are not supported
* List types are not supported
* Enum types are not supported
* Discriminated unions types are not supported
* References to other components are not supported
