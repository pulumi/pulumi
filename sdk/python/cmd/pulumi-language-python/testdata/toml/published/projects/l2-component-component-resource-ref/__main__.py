import pulumi
import pulumi_component as component

component1 = component.ComponentCustomRefOutput("component1", value="foo-bar-baz")
component2 = component.ComponentCustomRefInputOutput("component2", input_ref=component1.ref)
custom1 = component.Custom("custom1", value=component2.input_ref.value)
custom2 = component.Custom("custom2", value=component2.output_ref.value)
