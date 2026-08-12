import pulumi
import pulumi_primitive_defaults as primitive_defaults

res_explicit = primitive_defaults.Resource("resExplicit",
    boolean=True,
    float=3.14,
    integer=42,
    string="hello")
res_defaulted = primitive_defaults.Resource("resDefaulted")
