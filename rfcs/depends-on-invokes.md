# [RFC] DependsOn for Provider Functions

## Summary

Add a new `DependsOn` option for provider functions, which allows specifying additional dependencies that are not captured as part of the function’s inputs.

Related issues: https://github.com/pulumi/pulumi/issues/14243, https://github.com/pulumi/pulumi/issues/9593

## Background

[`Output`](https://www.pulumi.com/docs/iac/concepts/inputs-outputs/#outputs)s are the core mechanism for tracking dependencies in Pulumi. If the `Output` of one resource is passed to the input of another, Pulumi will recognise the dependency and appropriately order operations on those resources, ensuring that a dependency is not created after resources which depend on it. There are occasions when there is some user-known dependency between two resources, but where there is no natural Output property that can be used to link the two. For these situations, Pulumi provides [`DependsOn`](https://www.pulumi.com/docs/iac/concepts/options/dependson/), a resource option that programs can use to explicitly encode known dependencies and enforce ordering constraints on resource operations.

However, while resources typically make up the majority of a Pulumi program, they are not the only interface to Pulumi’s engine and provider set. [Provider functions](https://www.pulumi.com/docs/iac/concepts/resources/functions/), or “invokes”, allow providers to expose arbitrary functions to Pulumi programs. A common use case is looking up provider resources: [`getAmi`](https://www.pulumi.com/registry/packages/aws/api-docs/ec2/getami/) is an example from the AWS provider which gives programs a means to lookup an Amazon Machine Image (AMI), e.g. for use in constructing an EC2 instance later on. Presently, every invoke is offered in two “invocation forms”:
* A *direct form*, which accepts plain arguments and either blocks until a result is available, or acts asynchronously using a language-native concept of asynchronicity (e.g. Promise in JavaScript, Task in .Net, etc.)
* An *Output form*, which accepts any valid Pulumi Input (i.e. plain or Output values) and returns a Pulumi Output-wrapped result.

While direct-form invocations do not have any means to track dependencies, Output form invocations do, and will respect ordering according to the inputs they are passed. Unlike resources, however, Output form invokes do not currently offer a facility like `DependsOn`, whereby users can explicitly control the ordering of invocations where a dependency is not visible to Pulumi. This RFC proposes adding a similar option to invokes.

## Problem

Both direct form and Output form invokes take an optional argument to pass in the invoke options. These options allow configuring a parent or provider for the invoke.

As an example, in TypeScript we use [`InvokeOptions`](https://www.pulumi.com/docs/reference/pkg/nodejs/pulumi/pulumi/interfaces/InvokeOptions.html):

```typescript 
function getAmi(args: GetAmiArgs, opts?: InvokeOptions): Promise<GetAmiResult> { ... }

function getAmiOutput(args: GetAmiOutputArgs, opts?: InvokeOptions): Output<GetAmiResult> { ... }
```

For Go, the invokes take a variadic [`...InvokeOption`](https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v3/go/pulumi#InvokeOption) argument:

```go
func LookupAmi(ctx *Context, args *LookupAmiArgs, opts ...InvokeOption) (*LookupAmiResult, error) { ... }

func LookupAmiOutput(ctx *Context, args *LookupAmiOutputArgs, opts ...InvokeOption) LookupAmiResultOutput { ... }
```

Pulumi's type system represents the fact of potentially waiting for a dependency as an Output, however direct form invokes do not return an Output, and thus the `DependsOn` option does not make sense for these invokes. The option should be limited to Output form invokes, which do return an Output and fully take part in Pulumi’s dependency tracking system.

This means we cannot add the new `DependsOn` option to the existing `InvokeOptions`. We initially attempted this in https://github.com/pulumi/pulumi/pull/16560, and reverted it again in https://github.com/pulumi/pulumi/pull/16642.

Note that you can use [`apply`](https://www.pulumi.com/docs/iac/concepts/inputs-outputs/apply/) to work around the limitation. This is described in the [dependencies and ordering section](https://www.pulumi.com/docs/iac/concepts/resources/functions/#dependencies-and-ordering) of the [provider functions documentation](https://www.pulumi.com/docs/iac/concepts/resources/functions/).

## API Proposal
This RFC proposes to add a new `InvokeOutputOptions` type for languages other than Go to disambiguate the options for direct and Output form invokes. For Go we propose a slightly different approach.

This new argument type has to be introduced in a backwards compatibility preserving way, with different approaches for our supported languages.

### TypeScript
The [￼`InvokeOptions`￼](https://www.pulumi.com/docs/reference/pkg/nodejs/pulumi/pulumi/interfaces/InvokeOptions.html) type in the TypeScript SDK is an interface, which the new `InvokeOutputOptions` interface can extend:
```typescript
export interface InvokeOutputOptions extends InvokeOptions {
    dependsOn?: Input<Input<Resource>[]> | Input<Resource>;
}
```
Optionality in TypeScript means `T | undefined`, and looking up an property on object that has not been set returns `undefined`, thus any value satisfying the `InvokeOptions` interface also satisfies the `InvokeOptionsOutput` interface.

Output form invokes can take an argument of this type while preserving backwards compatibility.
```typescript
function getAmiOutput(args: GetAmiOutputArgs, opts?: InvokeOutputOptions): Output<GetAmiResult> { ... }
```

### Python

The [￼`InvokeOptions`￼](https://www.pulumi.com/docs/reference/pkg/python/pulumi/#pulumi.InvokeOptions) type in the Python SDK is a class. The new `InvokeOutputOptions` extends this class and adds the optional `depends_on` property:
```python
class InvokeOutputOptions(InvokeOptions):
  depends_on: Optional[Input[Union[Sequence[Input[Resource]], Resource]]]
```

In Python, optionality means `T | None`, and attempting to access a property that has not been defined on an object results in an `AttributeError`. This means that an instance of type `InvokeOptions` does not conform to `InvokeOutputOptions`. To allow passing both types, we can make the argument type the union of the types:

```python
def get_ami_output(..., opts: Optional[Union[InvokeOptions, InvokeOutputOptions]] = None) -> Output[GetAmiResult]
  ...
```

### Go
In Go, the type for invoke options is a variadic [`...InvokeOption`](https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v3/go/pulumi#InvokeOption), for example:
```go
func LookupAmiOutput(ctx *Context, args *LookupAmiOutputArgs, opts ...InvokeOption) LookupAmiResultOutput
```

We propose to update the existing [￼`DependsOn`￼](https://pkg.go.dev/github.com/pulumi/pulumi/sdk/v3/go/pulumi#DependsOn) for resources to also apply to invokes:
```go
func DependsOn(o []Resource) ResourceOption
```
becomes
```go
func DependsOn(o []Resource) ResourceOrInvokeOption
```

`ResourceOrInvokeOption` is composed of both `ResourceOption` and `InvokeOption`  and a value of this type can always be assigned to its narrower constituent types.
```go
type ResourceOrInvokeOption interface {
	ResourceOption
	InvokeOption
}
```

Since there are no union types in Go, we cannot change type of existing Output form invokes without breaking backwards compatibility. Instead, we propose to allow passing this option to both forms of invokes, and to return an error if the option is passed to a direct invoke.

```go
ami, err := ec2.LookupAmi(ctx,
	&ec2.LookupAmiArgs{...},
	pulumi.DependsOn(someResource),
)
// err: Can not pass DependsOn to direct form invoke, use Output form instead
```

Ideally the error message can point to the matching Output form invoke.

#### Alternative

Alternatively, we could add a 3rd invoke variant to Go SDKs, besides the direct and Output form variants, which takes an additional dependsOn argument:

```go
func LookupAmi(ctx *Context, args *LookupAmiArgs, opts ...InvokeOption) (*LookupAmiResult, error) { ... }
func LookupAmiOutput(ctx *Context, args *LookupAmiOutputArgs, opts ...InvokeOption) LookupAmiResultOutput { ... }
// New variant
func LookupAmiOutputWithDependsOn(ctx *Context, args *LookupAmiOutputArgs, dependsOn: []Resource, opts ...InvokeOption) LookupAmiResultOutput { ... }
```

Besides increasing the code size of SDKs, we feel that this would make Go SDKs more difficult for users to navigate and pick the function they need. The general use case does not require specifying additional dependencies, and exposing the variant at the level of functions feels like it would lead to more confusion.

### Java
Options are represented by the [￼￼`InvokeOptions`￼￼](https://github.com/pulumi/pulumi-java/blob/main/sdk/java/pulumi/src/main/java/com/pulumi/deployment/InvokeOptions.java#L20) class in the Java SDK. We propose adding a new class `InvokeOutputOptions` with a `dependsOn` field, and to add a new overload for the Output form invokes:

```java
public static Output<GetAmiResult> getAmi(GetAmiArgs args)
public static Output<GetAmiResult> getAmi(GetAmiArgs args, InvokeOptions options)
// new overload
public static Output<GetAmiResult> getAmi(GetAmiArgs args, InvokeOutputOptions options)
```

### Dotnet
Options are represented by the [`InvokeOptions`](https://www.pulumi.com/docs/reference/pkg/dotnet/Pulumi/Pulumi.InvokeOptions.html) class in the Dotnet SDK. Similar to the proposed solution for Java, we propose adding  new class `InvokeOutputOptions` with a `dependsOn` field, and to add a new overload for the Output form invokes:
```csharp
public static class GetAmi {
  public static Output<GetAmiResult> Invoke(GetAmiInvokeArgs? args = null, InvokeOptions? options = null)
  // new overload
  public static Output<GetAmiResult> Invoke(GetAmiInvokeArgs? args = null, InvokeOutputOptions? options = null)
}
```

### YAML
In the YAML SDK, the [`fn::invoke`](https://www.pulumi.com/docs/iac/languages-sdks/yaml/yaml-language-reference/#fninvoke) built-in function takes an `options` property of type [`InvokeOptions`](https://www.pulumi.com/docs/iac/languages-sdks/yaml/yaml-language-reference/#invoke-options). This type can be extended to include a new`dependsOn` field.

## Changelog
### 2024-11-06
Initial version
