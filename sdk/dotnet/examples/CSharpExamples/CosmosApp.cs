// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System.Collections.Generic;
using Pulumi.Azure.Core;
using Pulumi.Azure.AppService;
using Pulumi.Azure.Storage;
using CosmosDB = Pulumi.Azure.CosmosDB;
using System;
using System.Linq;

namespace Pulumi.CSharpExamples
{
    public class CosmosAppArgs
    {
        public Input<string>? ResourceGroupName { get; set; }
        public InputList<string>? Locations { get; set; }
        public Input<string>? DatabaseName { get; set; }
        public Input<string>? ContainerName { get; set; }
    }

    public class CosmosApp : ComponentResource
    {
        public CosmosApp(string name, CosmosAppArgs args, ResourceOptions? options = null)
            : base("examples:azure:CosmosApp", name, options)
        {
            if (args.Locations == null)
            {
                throw new ArgumentException(nameof(args.Locations));
            }

            var primaryLocation = args.Locations.ToOutput().Apply(ls => ls[0]);

            var cosmosAccount = new CosmosDB.Account($"cosmos-{name}",
                new CosmosDB.AccountArgs
                {
                    ResourceGroupName = args.ResourceGroupName,
                    Location = primaryLocation,
                    //GeoLocations = args.Locations.ToOutput().Apply(ls => ls.Select((location, failoverPriority) => ({ location, failoverPriority }))),
                    OfferType = "Standard",
                    //consistencyPolicy = new 
                    //{
                    //    consistencyLevel: "Session",
                    //    maxIntervalInSeconds: 300,
                    //    maxStalenessPrefix: 100000,
                    //},
                });
        }
    }

    public class GlobalApp
    {
        public static Dictionary<string, object> Run()
        {
            var resourceGroup = new ResourceGroup("dotnet-rg", new ResourceGroupArgs
            {
                Location = "West Europe"
            });

            var storageAccount = new Account("sa", new AccountArgs
            {
                ResourceGroupName = resourceGroup.Name,
                AccountReplicationType = "LRS",
                AccountTier = "Standard",
            });

            var appServicePlan = new Plan("asp", new PlanArgs
            {
                ResourceGroupName = resourceGroup.Name,
                Kind = "App",
                Sku = new PlanSkuArgs
                {
                    Tier = "Basic",
                    Size = "B1",
                },
            });

            var container = new Container("c", new ContainerArgs
            {
                StorageAccountName = storageAccount.Name,
                ContainerAccessType = "private",
            });

            var blob = new ZipBlob("zip", new ZipBlobArgs
            {
                StorageAccountName = storageAccount.Name,
                StorageContainerName = container.Name,
                Type = "block",
                Content = new FileArchive("wwwroot"),
            });

            var codeBlobUrl = SharedAccessSignature.SignedBlobReadUrl(blob, storageAccount);

            //var username = "sa"; // TODO: Pulumi.Config
            //var password = "pwd";
            //var sqlServer = new SqlServer("sql", new SqlServerArgs
            //{
            //    ResourceGroupName = resourceGroup.Name,
            //    AdministratorLogin = username,
            //    AdministratorLoginPassword = password,
            //    Version = "12.0",
            //});

            //var database = new Database("db", new DatabaseArgs
            //{
            //    ResourceGroupName = resourceGroup.Name,
            //    ServerName = sqlServer.Name,
            //    RequestedServiceObjectiveName = "S0",
            //});

            var app = new AppService("app", new AppServiceArgs
            {
                ResourceGroupName = resourceGroup.Name,
                AppServicePlanId = appServicePlan.Id,
                AppSettings =
                {
                    { "WEBSITE_RUN_FROM_ZIP", codeBlobUrl },
                },
                //ConnectionStrings = new[]
                //{
                //    new AppService.ConnectionStringArgs
                //    {
                //        Name = "db",
                //        Type = "SQLAzure",
                //        Value = Output.All<string>(sqlServer.Name, database.Name).Apply(values =>
                //        {
                //            return $"Server= tcp:${values[0]}.database.windows.net;initial catalog=${values[1]};userID=${username};password=${password};Min Pool Size=0;Max Pool Size=30;Persist Security Info=true;";
                //        }),
                //    },
                //},
            });

            return new Dictionary<string, object>
            {
                { "endpoint", app.DefaultSiteHostname },
            };
        }
    }
}
