// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using System.Threading.Tasks;
using Pulumi;

class Program
{
    static Task<int> Main() => Deployment.RunAsync<MyStack>();
}
