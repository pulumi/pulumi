import pulumi
import pulumi_myext as myext

greeting = myext.Greeting("greeting")
greeting_comp = myext.GreetingComponent("greetingComp")
pulumi.export("parameterValue", greeting.parameter_value)
pulumi.export("parameterValueFromComponent", greeting_comp.parameter_value)
pulumi.export("invokeGreeting", myext.greet_output(name="Pulumi").apply(lambda invoke: invoke.greeting))
