import pulumi
import pulumi_secret as secret

res = secret.Resource("res",
    private="closed",
    public="open",
    private_data=secret.DataArgs(
        private="closed",
        public="open",
    ),
    public_data=secret.DataArgs(
        private="closed",
        public="open",
    ),
    private_array=["closed"],
    private_map={
        "key": "closed",
    },
    private_data_array=[secret.DataArgs(
        private="closed",
        public="open",
    )],
    private_data_map={
        "key": secret.DataArgs(
            private="closed",
            public="open",
        ),
    })
