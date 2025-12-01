import pulumi
import pulumi_subpackage as subpackage

# The resource name is based on the parameter value
example = subpackage.HelloWorld("example")
example_component = subpackage.HelloWorldComponent("exampleComponent")
pulumi.export("parameterValue", example.parameter_value)
pulumi.export("parameterValueFromComponent", example_component.parameter_value)
