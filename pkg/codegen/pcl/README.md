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

:::{toctree}
:maxdepth: 1
:titlesonly:

Syntax and type checking </pkg/codegen/hcl2/README>
Binding </pkg/codegen/pcl/binding>
:::
