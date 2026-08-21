import pulumi
import pulumi_kebab_names as kebab_names

# The package name, module name and property names are kebab-case. Resource and object type names
# cannot be kebab-case yet: the metaschema forbids hyphens in the member segment of a token.
first = kebab_names.kebab_module.SomeResource("first",
    the_input=True,
    nested={
        "nested_value": "nested",
    })
second = kebab_names.kebab_module.AnotherResource("second", the_input=first.the_output.nested_output)
