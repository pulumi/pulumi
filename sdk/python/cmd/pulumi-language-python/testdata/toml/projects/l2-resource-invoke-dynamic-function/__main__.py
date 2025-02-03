import pulumi
import pulumi_any_type_function as any_type_function

local_value = "hello"
pulumi.export("dynamic", any_type_function.dyn_list_to_dyn_output(inputs=[
    "hello",
    local_value,
    {},
]).apply(lambda invoke: invoke.result))
