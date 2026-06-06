import pulumi
import pulumi_dash_names as dash_names

first = dash_names.dash_module.Some_resource("first",
    the_input=True,
    nested_value={
        "nested_value": "nested",
    })
third = dash_names.dash_module.Another_resource("third", the_input=dash_names.dash_module.some_data_output(the_input=first.the_output[0].nested_output,
    entry_values=["fuzz"]).apply(lambda invoke: invoke.nested_output[0].entry_value))
trailing = dash_names.dash_module.Trailing_resource_("trailing", trailing_input_=dash_names.dash_module.trailing_data__output(trailing_input_="some-name-").apply(lambda invoke: invoke.trailing_output_))
