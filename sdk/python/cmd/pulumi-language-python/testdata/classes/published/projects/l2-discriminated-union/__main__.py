import pulumi
import pulumi_discriminated_union as discriminated_union

example1 = discriminated_union.Example("example1",
    union_of=discriminated_union.VariantOneArgs(
        discriminant_kind="variant1",
        field1="v1 union",
    ),
    array_of_union_of=[discriminated_union.VariantOneArgs(
        discriminant_kind="variant1",
        field1="v1 array(union)",
    )])
example2 = discriminated_union.Example("example2",
    union_of=discriminated_union.VariantTwoArgs(
        discriminant_kind="variant2",
        field2="v2 union",
    ),
    array_of_union_of=[
        discriminated_union.VariantTwoArgs(
            discriminant_kind="variant2",
            field2="v2 array(union)",
        ),
        discriminated_union.VariantOneArgs(
            discriminant_kind="variant1",
            field1="v1 array(union)",
        ),
    ])
