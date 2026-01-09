import pulumi
import pulumi_range as range

root = range.Root("root")
# creating resources by iterating a property of type array(string) of another resource
from_list_of_strings = []
def create_from_list_of_strings(range_body):
    for range in [{"key": k, "value": v} for [k, v] in enumerate(range_body)]:
        from_list_of_strings.append(range.Example(f"fromListOfStrings-{range['key']}", some_string=range["value"]))

root.array_of_string.apply(create_from_list_of_strings)
# creating resources by iterating a property of type map(string) of another resource
from_map_of_strings = []
def create_from_map_of_strings(range_body):
    for range in [{"key": k, "value": v} for [k, v] in enumerate(range_body)]:
        from_map_of_strings.append(range.Example(f"fromMapOfStrings-{range['key']}", some_string=f"{range['key']} {range['value']}"))

root.map_of_string.apply(create_from_map_of_strings)
# computed range list expression to create instances of range:index:Example resource
from_computed_list_of_strings = []
def create_from_computed_list_of_strings(range_body):
    for range in [{"key": k, "value": v} for [k, v] in enumerate(range_body)]:
        from_computed_list_of_strings.append(range.Example(f"fromComputedListOfStrings-{range['key']}", some_string=f"{range['key']} {range['value']}"))

pulumi.Output.all(
    root.map_of_string["hello"],
    root.map_of_string["world"]
).apply(create_from_computed_list_of_strings)
# computed range for expression to create instances of range:index:Example resource
from_computed_for_expression = []
def create_from_computed_for_expression(range_body):
    for range in [{"key": k, "value": v} for [k, v] in enumerate(range_body)]:
        from_computed_for_expression.append(range.Example(f"fromComputedForExpression-{range['key']}", some_string=f"{range['key']} {range['value']}"))

pulumi.Output.all(
    array_of_string=root.array_of_string,
    map_of_string=root.map_of_string
).apply(lambda resolved_outputs: create_from_computed_for_expression([resolved_outputs['map_of_string'][value] for value in resolved_outputs['array_of_string']]))
