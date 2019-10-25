// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System.Threading.Tasks;
using Pulumi;

class Program
{
    static Task<int> Main()
    {
        return Deployment.RunAsync(Pulumi.CSharpExamples.GlobalApp.Run);
    }
}
