import * as pulumi from "@pulumi/pulumi";
import * as azure_native from "@pulumi/azure-native";
import * as random from "@pulumi/random";

const config = new pulumi.Config();
const sqlAdmin = config.get("sqlAdmin") || "pulumi";
const appservicegroup = new azure_native.resources.ResourceGroup("appservicegroup", {});
const sa = new azure_native.storage.StorageAccount("sa", {
    resourceGroupName: appservicegroup.name,
    kind: azure_native.storage.Kind.StorageV2,
    sku: {
        name: azure_native.storage.SkuName.Standard_LRS,
    },
});
const container = new azure_native.storage.BlobContainer("container", {
    resourceGroupName: appservicegroup.name,
    accountName: sa.name,
    publicAccess: azure_native.storage.PublicAccess.None,
});
const blobAccessToken = pulumi.secret(pulumi.all([sa.name, appservicegroup.name, sa.name, container.name]).apply(([saName, appservicegroupName, saName1, containerName]) => azure_native.storage.listStorageAccountServiceSASOutput({
    accountName: saName,
    protocols: azure_native.storage.HttpProtocol.Https,
    sharedAccessStartTime: "2022-01-01",
    sharedAccessExpiryTime: "2030-01-01",
    resource: azure_native.storage.SignedResource.C,
    resourceGroupName: appservicegroupName,
    permissions: azure_native.storage.Permissions.R,
    canonicalizedResource: `/blob/${saName1}/${containerName}`,
    contentType: "application/json",
    cacheControl: "max-age=5",
    contentDisposition: "inline",
    contentEncoding: "deflate",
})).apply(invoke => invoke.serviceSasToken));
const appserviceplan = new azure_native.web.AppServicePlan("appserviceplan", {
    resourceGroupName: appservicegroup.name,
    kind: "App",
    sku: {
        name: "B1",
        tier: "Basic",
    },
});
const blob = new azure_native.storage.Blob("blob", {
    resourceGroupName: appservicegroup.name,
    accountName: sa.name,
    containerName: container.name,
    type: azure_native.storage.BlobType.Block,
    source: new pulumi.asset.FileArchive("./www"),
});
const appInsights = new azure_native.insights.Component("appInsights", {
    resourceGroupName: appservicegroup.name,
    applicationType: azure_native.insights.ApplicationType.Web,
    kind: "web",
});
const sqlPassword = new random.RandomPassword("sqlPassword", {
    length: 16,
    special: true,
});
const sqlServer = new azure_native.sql.Server("sqlServer", {
    resourceGroupName: appservicegroup.name,
    administratorLogin: sqlAdmin,
    administratorLoginPassword: sqlPassword.result,
    version: "12.0",
});
const db = new azure_native.sql.Database("db", {
    resourceGroupName: appservicegroup.name,
    serverName: sqlServer.name,
    sku: {
        name: "S0",
    },
});
const app = new azure_native.web.WebApp("app", {
    resourceGroupName: appservicegroup.name,
    serverFarmId: appserviceplan.id,
    siteConfig: {
        appSettings: [
            {
                name: "WEBSITE_RUN_FROM_PACKAGE",
                value: pulumi.interpolate`https://${sa.name}.blob.core.windows.net/${container.name}/${blob.name}?${blobAccessToken}`,
            },
            {
                name: "APPINSIGHTS_INSTRUMENTATIONKEY",
                value: appInsights.instrumentationKey,
            },
            {
                name: "APPLICATIONINSIGHTS_CONNECTION_STRING",
                value: pulumi.interpolate`InstrumentationKey=${appInsights.instrumentationKey}`,
            },
            {
                name: "ApplicationInsightsAgent_EXTENSION_VERSION",
                value: "~2",
            },
        ],
        connectionStrings: [{
            name: "db",
            type: azure_native.web.ConnectionStringType.SQLAzure,
            connectionString: pulumi.interpolate`Server= tcp:${sqlServer.name}.database.windows.net;initial catalog=${db.name};userID=${sqlAdmin};password=${sqlPassword.result};Min Pool Size=0;Max Pool Size=30;Persist Security Info=true;`,
        }],
    },
});
export const endpoint = app.defaultHostName;
