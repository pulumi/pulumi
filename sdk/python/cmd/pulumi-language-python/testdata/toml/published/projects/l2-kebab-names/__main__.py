import pulumi
import pulumi_kebab_names as kebab_names

# The package name and module name are kebab-case. Resource and object type names cannot be
# kebab-case yet (the metaschema forbids hyphens in the member segment of a token), and kebab-case
# property names are not yet handled by all code generators.
first = kebab_names.kebab_module.SomeResource("first",
    the_input=True,
    nested={
        "nested_value": "nested",
    })
second = kebab_names.kebab_module.AnotherResource("second", the_input=first.the_output.nested_output)
pulumi.export("theOutput", first.the_output)
