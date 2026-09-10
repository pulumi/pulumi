import pulumi
import pulumi_discriminated_union_marked_key as discriminated_union_marked_key

first = discriminated_union_marked_key.Example("first", union_in=discriminated_union_marked_key.VariantTwoArgs(
    discriminant_kind="variant2",
    field2="known",
))
second = discriminated_union_marked_key.Example("second", union_in=first.union_out)
pulumi.export("out", second.union_out)
