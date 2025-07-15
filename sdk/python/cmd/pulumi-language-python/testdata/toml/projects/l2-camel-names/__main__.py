import pulumi
import pulumi_camelNames as camelNames

first_resource = camel_names.cool_module.SomeResource("firstResource", the_input=True)
second_resource = camel_names.cool_module.SomeResource("secondResource", the_input=first_resource.the_output)
