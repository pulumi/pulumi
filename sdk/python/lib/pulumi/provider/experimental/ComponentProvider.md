# Component Provider

The `pulumi.provider.experimental` package allows writing a Pulumi provider in Python with minimal boilerplate, using type annotations to define the schema. The provider can be published to Git or referenced locally.

## Building a Component Provider

Create a new directory that will hold the provider implementation.

First we need to let Pulumi's plugin system know which language runtime should be used to run the provider plugin. Create a `PulumiPlugin.yaml` file with the contents:

```yaml
runtime: python
```

> [!NOTE]
> Python component providers currently only support the pip toolchain with a virtual environment named `venv`. Don't add any other runtime options to the `PulumiPlugin.yaml` file.

We also need to create a `requirements.txt` file to list the dependencies of the provider:

```txt
pulumi>=3.150.0
pulumi_random=~4.0
```

This uses a prerelease version of the Python Pulumi SDK that includes the experimental component provider feature. We'll use the `pulumi_random` package to generate a random greeting.

The resources provided by the provider are are defined as Python classes that subclass `pulumi.ComponentResource`. Create a new file `my-component.py` to hold the component implementation:

```python
from typing import Optional

import pulumi

class MyComponent(pulumi.ComponentResource):
    """A component that greets someone"""
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
    component_provider_host(Metadata(name="my-provider", version="1.0.0"))
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
+    """Who to greet"""
+
 class MyComponent(pulumi.ComponentResourc0e):
     """A component that greets someone"""
-    def __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):
+    def __init__(self, name: str, args: MyComponentArgs, opts: Optional[pulumi.ResourceOptions] = None):
         super().__init__("my-provider:index:MyComponent", name, {}, opts)
         self.register_outputs({})
```

Outputs are defined as attributes on the component class:

```diff
 class MyComponent(pulumi.ComponentResource):
+    greeting: pulumi.Output[str]
+    """The greeting message"""
+
     def __init__(self, name: str, args: MyComponentArgs, opts: Optional[pulumi.ResourceOptions] = None):
         super().__init__("my-provider:index:MyComponent", name, {}, opts)
         self.register_outputs({})
```

The docstrings for the inputs and outputs that we added will be used as descriptions in the schema of the provider.

To return data from the component, assign a value to the output attribute:

```diff
+import pulumi_random as random
 from typing import Optional, TypedDict

...

     def __init__(self, name: str, args: MyComponentArgs, opts: Optional[pulumi.ResourceOptions] = None):
         super().__init__("my-provider:index:MyComponent", name, {}, opts)
+        who = args.get("who") or "Pulumipus"
+        greeting_word = random.RandomShuffle(
+           f"{name}-greeting",
+           inputs=["Hello", "Bonjour", "Ciao", "Hola"],
+           result_count=1,
+           opts=pulumi.ResourceOptions(parent=self),
+        )
+         self.greeting = pulumi.Output.concat(greeting_word.results[0], ", ", who, "!")
-         self.register_outputs({})
+         self.register_outputs({
+             "greeting": self.greeting,
+         })
```

Note how we set the name of the `RandomShuffle` resource to include the name of the component, to avoid name collisions if more than one instance of the component is created. We also set the parent of the `RandomShuffle` resource to the component, to properly establish the dependency between the two resources.

As a last step, we need to publish the provider so that we can use it. Create a new Git repository and push the provider code to it. Add a tag to the repository to mark the version of the provider:

```bash
git init
git add .
git commit -m "Initial commit"
git tag v1.0.0
git remote add origin $MY_GIT_REPO
git push --tags origin main
```

## Using the Component Provider

Now that our component provider has been published, we can use it in a separate project. We'll use TypeScript in this example, but the provider can be used from any language supported by Pulumi. Create a new directory for the project and then initialize a new Pulumi project:

```bash
cd $MY_PULUMI_RPOJECT
pulumi new typescript
```

To start using the provider we need to add it as a dependency to the project:

```bash
pulumi package add $MY_GIT_REPO@v1.0.0
```

This will download the provider from the git repo, generate the TypeScript SDK for the provider in `sdks/my-provider` and add a references to it in `package.json`.

> [!NOTE]
> You can commit the generated SDK to your project's repository to avoid having to regenerate it when checking out the project on a different machine, for example in CI.

With the SDK in place, we can start using our component, edit `index.ts`:

```typescript
import * as myProvider from "@pulumi/my-provider";

let greeter = new myProvider.MyComponent("greeter", { who: "Bonnie" });

export let greeting = greeter.greeting;
```

Run the program:

```bash
pulumi up
pulumi stack output greeting
> Ciao, Bonnie!
```

## Debugging

### Inspecting the schema

While developing it can be useful to inspect the schema of the provider to ensure it matches the expected shape. To do this, run the following commands:

```bash
# Ensure the dependencies for the provider are installed inside the provider repository
cd $MY_PROVIDER_REPO
python -m venv venv
source venv/*/activate
pip install -r requirements.txt
# Get the schema
pulumi package get-schema ./
```

```json
{
  "name": "my-provider",
  "version": "1.0.0",
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

### Running the provider from a local directory

During development, you may want to test the provider from a local directory without having to publish it to a Git repository. You can let Pulumi know where to find the provider by adding a reference to the provider directory in the `Pulumi.yaml`:

```yaml
name: my-provider-example
runtime: yaml
plugins:
  providers:
    - name: my-provider
      path: ../provider # Path to the provider directory
resources:
  greeter:
    type: my-provider:index:MyComponent
    properties:
      who: Bonnie
outputs:
  greeting: ${greeter.greeting}
```

## Current Limitations

The current implementation is not yet complete, and has the following limitations:

* The module must always be `index`
* Enum types are not supported
* Discriminated unions types are not supported
* References to other components are not supported
