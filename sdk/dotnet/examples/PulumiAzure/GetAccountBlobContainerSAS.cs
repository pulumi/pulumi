using System.Threading.Tasks;

namespace Pulumi.Azure.Storage
{
    public class GetAccountBlobContainerSASArgs : ResourceArgs
    {
        public Input<string> ConnectionString { get; set; }
        public Input<string> ContainerName { get; set; }
        public Input<string> Start { get; set; }
        public Input<string> Expiry { get; set; }
        public Input<GetAccountBlobContainerSASPermissions> Permissions { get; set; }

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("connectionString", ConnectionString);
            builder.Add("containerName", ContainerName);
            builder.Add("start", Start);
            builder.Add("expiry", Expiry);
            builder.Add("permissions", Permissions);
        }
    }

    public class GetAccountBlobContainerSASPermissions : ResourceArgs
    {
        public Input<bool> Add { get; set; }
        public Input<bool> Create { get; set; }
        public Input<bool> Delete { get; set; }
        public Input<bool> List { get; set; }
        public Input<bool> Read { get; set; }
        public Input<bool> Write { get; set; }

        protected override void AddProperties(PropertyBuilder builder)
        {
            builder.Add("add", Add);
            builder.Add("create", Create);
            builder.Add("delete", Delete);
            builder.Add("list", List);
            builder.Add("read", Read);
            builder.Add("write", Write);
        }
    }

    public static class DataSource
    {
        public static Task<string> GetAccountBlobContainerSAS(GetAccountBlobContainerSASArgs args)
        {
            return Deployment.Instance.InvokeAsync<string>(
                "azure:storage/getAccountBlobContainerSAS:getAccountBlobContainerSAS",
                args,
                dict => dict["sas"]?.ToString());
        }
    }
}
