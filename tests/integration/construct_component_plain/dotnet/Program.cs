using System.Threading.Tasks;
using Pulumi;

class Program
{
    static Task<int> Main() => Deployment.RunAsync<MyStack>();
}
