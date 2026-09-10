import pulumi

pulumi.export("expandedMax", max(*[
    1,
    2,
    3,
]))
pulumi.export("expandedMaxWithPrefix", max(0, *[
    1,
    2,
    3,
]))
