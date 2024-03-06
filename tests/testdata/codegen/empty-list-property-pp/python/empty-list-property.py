import pulumi
import pulumi_azure_native as azure_native

storage_accounts = azure_native.storage.StorageAccount("storageAccounts",
    account_name="sto4445",
    kind="BlockBlobStorage",
    location="eastus",
    resource_group_name="res9101",
    sku=azure_native.storage.SkuArgs(
        name="Premium_LRS",
    ),
    network_rule_set=azure_native.storage.NetworkRuleSetArgs(
        default_action=azure_native.storage.DefaultAction.ALLOW,
        ip_rules=[],
    ))
