# The output class a generated SDK emits for an object type.
import pulumi

__all__ = ["Inner"]


@pulumi.output_type
class Inner(dict):
    n: int = pulumi.property("n")
