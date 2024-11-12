import pulumi
import pulumi_subpackage as subpackage

# The resource name is based on the parameter value
example = subpackage.HelloWorld("example")
pulumi.export("parameterValue", example.parameter_value)
