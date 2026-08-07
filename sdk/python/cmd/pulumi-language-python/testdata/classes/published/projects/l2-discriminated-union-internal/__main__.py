import pulumi
import pulumi_discriminated_union_internal as discriminated_union_internal

example1 = discriminated_union_internal.Example("example1",
    union_of=discriminated_union_internal.AlphaArgs(
        type__="Alpha",
        payload="p1",
        weight=1,
    ),
    secret_union=discriminated_union_internal.BetaArgs(
        type__="Beta",
        payload="s1",
        tint="blue",
    ))
example2 = discriminated_union_internal.Example("example2", union_of=discriminated_union_internal.BetaArgs(
    type__="Beta",
    payload="p2",
    tint="red",
))
example3 = discriminated_union_internal.Example("example3", union_of=discriminated_union_internal.GammaArgs(
    type__="Gamma",
    payload="p3",
    active=True,
))
