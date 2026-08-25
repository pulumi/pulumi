import pulumi
import pulumi_kebab_names as kebab_names

# The package name, module name, resource names, object type names and property names are all
# kebab-case.
first = kebab_names.kebab_module.SomeResource("first",
    the_input=True,
    nested={
        "nested_value": "nested",
    })
second = kebab_names.kebab_module.AnotherResource("second", the_input=first.the_output.nested_output)
pulumi.export("theOutput", first.the_output)
pulumi.export("invoked", kebab_names.kebab_module.do_something_output(the_input="hello",
    nested={
        "value": "nested",
    }).the_output)
