import pulumi

config = pulumi.Config()
names = config.get_object("names")
if names is None:
    names = [
        None,
        "hello",
        None,
    ]
pulumi.export("namesLength", len(names))
