using System.Collections.Generic;
using Pulumi.Azure.Core;
using Pulumi.Azure.Sql;
using Storage = Pulumi.Azure.Storage;
using AppService = Pulumi.Azure.AppService;

namespace Pulumi.CSharpExamples
{
    public class WebApp
    {
        public static IDictionary<string, Output<string>> Run()
        {
            var resourceGroup = new ResourceGroup("rg");

            var storageAccount = new Storage.Account("sa", new Storage.AccountArgs
            {
                ResourceGroupName = resourceGroup.Name,
                AccountKind = "StorageV2",
                AccountTier = "Standard",
                AccountReplicationType = "LRS",
            });

            var appServicePlan = new AppService.Plan("asp", new AppService.PlanArgs
            {
                ResourceGroupName = resourceGroup.Name,
                Kind = "App",
                Sku = new AppService.PlanSkuArgs
                {
                    Tier = "Basic",
                    Size = "B1",
                },
            });

            var container = new Storage.Container("c", new Storage.ContainerArgs
            {
                StorageAccountName = storageAccount.Name,
                ContainerAccessType = "private",
            });

            var blob = new Storage.ZipBlob("zip", new Storage.ZipBlobArgs
            {
                StorageAccountName = storageAccount.Name,
                StorageContainerName = container.Name,
                Type = "block",
                Content = new Asset.FileArchive("wwwroot"),
            });

            var codeBlobUrl = Storage.SharedAccessSignature.SignedBlobReadUrl(blob, storageAccount);

            var username = "sa"; // TODO: Pulumi.Config
            var password = "pwd";
            var sqlServer = new SqlServer("sql", new SqlServerArgs
            {
                ResourceGroupName = resourceGroup.Name,
                AdministratorLogin = username, 
                AdministratorLoginPassword = password,
                Version = "12.0",
            });

            var database = new Database("db", new DatabaseArgs
            {
                ResourceGroupName = resourceGroup.Name,
                ServerName = sqlServer.Name,
                RequestedServiceObjectiveName = "S0",
            });

            // Namespace == Class name feels awkward
            var app = new AppService.AppService("app", new AppService.AppServiceArgs
            {
                ResourceGroupName = resourceGroup.Name,
                AppServicePlanId = appServicePlan.Id,
                AppSettings =
                {
                    { "WEBSITE_RUN_FROM_ZIP", codeBlobUrl },
                },
                ConnectionStrings = new[]
                {
                    new AppService.ConnectionStringArgs 
                    { 
                        Name = "db", 
                        Type = "SQLAzure",
                        Value = Output.All<string>(sqlServer.Name, database.Name).Apply(values =>
                        {
                            return $"Server= tcp:${values[0]}.database.windows.net;initial catalog=${values[1]};userID=${username};password=${password};Min Pool Size=0;Max Pool Size=30;Persist Security Info=true;";
                        }),
                    },
                },
            });

            return new Dictionary<string, Output<string>>
            {
                { "endpoint", app.DefaultSiteHostname.Apply(hostname => $"https://{hostname}") }
            };
        }
    }
}
