import pulumi
import pulumi_plaincomponent as plaincomponent

my_component = plaincomponent.Component("myComponent",
    name="my-resource",
    settings=plaincomponent.SettingsArgs(
        enabled=True,
        tags={
            "env": "test",
        },
    ))
pulumi.export("label", my_component.label)
