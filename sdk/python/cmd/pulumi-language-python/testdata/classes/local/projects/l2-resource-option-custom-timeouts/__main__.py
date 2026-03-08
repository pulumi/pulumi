import pulumi
import pulumi_simple as simple

no_timeouts = simple.Resource("noTimeouts", value=True)
create_only = simple.Resource("createOnly", value=True,
opts = pulumi.ResourceOptions(custom_timeouts=pulumi.CustomTimeouts(create="5m")))
update_only = simple.Resource("updateOnly", value=True,
opts = pulumi.ResourceOptions(custom_timeouts=pulumi.CustomTimeouts(update="10m")))
delete_only = simple.Resource("deleteOnly", value=True,
opts = pulumi.ResourceOptions(custom_timeouts=pulumi.CustomTimeouts(delete="3m")))
all_timeouts = simple.Resource("allTimeouts", value=True,
opts = pulumi.ResourceOptions(custom_timeouts=pulumi.CustomTimeouts(create="2m", update="4m", delete="1m")))
