# Component Provider

The `@pulumi/pulumi/provider/experimental` package allows writing a Pulumi provider in TypeScript with minimal boilerplate, letting pulumi infer the schema. The provider can be published to Git or referenced locally.

## Building a Component Provider

Create a new directory that will hold the provider implementation.

First we need to let Pulumi's plugin system know which language runtime should be used to run the provider plugin. Create a `PulumiPlugin.yaml` file with the contents:

```yaml
runtime: nodejs
```

We also need to create a `package.json` file to define the project and list the dependencies of the provider.

```txt
{
    "name": "my-provider",
    "main": "index.ts",
    "devDependencies": {
        "@types/node": "^18",
        "typescript": "^5.0.0"
    },
    "dependencies": {
        "@pulumi/pulumi": "^3.153.0",
		"@pulumi/random": "^4.0.0"
    }
}
```

This uses version 3.153.0 of the NodeJS Pulumi SDK that includes the experimental component provider feature. We'll use the `@pulumi/random` package to generate a random greeting.

The resources provided by the provider are are defined as TypeScript classes that subclass `pulumi.ComponentResource`. Create a new file `myComponent.ts` to hold the component implementation:

```typescript
import * as pulumi from "@pulumi/pulumi";

export interface MyComponentArgs {
}

export class MyComponent extends pulumi.ComponentResource {
    constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
	super("my-provider:index:MyComponent", name, args, opts);
    }
}
```

> [!NOTE]
> The type string `my-provider:index:MyComponent` follows the pattern `<package>:<module>:<type>`. The package is your provider's name, module must be `index`. The `type` must match the class name.

Next, create a `index.ts` file to host the provider:

```typescript
import * as experimental from "@pulumi/pulumi/provider/experimental";

experimental.componentProviderHost();
```

The call to `componentProviderHost` will start the provider and listen for RPC requests from the Pulumi engine. Any subclasses of `pulumi.ComponentResource` found in any TypeScript source code files specfied in the `tsconfig.json` file will be exposed by the provider.

Our `tsconfig.json` file looks like this:

```json
{
    "compilerOptions": {
        "strict": true,
        "outDir": "bin",
        "target": "es2020",
        "module": "commonjs",
        "moduleResolution": "node",
        "sourceMap": true,
        "experimentalDecorators": true,
        "pretty": true,
        "noFallthroughCasesInSwitch": true,
        "noImplicitReturns": true,
        "forceConsistentCasingInFileNames": true
    },
    "files": [
        "index.ts",
        "myComponent.ts"
    ]
}
```

At this point, we can run the provider, but it does not yet do anything useful. To get data in and out of the provider we need to define the inputs and outputs of the component. Inputs are infered from the `args` parameter of the component's constructor. Add the input argument in the `MyComponentArgs` interface:

```diff
 import * as pulumi from "@pulumi/pulumi";

 export interface MyComponentArgs {
+    /**
+     * Who to greet
+     */
+    name?: string;
 }

 export class MyComponent extends pulumi.ComponentResource {
```

Outputs are defined as attributes on the component class:

```diff
 }

 export class MyComponent extends pulumi.ComponentResource {
+    /**
+     * The greeting message
+     */
+    greeting: pulumi.Output<string>;
+
     constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
        super("provider:index:MyComponent", name, args, opts);
     }
```

The docstrings for the inputs and outputs that we added will be used as descriptions in the schema of the provider.

To return data from the component, assign a value to the output attribute:

```diff
 import * as pulumi from "@pulumi/pulumi";
+import * as random from "@pulumi/random";

 export interface MyComponentArgs {
     /**
@@ -14,6 +15,13 @@ export class MyComponent extends pulumi.ComponentResource {
     greeting: pulumi.Output<string>;

     constructor(name: string, args: MyComponentArgs, opts?: pulumi.ComponentResourceOptions) {
+       const greetingNname = args.name || "Pulumipus";
        super("provider:index:MyComponent", name, args, opts);
+       const greetingWord = random.RandomShuffle(`${name}-greeting-word`, {
+           inputs: ["Hello", "Hola", "Bonjour", "Ciao", "Aloha"],
+           result_count: 1,
+
+       }, { parent: this });
+       this.greeting = pulumi.interpolate`${greetingWord.results}, ${greetingName}`;
     }
 }
```

Note how we set the name of the `RandomShuffle` resource to include the name of the component, to avoid name collisions if more than one instance of the component is created. We also set the parent of the `RandomShuffle` resource to the component, to properly establish the dependency between the two resources.

As a last step, we need to publish the provider so that we can use it. Create a new Git repository and push the provider code to it. Add a tag to the repository to mark the version of the provider:

```bash
git init
git add .
git commit -m "Initial commit"
git tag v0.1.0
git remote add origin $MY_GIT_REPO
git push --tags origin main
```

## Using the Component Provider

Now that our component provider has been published, we can use it in a separate project. We'll use Python in this example, but the provider can be used from any language supported by Pulumi. Create a new directory for the project and then initialize a new Pulumi project:

```bash
cd $MY_PULUMI_RPOJECT
pulumi new python
```

To start using the provider we need to add it as a dependency to the project:

```bash
pulumi package add $MY_GIT_REPO@v1.0.0
```

> [!NOTE]
> You still need to run this command even when [running the provider from a local directory](#running-the-provider-from-a-local-directory). In this case, just call `add` with a path to the directory of your component, rather than a Git repository and version.

This will download the provider from the git repo, generate the Python SDK for the provider in `sdks/greeting` and add a references to it in `requirements.txt`.

> [!NOTE]
> You can commit the generated SDK to your project's repository to avoid having to regenerate it when checking out the project on a different machine, for example in CI.

With the SDK in place, we can start using our component, edit `__main__.py`:

```typescript
import pulumi
import pulumi_greeting as greeting

greeting = greeting.MyComponent("greeter", greeting.MyComponentArgs(name="Bonnie"))
pulumi.export("greeting", greeting.greeting)
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
npm install
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
```

> [!NOTE]
> To import the provider into a YAML project, you will also need to add an explicit resource to your `Pulumi.yaml`, as well as the explicit `outputs`:
>
> ```yaml
> resources:
>   greeter:
>     type: my-provider:index:MyComponent
>     properties:
>       name: Bonnie
> outputs:
>   greeting: ${greeter.greeting}
> ```

## Current Limitations

The current implementation is not yet complete, and has the following limitations:

* Enum types are not supported
* Discriminated unions types are not supported
* References to other components are not supported
