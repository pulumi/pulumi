import pulumi
import pulumi_inherit as inherit

derived = inherit.Derived("derived",
    message="hello",
    scale=3)
pulumi.export("baseOutput", derived.base_output)
pulumi.export("derivedOutput", derived.derived_output)
