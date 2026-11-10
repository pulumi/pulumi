import pulumi

pulumi.export("sumTimes", (1 + 2) * float(3))
pulumi.export("minusMinus", float(10) - (4 - 3))
pulumi.export("notAnd", not (True and False))
pulumi.export("negateSum", -(1 + 2))
pulumi.export("conditionalIndex", ([1] if True else [2])[0])
pulumi.export("conditionalCondition", "yes" if (False if True else True) else "no")
