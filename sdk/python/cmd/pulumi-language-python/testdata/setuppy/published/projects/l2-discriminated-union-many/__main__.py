import pulumi
import pulumi_discriminated_union_many as discriminated_union_many

example1 = discriminated_union_many.Example("example1", union_of={
    "discriminant_kind": "variant1",
    "payload": "p1",
    "extra": "e1",
})
example2 = discriminated_union_many.Example("example2", union_of={
    "discriminant_kind": "variant2",
    "payload": "p2",
    "extra": "e2",
})
example3 = discriminated_union_many.Example("example3", union_of={
    "discriminant_kind": "variant3",
    "payload": "p3",
    "count": 3,
})
example4 = discriminated_union_many.Example("example4", union_of={
    "discriminant_kind": "variant4",
    "payload": "p4",
    "enabled": True,
})
example5 = discriminated_union_many.Example("example5", union_of={
    "discriminant_kind": "variant5",
    "payload": "p5",
    "label": "l5",
})
example6 = discriminated_union_many.Example("example6", union_of={
    "discriminant_kind": "variant6",
    "payload": "p6",
    "code": 6,
})
example7 = discriminated_union_many.Example("example7", union_of={
    "discriminant_kind": "variant7",
    "payload": "p7",
    "message": "m7",
})
example8 = discriminated_union_many.Example("example8", union_of={
    "discriminant_kind": "variant8",
    "payload": "p8",
    "size": 8,
})
example9 = discriminated_union_many.Example("example9", union_of={
    "discriminant_kind": "variant9",
    "payload": "p9",
    "flag": False,
})
example10 = discriminated_union_many.Example("example10", union_of={
    "discriminant_kind": "variant10",
    "payload": "p10",
    "note": "n10",
})
