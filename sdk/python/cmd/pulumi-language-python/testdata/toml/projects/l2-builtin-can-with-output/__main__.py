import pulumi
import pulumi_component as component
import pulumi_simple_invoke as simple_invoke

def canOutput_(fn) -> pulumi.Output[bool]:
	try:
		result = pulumi.Output.from_input(fn())
		return result.apply(lambda x: x != None)
	except:
		return pulumi.Output.from_input(False)


def can_(fn) -> bool:
		try:
			result = fn()
			return True
		except:
			return False
	

component1 = component.ComponentCustomRefOutput("component1", value="foo-bar-baz")
component_can_should_be_true = canOutput_(lambda: component1.ref)
pulumi.export("componentCan", component_can_should_be_true)
invoke_can_should_be_true = canOutput_(lambda: simple_invoke.my_invoke_output(value="hello"))
pulumi.export("invokeCan", invoke_can_should_be_true)
ternary_should_not_use_apply = "option_one" if can_(lambda: True) else "option_two"
pulumi.export("ternaryCan", ternary_should_not_use_apply)
ternary_should_use_apply = canOutput_(lambda: component1.ref).apply(lambda can: "option_one" if can else "option_two")
pulumi.export("ternaryCanOutput", ternary_should_use_apply)
str = "str"
pulumi.export("scalarCan", can_(lambda: str))
