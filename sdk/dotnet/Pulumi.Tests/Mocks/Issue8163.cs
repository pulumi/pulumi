// Copyright 2016-2021, Pulumi Corporation

using System.Threading.Tasks;

namespace Pulumi.Tests.Mocks
{
    public sealed class GetRoleArgs : Pulumi.InvokeArgs
    {
        /// <summary>
        /// The friendly IAM role name to match.
        /// </summary>
        [Input("name", required: true)]
        public string Name { get; set; } = null!;

        public GetRoleArgs()
        {
        }
    }

    public sealed class GetRoleInvokeArgs : Pulumi.InvokeArgs
    {
        /// <summary>
        /// The friendly IAM role name to match.
        /// </summary>
        [Input("name", required: true)]
        public Input<string> Name { get; set; } = null!;

        public GetRoleInvokeArgs()
        {
        }
    }

    [OutputType]
    public sealed class GetRoleResult
    {
        /// <summary>
        /// The Amazon Resource Name (ARN) specifying the role.
        /// </summary>
        public readonly string Arn;
        public readonly string Id;

        [OutputConstructor]
        private GetRoleResult(
            string arn,

            string id)
        {
            Arn = arn;
            Id = id;
        }
    }

    public static class GetRole
    {
        public static Task<GetRoleResult> InvokeAsync(GetRoleArgs args, InvokeOptions? options = null)
            => Pulumi.Deployment.Instance.InvokeAsync<GetRoleResult>("aws:iam/getRole:getRole", args ?? new GetRoleArgs(), options);

        public static Output<GetRoleResult> Invoke(GetRoleInvokeArgs args, InvokeOptions? options = null)
            => Pulumi.Deployment.Instance.Invoke<GetRoleResult>("aws:iam/getRole:getRole", args ?? new GetRoleInvokeArgs(), options);
    }
}
