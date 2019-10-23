// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

using System.Collections.Generic;
using System.Threading.Tasks;
using Pulumi;

class Program
{
    static Task<int> Main(string[] args)
    {
        return Deployment.RunAsync(() => 
        {
            return new Dictionary<string, object>
            {
                {  "xyz", "ABC" },
                {  "foo", 42 },
            };
        });
    }
}