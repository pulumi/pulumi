import pulumi
import pulumi_simple as simple

config = pulumi.Config()
create_timeout = config.require("createTimeout")
no_timeouts = simple.Resource("noTimeouts", value=True)
create_only = simple.Resource("createOnly", value=True,
opts = pulumi.ResourceOptions(custom_timeouts=pulumi.CustomTimeouts(create="5m")))
update_only = simple.Resource("updateOnly", value=True,
opts = pulumi.ResourceOptions(custom_timeouts=pulumi.CustomTimeouts(update="10m")))
delete_only = simple.Resource("deleteOnly", value=True,
opts = pulumi.ResourceOptions(custom_timeouts=pulumi.CustomTimeouts(delete="3m")))
read_only = simple.Resource("readOnly", value=True,
opts = pulumi.ResourceOptions(custom_timeouts=pulumi.CustomTimeouts(read="9m")))
all_timeouts = simple.Resource("allTimeouts", value=True,
opts = pulumi.ResourceOptions(custom_timeouts=pulumi.CustomTimeouts(create="2m", update="4m", delete="1m", read="5m")))
config_timeout = simple.Resource("configTimeout", value=True,
opts = pulumi.ResourceOptions(custom_timeouts=pulumi.CustomTimeouts(create=create_timeout)))
