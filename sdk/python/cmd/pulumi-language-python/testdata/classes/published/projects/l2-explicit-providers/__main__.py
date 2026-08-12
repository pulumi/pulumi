import pulumi
import pulumi_component as component

explicit = component.Provider("explicit")
list = component.ComponentCallable("list", value="value",
opts = pulumi.ResourceOptions(providers=[explicit]))
map = component.ComponentCallable("map", value="value",
opts = pulumi.ResourceOptions(providers={
        "component": explicit,
    }))
