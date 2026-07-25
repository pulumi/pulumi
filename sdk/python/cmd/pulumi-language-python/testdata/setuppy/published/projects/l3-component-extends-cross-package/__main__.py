import pulumi
import pulumi_inheritderived as inheritderived

derived = inheritderived.DerivedComponent("derived",
    message="hello",
    scale=7)
pulumi.export("baseOutput", derived.base_output)
pulumi.export("derivedOutput", derived.derived_output)
