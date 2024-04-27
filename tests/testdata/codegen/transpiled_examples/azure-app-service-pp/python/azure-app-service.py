import pulumi
import pulumi_azure_native as azure_native
import pulumi_random as random

config = pulumi.Config()
sql_admin = config.get("sqlAdmin")
if sql_admin is None:
    sql_admin = "pulumi"
appservicegroup = azure_native.resources.ResourceGroup("appservicegroup")
sa = azure_native.storage.StorageAccount("sa",
    resource_group_name=appservicegroup.name,
    kind=azure_native.storage.Kind.STORAGE_V2,
    sku=azure_native.storage.SkuArgs(
        name=azure_native.storage.SkuName.STANDARD_LRS,
    ))
container = azure_native.storage.BlobContainer("container",
    resource_group_name=appservicegroup.name,
    account_name=sa.name,
    public_access=azure_native.storage.PublicAccess.NONE)
blob_access_token = pulumi.Output.secret(pulumi.Output.all(sa.name, appservicegroup.name, sa.name, container.name).apply(lambda saName, appservicegroupName, saName1, containerName: azure_native.storage.list_storage_account_service_sas_output(account_name=sa_name,
    protocols=azure_native.storage.HttpProtocol.HTTPS,
    shared_access_start_time="2022-01-01",
    shared_access_expiry_time="2030-01-01",
    resource=azure_native.storage.SignedResource.C,
    resource_group_name=appservicegroup_name,
    permissions=azure_native.storage.Permissions.R,
    canonicalized_resource=f"/blob/{sa_name1}/{container_name}",
    content_type="application/json",
    cache_control="max-age=5",
    content_disposition="inline",
    content_encoding="deflate")).apply(lambda invoke: invoke.service_sas_token))
appserviceplan = azure_native.web.AppServicePlan("appserviceplan",
    resource_group_name=appservicegroup.name,
    kind="App",
    sku=azure_native.web.SkuDescriptionArgs(
        name="B1",
        tier="Basic",
    ))
blob = azure_native.storage.Blob("blob",
    resource_group_name=appservicegroup.name,
    account_name=sa.name,
    container_name=container.name,
    type=azure_native.storage.BlobType.BLOCK,
    source=pulumi.FileArchive("./www"))
app_insights = azure_native.insights.Component("appInsights",
    resource_group_name=appservicegroup.name,
    application_type=azure_native.insights.ApplicationType.WEB,
    kind="web")
sql_password = random.RandomPassword("sqlPassword",
    length=16,
    special=True)
sql_server = azure_native.sql.Server("sqlServer",
    resource_group_name=appservicegroup.name,
    administrator_login=sql_admin,
    administrator_login_password=sql_password.result,
    version="12.0")
db = azure_native.sql.Database("db",
    resource_group_name=appservicegroup.name,
    server_name=sql_server.name,
    sku=azure_native.sql.SkuArgs(
        name="S0",
    ))
app = azure_native.web.WebApp("app",
    resource_group_name=appservicegroup.name,
    server_farm_id=appserviceplan.id,
    site_config=azure_native.web.SiteConfigArgs(
        app_settings=[
            azure_native.web.NameValuePairArgs(
                name="WEBSITE_RUN_FROM_PACKAGE",
                value=pulumi.Output.all(sa.name, container.name, blob.name, blob_access_token).apply(lambda saName, containerName, blobName, blob_access_token: f"https://{sa_name}.blob.core.windows.net/{container_name}/{blob_name}?{blob_access_token}"),
            ),
            azure_native.web.NameValuePairArgs(
                name="APPINSIGHTS_INSTRUMENTATIONKEY",
                value=app_insights.instrumentation_key,
            ),
            azure_native.web.NameValuePairArgs(
                name="APPLICATIONINSIGHTS_CONNECTION_STRING",
                value=app_insights.instrumentation_key.apply(lambda instrumentation_key: f"InstrumentationKey={instrumentation_key}"),
            ),
            azure_native.web.NameValuePairArgs(
                name="ApplicationInsightsAgent_EXTENSION_VERSION",
                value="~2",
            ),
        ],
        connection_strings=[azure_native.web.ConnStringInfoArgs(
            name="db",
            type=azure_native.web.ConnectionStringType.SQL_AZURE,
            connection_string=pulumi.Output.all(sql_server.name, db.name, sql_password.result).apply(lambda sqlServerName, dbName, result: f"Server= tcp:{sql_server_name}.database.windows.net;initial catalog={db_name};userID={sql_admin};password={result};Min Pool Size=0;Max Pool Size=30;Persist Security Info=true;"),
        )],
    ))
pulumi.export("endpoint", app.default_host_name)
