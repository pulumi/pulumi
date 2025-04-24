import pulumi
import pulumi_component as component

component1 = component.ComponentCustomRefOutput("component1", value="foo-bar-baz")
custom1 = component.Custom("custom1", value=component1.value)
custom2 = component.Custom("custom2", value=component1.ref.value)
