import pulumi

config = pulumi.Config()
names = config.require_object("names")
tags = config.require_object("tags")
pulumi.export("greetings", [f"Hello, {name}!" for name in names])
pulumi.export("numbered", [f"{i}-{name}" for i, name in enumerate(names)])
pulumi.export("tagList", [f"{k}={v}" for k, v in sorted(tags.items())])
pulumi.export("greetingMap", {name: f"Hello, {name}!" for name in names})
pulumi.export("filteredList", [name for name in names if name != "b"])
pulumi.export("filteredMap", {name: f"Hello, {name}!" for name in names if name != "b"})
