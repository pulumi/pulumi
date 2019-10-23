using Pulumi.Serialization;

namespace Pulumi.Azure.Storage
{
    public class ZipBlob : CustomResource
    {
        [OutputProperty("name")]
        public Output<string> Name { get; private set; }

        [OutputProperty("storageContainerName")]
        public Output<string> StorageContainerName { get; private set; }

        public ZipBlob(string name, ZipBlobArgs args = default, ResourceOptions opts = default) 
            : base("azure:storage/zipBlob:ZipBlob", name, args, opts)
        {
        }
    }

    public class ZipBlobArgs : ResourceArgs
    {
        public Input<AssetOrArchive> Content { get; set; }
        public Input<string> StorageAccountName { get; set; }
        public Input<string> StorageContainerName { get; set; }
        public Input<string> Type { get; set; }

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("content", Content);
            builder.Add("storageAccountName", StorageAccountName);
            builder.Add("storageContainerName", StorageContainerName);
            builder.Add("type", Type);
        }
    }


    public static class SharedAccessSignature
    {
        public static Output<string> SignedBlobReadUrl(ZipBlob blob, Account account)
        {
            return Output
                .All<string>(account.Name, account.PrimaryConnectionString, blob.StorageContainerName, blob.Name)
                .Apply(async values =>
                {
                    var sas = await DataSource.GetAccountBlobContainerSAS(
                        new GetAccountBlobContainerSASArgs
                        {
                            ConnectionString = values[1],
                            ContainerName = values[2],
                            Start = "2019-01-01",
                            Expiry = "2100-01-01",
                            Permissions = new GetAccountBlobContainerSASPermissions
                            {
                                Read = true,
                                Write = false,
                                Delete = false,
                                List = false,
                                Add = false,
                                Create = false,
                            },
                        }
                    );
                    return $"https://{values[0]}.blob.core.windows.net/{values[2]}/{values[3]}{sas}";
                });
        }
    }
}
