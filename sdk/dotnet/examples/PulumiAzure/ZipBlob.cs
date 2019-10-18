using System;
using System.Collections.Immutable;

namespace Pulumi.Azure.Storage
{
    public class ZipBlob// : CustomResource
    {
        public Output<string> Name { get; }
        public Output<string> StorageContainerName { get; }

        public ZipBlob(string name, ZipBlobArgs args = default, ResourceOptions opts = default)// : base("storage.Container", name, props(args), opts)
        {
            this.Name = Output.Create(name + "abc123de");
            this.StorageContainerName = args.StorageAccountName;
            Console.WriteLine($"    └─ storage.ZipBlob        {name,-11} created");
        }
    }

    public class ZipBlobArgs
    {
        public Input<string> StorageAccountName { get; set; }
        public Input<string> StorageContainerName { get; set; }
        public Input<string> Type { get; set; }
        public Input<Asset.IArchive> Content { get; set; }
    }


    public static class SharedAccessSignature
    {
        public static Output<string> SignedBlobReadUrl(ZipBlob blob, Account account)
        {
            return Output
                .All<string>(account.Name, account.PrimaryConnectionString, blob.StorageContainerName, blob.Name)
                .Apply(values =>
                {
                    var sas = "12124=="; //TODO: Storage.GetAccountBlobContainerSAS();
                        return $"https://${values[0]}.blob.core.windows.net/${values[2]}/${values[3]}${sas}";
                });
        }
    }
}
