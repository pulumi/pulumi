(pcl)=
# Pulumi Configuration Language (PCL)

*Pulumi Configuration Language* (PCL) is an internal representation of Pulumi
programs which supports all core concepts of the Pulumi programming model in a
minimal form. Although not exposed directly to users today, this intermediate
representation is used to support a variety of program conversion tasks, from
and to various supported Pulumi languages.

Pulumi supports a number of operations pertaining to PCL programs:

* [*Lexing and parsing*](pcl-lexing-parsing), in which PCL source is parsed into
  an *abstract syntax tree* (AST).

* [*Type checking*](pcl-type-checking) and [*binding*](pcl-binding), in which a
  PCL AST is verified to be well-formed (describing resources, outputs, and so
  on), associated with ("bound to") a set of Pulumi [*schema*](schema), and type
  checked and resolved.

* [*Code generation*](programgen) ("programgen"), in which a PCL program is
  converted into a program written in a supported target language. Much of this
  underpins the core of tools such as `pulumi convert`.

(pcl-program)=
## PCL programs

PCL programs are comprised of PCL source files which typically have the suffix
`.pp`. This repository contains a large number of examples -- in particular, a
large amount of PCL is used to drive [language conformance
tests](language-conformance-tests). PCL programs support the following concepts:

(pcl-resource)=
### Resources

*Resources* are defined as [blocks](pcl-lexing-parsing) of type `resource`. They
correspond to stack resource registrations (see [resource
registration](resource-registration) and
[](pulumirpc.ResourceMonitor.RegisterResource) for more). A resource block must
have exactly two labels -- the first is the resource's name, to be used both as
a reference to it elsewhere in the program and as its name in the resulting
Pulumi stack (see the sections in this documentation on [URNs](urns) and
[resource registration](resource-registration)). The resource's body contains a
set of attributes, which correspond to the resource's input properties. An
optional `options` block may be used to specify resource options (e.g.
`provider`, `protect`, `ignoreChanges`, etc.).

```hcl
resource "r" "pkg:index:Resource" {
    name = "r"

    options {
        protect = true
    }
}
```

(pcl-component)=
### Components

A *component* in the context of a PCL program is equivalent to the notion of a
"local" [component resource](component-resources) in a Pulumi program.
Components are defined as blocks of type `component`. A component block must
have two labels. The first is the component's name, to be used both as a
reference to it elsewhere in the program and as its name in the resulting Pulumi
stack. The second is a path to a directory containing a PCL program (set of
`.pp` files) that defines the component's implementation. The component's body
contains a set of attributes, which correspond to the component's input
properties. An optional `options` block may be used to specify component options
(e.g. `provider`, `protect`, etc.).

```hcl
component "c" "./c" {
    name = "c"
    value = 42
}
```

Program generation for components results in e.g. the generation of local
component types, such as those extending the `ComponentResource` class in SDKs
exposing that type.

(pcl-output)=
### Outputs

*Outputs* are defined as [blocks](pcl-lexing-parsing) of type `output`. They
correspond exactly to exported stack outputs, such as `pulumi.export` in Python
or `ctx.Export` in Go, which correspond to
[](pulumirpc.ResourceMonitor.RegisterResourceOutputs) on the stack resource
under the hood. Output blocks must have exactly one label, which is the output's
name. The output's body must contain a single attribute, `value`, which is the
output's value.

```hcl
resource "r" "pkg:index:Resource" {
    name = "r"
}

output "o" {
    value = r.name
}
```

(pcl-config)=
### Configuration

*Configuration variables* define the names and schema of values that are
expected to be supplied to a program through Pulumi configuration (e.g. from a
`Pulumi.<stack>.yaml`). They are defined as [blocks](pcl-lexing-parsing) of type
`config`, with each block taking labels for the variable's name and type. The
block's body may be used to specify other aspects of the variable, such as a
description and default value. `config` blocks are typically used to generate
program code for referring to Pulumi configuration, e.g. by creating and using a
`pulumi.Config()` object in NodeJS.

```hcl
config "key" "string" {
  description = "The key to use when encrypting data"
  default     = ""
}
```

(pcl-invoke)=
### `invoke`

The `invoke` intrinsic is used to express provider function calls and
corresponds to the [](pulumirpc.ResourceMonitor)'s
[](pulumirpc.ResourceMonitor.Invoke) method.

```hcl
result1 = invoke("pkg:index:Function", { x = 3, y = 4 })

result2 = invoke("pkg:index:Function", { z = 5 }, { parent = p })
```

(pcl-call)=
### `call`

The `call` intrinsic is used to express a method invocation on a resource and
corresponds to the [](pulumirpc.ResourceMonitor)'s
[](pulumirpc.ResourceMonitor.Call) method.

```hcl
resource "c" "pkg:index:Component" {
    ...
}

result = call(c, "method", { x = 3, y = 4 })
```

(pcl-builtin-function)=
### Built-in functions

PCL offers a number of *built-in* functions for use in expressions. These are
captured by the `pulumiBuiltins` function in
[](gh-file:pulumi#pkg/codegen/pcl/functions.go).

:::{toctree}
:maxdepth: 1
:titlesonly:

Syntax and type checking </pkg/codegen/hcl2/README>
Binding </pkg/codegen/pcl/binding>
:::
