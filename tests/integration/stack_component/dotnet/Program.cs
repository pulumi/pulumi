// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

using System.Collections.Generic;
using System.Threading.Tasks;
using Pulumi;
using Pulumi.Serialization;

class MyStack : Stack
{
    [Output("abc")]
    public Output<string> Abc { get; private set; }

    [Output("xyz")]
    public Output<string> Xyz { get; private set; }

    [Output("foo")]
    public Output<int> Foo { get; private set; }

    // This should NOT be exported as stack output due to the missing attribute
    public Output<string> Bar { get; private set; }

    public MyStack()
    {
        this.Abc = Output.Create("ABC");
    }

    protected override void Initialize()
    {
        this.Xyz = Output.Create("XYZ");
    }

    protected override async Task InitializeAsync()
    {
        await Task.Delay(10); // mocks some async work
        this.Foo = Output.Create(42);
        this.Bar = Output.Create("this should not come to output");
    }
}

class Program
{
    static Task<int> Main(string[] args) => Deployment.RunAsync<MyStack>();
}