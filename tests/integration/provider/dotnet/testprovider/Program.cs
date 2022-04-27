using Pulumi.Provider;


public static class Program {
    public static void Main(string[] args) {
        Provider.Serve(args, host => new TestProvider(host));
    }
}