import pulumi


def not_implemented(msg):
    raise NotImplementedError(msg)

pulumi.export("result", not_implemented("expression here is not implemented yet"))
