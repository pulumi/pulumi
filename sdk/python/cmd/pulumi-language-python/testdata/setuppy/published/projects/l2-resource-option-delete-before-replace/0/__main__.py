import pulumi
import pulumi_simple as simple

# Stage 0: Initial resource creation
# Resource with deleteBeforeReplace option
with_option = simple.Resource("withOption", value=True,
opts = pulumi.ResourceOptions(replace_on_changes=["value"],
    delete_before_replace=True))
# Resource without deleteBeforeReplace (default create-before-delete behavior)
without_option = simple.Resource("withoutOption", value=True,
opts = pulumi.ResourceOptions(replace_on_changes=["value"]))
