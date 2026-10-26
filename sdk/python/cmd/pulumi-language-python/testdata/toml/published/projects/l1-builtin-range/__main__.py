import pulumi

config = pulumi.Config()
from_ = config.require_int("from")
to = config.require_int("to")
pulumi.export("rangeTo", range(to))
pulumi.export("rangeFromTo", range(from_, to))
pulumi.export("literalTo", range(3))
pulumi.export("literalFromTo", range(2, 5))
pulumi.export("literalEmpty", range(3, 3))
