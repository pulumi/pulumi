import pulumi
import pulumi_azure_native as azure_native

database_account = azure_native.documentdb.DatabaseAccount("databaseAccount",
    account_name="ddb1",
    api_properties=azure_native.documentdb.ApiPropertiesArgs(
        server_version="3.2",
    ),
    backup_policy=azure_native.documentdb.PeriodicModeBackupPolicyArgs(
        periodic_mode_properties=azure_native.documentdb.PeriodicModePropertiesArgs(
            backup_interval_in_minutes=240,
            backup_retention_interval_in_hours=8,
        ),
        type="Periodic",
    ),
    database_account_offer_type=azure_native.documentdb.DatabaseAccountOfferType.STANDARD,
    locations=[azure_native.documentdb.LocationArgs(
        failover_priority=0,
        is_zone_redundant=False,
        location_name="sourthcentralus",
    )],
    resource_group_name="rg1")
