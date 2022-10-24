import pulumi

basic_str_var = "foo"
pulumi.export("strVar", basic_str_var)
pulumi.export("computedStrVar", f"{basic_str_var}/computed")
pulumi.export("strArrVar", [
    "fiz",
    "buss",
])
pulumi.export("intVar", 42)
pulumi.export("intArr", [
    1,
    2,
    3,
    4,
    5,
])
pulumi.export("readme", (lambda path: open(path).read())("./Pulumi.README.md"))
