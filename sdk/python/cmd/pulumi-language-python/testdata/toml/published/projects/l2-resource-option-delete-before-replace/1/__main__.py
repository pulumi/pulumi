import pulumi
import pulumi_simple as simple

# Stage 1: Change properties to trigger replacements
# Resource with deleteBeforeReplace option - should delete before creating
with_option = simple.Resource("withOption", value=False,
opts = pulumi.ResourceOptions(replace_on_changes=["value"],
    delete_before_replace=True))
# Resource without deleteBeforeReplace - should create before deleting (default)
without_option = simple.Resource("withoutOption", value=False,
opts = pulumi.ResourceOptions(replace_on_changes=["value"]))
