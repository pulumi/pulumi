import pulumi
import pulumi_conformance_component as conformance_component
import pulumi_replaceonchanges as replaceonchanges
import pulumi_simple as simple

# Stage 1: Change properties to trigger replacements
# Scenario 1: Change replaceProp → REPLACE (schema triggers)
schema_replace = replaceonchanges.ResourceA("schemaReplace",
    value=True,
    replace_prop=False)
# Changed from true
# Scenario 2: Change value → REPLACE (option triggers)
option_replace = replaceonchanges.ResourceB("optionReplace", value=False,
opts = pulumi.ResourceOptions(replace_on_changes=["value"]))
# Scenario 3: Change value → REPLACE (option on value triggers)
both_replace_value = replaceonchanges.ResourceA("bothReplaceValue",
    value=False,
    replace_prop=True,
    opts = pulumi.ResourceOptions(replace_on_changes=["value"]))
# Scenario 4: Change replaceProp → REPLACE (schema on replaceProp triggers)
both_replace_prop = replaceonchanges.ResourceA("bothReplaceProp",
    value=True,
    replace_prop=False,
    opts = pulumi.ResourceOptions(replace_on_changes=["value"]))
# Scenario 5: Change value → UPDATE (no replaceOnChanges)
regular_update = replaceonchanges.ResourceB("regularUpdate", value=False)
# Changed from true
# Scenario 6: No change → SAME (no operation)
no_change = replaceonchanges.ResourceB("noChange", value=True,
opts = pulumi.ResourceOptions(replace_on_changes=["value"]))
# Scenario 7: Change replaceProp (not value) → UPDATE (marked property unchanged)
wrong_prop_change = replaceonchanges.ResourceA("wrongPropChange",
    value=True,
    replace_prop=False,
    opts = pulumi.ResourceOptions(replace_on_changes=["value"]))
# Scenario 8: Change value → REPLACE (multiple properties marked)
multiple_prop_replace = replaceonchanges.ResourceA("multiplePropReplace",
    value=False,
    replace_prop=True,
    opts = pulumi.ResourceOptions(replace_on_changes=[
            "value",
            "replaceProp",
        ]))
# Remote component: change value → REPLACE
remote_with_replace = conformance_component.Simple("remoteWithReplace", value=False,
opts = pulumi.ResourceOptions(replace_on_changes=["value"]))
# Keep a simple resource so all expected plugins are required.
simple_resource = simple.Resource("simpleResource", value=False)
