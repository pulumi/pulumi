import pulumi
import pulumi_component_property_deps as component_property_deps

custom1 = component_property_deps.Custom("custom1", value="hello")
custom2 = component_property_deps.Custom("custom2", value="world")
component1 = component_property_deps.Component("component1",
    resource=custom1,
    resource_list=[
        custom1,
        custom2,
    ],
    resource_map={
        "one": custom1,
        "two": custom2,
    })
pulumi.export("propertyDepsFromCall", component1.refs(resource=custom1,
    resource_list=[
        custom1,
        custom2,
    ],
    resource_map={
        "one": custom1,
        "two": custom2,
    }).apply(lambda call: call.result))
