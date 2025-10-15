import pulumi
import pulumi_camelNames as camel_names

first_resource = camel_names.coolmodule.SomeResource("firstResource", the_input=True)
second_resource = camel_names.coolmodule.SomeResource("secondResource", the_input=first_resource.the_output)
