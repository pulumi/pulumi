import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import pulumi_simple as simple

class KeywordComponentArgs(TypedDict, total=False):
    input: Input[bool]

class KeywordComponent(pulumi.ComponentResource):
    def __init__(self, name: str, args: KeywordComponentArgs, opts:Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:KeywordComponent", name, args, opts)

        # A resource named `this` collides with the receiver pointer of the
        # ComponentResource class generated for this component. NodeJS must rename the
        # resource variable (e.g. to `_this`) while keeping the `parent: this` pointer
        # intact.
        this = simple.Resource(f"{name}-this", value=args["input"],
        opts = pulumi.ResourceOptions(parent=self))

        # Referencing `this` exercises that the rename is applied to references too, not
        # just the declaration. The name `parent` also overlaps with the `parent`
        # resource-option key, which must not be confused with this resource variable.
        parent = simple.Resource(f"{name}-parent", value=this.value,
        opts = pulumi.ResourceOptions(parent=self))

        self.result = parent.value
        self.register_outputs({
            'result': parent.value
        })