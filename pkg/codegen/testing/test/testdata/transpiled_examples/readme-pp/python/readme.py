import pulumi

pulumi.export("strVar", "foo")
pulumi.export("arrVar", [
    "fizz",
    "buzz",
])
pulumi.export("readme", (lambda path: open(path).read())("./Pulumi.README.md"))
