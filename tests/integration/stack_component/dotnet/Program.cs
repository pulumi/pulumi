// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

using System.Collections.Generic;
using System.Threading.Tasks;
using Pulumi;

class MyStack : Stack
{
    [Output("abc")]
    public Output<string> Abc { get; }

    [Output]
    public Output<int> Foo { get; }

    // This should NOT be exported as stack output due to the missing attribute
    public Output<string> Bar { get; }

    public MyStack()
    {
        this.Abc = Output.Create("ABC");
        this.Foo = Output.Create(42);
        this.Bar = Output.Create("this should not come to output");
    }
}

class Program
{
    static Task<int> Main(string[] args) => Deployment.RunAsync<MyStack>();
}