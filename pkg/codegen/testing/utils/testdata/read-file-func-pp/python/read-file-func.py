import pulumi

key = (lambda path: open(path).read())("key.pub")
pulumi.export("result", key)
