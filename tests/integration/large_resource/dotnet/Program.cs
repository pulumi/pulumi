// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

using System.Collections.Generic;
using System.Threading.Tasks;
using System;
using Pulumi;

class Program
{
    static Task<int> Main(string[] args)
    {
        return Deployment.RunAsync(() =>
        {
            // Create and export a very long string (>4mb)
            return new Dictionary<string, object>
            {
                {  "LongString", new string('a', 5 * 1024 * 1024) }
            };
        });
    }
}
