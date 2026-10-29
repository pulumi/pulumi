import pulumi
import pulumi_failingcomponent as failingcomponent
import pulumi_simple as simple

comp = failingcomponent.Component("comp")
dependent = simple.Resource("dependent", value=comp.value.apply(lambda value: value == "hello"))
independent = simple.Resource("independent", value=True)
pulumi.export("componentValue", comp.value)
