using Pulumi;
using Pulumi.Providers;

public class TestProvider : Provider {
    readonly IHost host;

    public TestProvider(IHost host) {
        this.host = host;
    }
}