import pulumi
import pulumi_snake_names as snake_names

# Resource inputs are correctly translated
first = snake_names.cool_module.Some_resource("first",
    the_input=True,
    nested=snake_names.cool_module.Nested_inputArgs(
        nested_value="nested",
    ))
pulumi.export("theOutput", first.the_output)
# Datasource outputs are correctly translated
third = snake_names.cool_module.Another_resource("third", the_input=snake_names.cool_module.some_data_output(the_input=first.the_output["someKey"][0].nested_output,
    nested=[snake_names.cool_module.Entry(
        value="fuzz",
    )]).nested_output[0]["key"].value)
