import pulumi
import pulumi_azure_native as azure_native

server = azure_native.dbforpostgresql.Server("server",
    location="brazilsouth",
    properties=azure_native.dbforpostgresql.ServerPropertiesForRestoreArgs(
        create_mode="PointInTimeRestore",
        restore_point_in_time="2017-12-14T00:00:37.467Z",
        source_server_id="/subscriptions/ffffffff-ffff-ffff-ffff-ffffffffffff/resourceGroups/SourceResourceGroup/providers/Microsoft.DBforPostgreSQL/servers/sourceserver",
    ),
    resource_group_name="TargetResourceGroup",
    server_name="targetserver",
    sku=azure_native.dbforpostgresql.SkuArgs(
        capacity=2,
        family="Gen5",
        name="B_Gen5_2",
        tier="Basic",
    ),
    tags={
        "ElasticServer": "1",
    })
