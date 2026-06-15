import pulumi
import pulumi_myext as extbase

greeting = extbase.Greeting("greeting")
greeting_comp = extbase.GreetingComponent("greetingComp")
pulumi.export("parameterValue", greeting.parameter_value)
pulumi.export("parameterValueFromComponent", greeting_comp.parameter_value)
