using System.Threading.Tasks;
using Pulumi.Serialization;

namespace Pulumi.Azure.Storage
{
    public class GetAccountBlobContainerSASArgs : ResourceArgs
    {
        [Input("connectionString")]
        public Input<string> ConnectionString { get; set; }
        [Input("containerName")]
        public Input<string> ContainerName { get; set; }
        [Input("start")]
        public Input<string> Start { get; set; }
        [Input("expiry")]
        public Input<string> Expiry { get; set; }
        [Input("permissions")]
        public Input<GetAccountBlobContainerSASPermissions> Permissions { get; set; }
    }

    public class GetAccountBlobContainerSASPermissions : ResourceArgs
    {
        [Input("add")]
        public Input<bool> Add { get; set; }
        [Input("create")]
        public Input<bool> Create { get; set; }
        [Input("delete")]
        public Input<bool> Delete { get; set; }
        [Input("list")]
        public Input<bool> List { get; set; }
        [Input("read")]
        public Input<bool> Read { get; set; }
        [Input("write")]
        public Input<bool> Write { get; set; }
    }

    [OutputType]
    public class GetAccountBlobContainerSASResult
    {
        public readonly string Sas;

        [OutputConstructor]
        private GetAccountBlobContainerSASResult(string sas)
        {
            this.Sas = sas;
        }
    }

    public static class DataSource
    {
        public static async Task<string> GetAccountBlobContainerSAS(GetAccountBlobContainerSASArgs args)
        {
            var result = await Deployment.Instance.InvokeAsync<GetAccountBlobContainerSASResult>(
                "azure:storage/getAccountBlobContainerSAS:getAccountBlobContainerSAS", args).ConfigureAwait(false);
            return result.Sas.ToString();
        }
    }
}
