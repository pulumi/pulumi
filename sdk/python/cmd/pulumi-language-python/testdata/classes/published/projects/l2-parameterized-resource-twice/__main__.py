import pulumi
import pulumi_byepackage as byepackage
import pulumi_hipackage as hipackage

# The resource name is based on the parameter value
example1 = hipackage.HelloWorld("example1")
example_component1 = hipackage.HelloWorldComponent("exampleComponent1")
pulumi.export("parameterValue1", example1.parameter_value)
pulumi.export("parameterValueFromComponent1", example_component1.parameter_value)
# The resource name is based on the parameter value
example2 = byepackage.GoodbyeWorld("example2")
example_component2 = byepackage.GoodbyeWorldComponent("exampleComponent2")
pulumi.export("parameterValue2", example2.parameter_value)
pulumi.export("parameterValueFromComponent2", example_component2.parameter_value)
