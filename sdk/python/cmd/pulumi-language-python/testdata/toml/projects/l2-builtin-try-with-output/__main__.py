import pulumi
import pulumi_component as component
import pulumi_simple_invoke as simple_invoke
import typing

def tryOutput_(*fns) -> pulumi.Output[typing.Any]:
	if len(fns) == 0:
		raise Exception("tryOutput: all parameters failed")

	fn, *rest = fns
	result_output = None
	try:
		result = fn()
		result_output = pulumi.Output.from_input(result)
	except:
		return tryOutput_(*rest)

	return result_output


def try_(*fns) -> typing.Any:
		for fn in fns:
			try:
				return fn()
			except:
				continue
		raise Exception("try: all parameters failed")
	

component1 = component.ComponentCustomRefOutput("component1", value="foo-bar-baz")
# TODO(pulumi/pulumi#18895) When value is directly a scope traversal inside the
# output this fails to generate the "apply" call. eg if the output's internals
# are `value = componentTried.value`
#
# Apply is used when a resource output attribute is accessed.
component_tried = tryOutput_(
    lambda: component1.ref,
    lambda: "fallback"
).apply(lambda try_: try_.value)
pulumi.export("tryWithOutput", component_tried)
component_tried_nested = tryOutput_(
    lambda: component1.ref.value,
    lambda: "fallback"
)
pulumi.export("tryWithOutputNested", component_tried_nested)
# Invokes produces outputs. 
# This output will have apply called on it and try utilized within the apply.
# The result of this apply is already an output which has apply called on it
# again to pull out `result`
result_containing_output = tryOutput_(
    lambda: simple_invoke.my_invoke_output(value="hello"),
    lambda: "fakefallback"
).apply(lambda try_: try_.result)
pulumi.export("hello", result_containing_output)
str = "str"
pulumi.export("tryScalar", try_(
    lambda: str,
    lambda: "fallback"
))
