using Pulumi.Rpc;
using System;
using System.Collections.Immutable;

namespace Pulumi.Azure.Storage
{
    public class ZipBlob : CustomResource
    {
        [ResourceField("name")]
        private readonly StringOutputCompletionSource _name;
        public Output<string> Name => _name.Output;

        [ResourceField("storageContainerName")]
        private readonly StringOutputCompletionSource _storageContainerName;
        public Output<string> StorageContainerName => _storageContainerName.Output;

        public ZipBlob(string name, ZipBlobArgs args = default, ResourceOptions opts = default) 
            : base("azure:storage/zipBlob:ZipBlob", name, args, opts)
        {
            _name = new StringOutputCompletionSource(this);
            _storageContainerName = new StringOutputCompletionSource(this);
            this.OnConstructorCompleted();
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
                .ApplyAsync(async values =>
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
