using Pulumi.Rpc;
using System;
using System.Collections.Immutable;

namespace Pulumi.Azure.Storage
{
    public class ZipBlob : CustomResource
    {
        [ResourceField("name")]
        private readonly StringOutputCompletionSource _name;
        public Output<string> Name1 => _name.Output;

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
                .All<string>(account.Name1, account.PrimaryConnectionString, blob.StorageContainerName, blob.Name1)
                .Apply(values =>
                {
                    // TODO 
                    //const sas = await storage.getAccountBlobContainerSAS({
                    //    connectionString,
                    //    containerName,
                    //    start: "2019-01-01",
                    //    expiry: signatureExpiration,
                    //    permissions:
                    //        {
                    //        read: true,
                    //        write: false,
                    //        delete: false,
                    //        list: false,
                    //        add: false,
                    //        create: false,
                    //    },
                    //}, { async: true });
                    return $"https://{values[0]}.blob.core.windows.net/{values[2]}/{values[3]}?st=2019-10-21T14%3A32%3A52Z&se=2020-10-22T11%3A10%3A00Z&sp=r&sv=2018-03-28&sr=b&sig=gZmY3QZHdGzAIwzh5WpgC%2FkNtFsvbRKGSfBJZTq078E%3D";
                });
        }
    }
}
