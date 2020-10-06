import pulumi
import pulumi_azure as azure

config = pulumi.Config()
resource_group_name_param = config.require("resourceGroupNameParam")
resource_group_var = azure.core.get_resource_group(name=resource_group_name_param)
location_param = config.get("locationParam")
if location_param is None:
    location_param = resource_group_var.location
