// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using Pulumi;

class MyStack : Stack
{
    [Output("abc")]
    public Output<string> Abc { get; private set; }

    [Output]
    public Output<int> Foo { get; private set; }

    // This should NOT be exported as stack output due to the missing attribute
    public Output<string> Bar { get; private set; }

    public MyStack(Dependancy dependancy)
    {
        this.Abc = Output.Create(dependancy.Abc);
        this.Foo = Output.Create(dependancy.Foo);
        this.Bar = Output.Create(dependancy.Bar);
    }
}

class Program
{
    static Task<int> Main(string[] args)
    {
        Deployment.RunAsync<MyStack>(new TestServiceProvider());
    }
}

class Dependency
{
    public string Abc { get; set; } = "ABC";
    public int Foo { get; set; } = 42;
    public string Bar { get; set; } = "this should not come to output";
}

class TestServiceProvider : IServiceProvider
{
    public object GetService(Type serviceType)
    {
        if (serviceType == typeof(MyStack))
        {
            return new MyStack(new Dependency()); 
        }

        return null;
    }
}
