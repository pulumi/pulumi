import pulumi
import pulumi_azure_native as azure_native

storage_accounts = azure_native.storage.StorageAccount("storageAccounts",
    account_name="sto4445",
    kind=azure_native.storage.Kind.BLOCK_BLOB_STORAGE,
    location="eastus",
    resource_group_name="res9101",
    sku={
        "name": azure_native.storage.SkuName.PREMIUM_LRS,
    },
    network_rule_set={
        "default_action": azure_native.storage.DefaultAction.ALLOW,
        "ip_rules": [],
    })
