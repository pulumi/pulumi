import pulumi
import pulumi_conformance_component as conformance_component
import pulumi_replaceonchanges as replaceonchanges
import pulumi_simple as simple

# Stage 0: Initial resource creation
# Scenario 1: Schema-based replaceOnChanges on replaceProp
schema_replace = replaceonchanges.ResourceA("schemaReplace",
    value=True,
    replace_prop=True)
# Scenario 2: Option-based replaceOnChanges on value
option_replace = replaceonchanges.ResourceB("optionReplace", value=True,
opts = pulumi.ResourceOptions(replace_on_changes=["value"]))
# Scenario 3: Both schema and option - will change value
both_replace_value = replaceonchanges.ResourceA("bothReplaceValue",
    value=True,
    replace_prop=True,
    opts = pulumi.ResourceOptions(replace_on_changes=["value"]))
# Scenario 4: Both schema and option - will change replaceProp
both_replace_prop = replaceonchanges.ResourceA("bothReplaceProp",
    value=True,
    replace_prop=True,
    opts = pulumi.ResourceOptions(replace_on_changes=["value"]))
# Scenario 5: No replaceOnChanges - baseline update
regular_update = replaceonchanges.ResourceB("regularUpdate", value=True)
# Scenario 6: replaceOnChanges set but no change
no_change = replaceonchanges.ResourceB("noChange", value=True,
opts = pulumi.ResourceOptions(replace_on_changes=["value"]))
# Scenario 7: replaceOnChanges on value, but only replaceProp changes
wrong_prop_change = replaceonchanges.ResourceA("wrongPropChange",
    value=True,
    replace_prop=True,
    opts = pulumi.ResourceOptions(replace_on_changes=["value"]))
# Scenario 8: Multiple properties in replaceOnChanges array
multiple_prop_replace = replaceonchanges.ResourceA("multiplePropReplace",
    value=True,
    replace_prop=True,
    opts = pulumi.ResourceOptions(replace_on_changes=[
            "value",
            "replaceProp",
        ]))
# Remote component with replaceOnChanges
remote_with_replace = conformance_component.Simple("remoteWithReplace", value=True,
opts = pulumi.ResourceOptions(replace_on_changes=["value"]))
# Keep a simple resource so all expected plugins are required.
simple_resource = simple.Resource("simpleResource", value=False)
