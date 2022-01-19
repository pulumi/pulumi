// Copyright 2021, Pulumi Corporation.  All rights reserved.

using System;
using System.Threading.Tasks;
using Pulumi;

class Program
{
    static Task<int> Main(string[] args)
    {
        return Deployment.RunAsync(() => {
            for (int i = 0; i < 10; i++) {
                Console.WriteLine("Line {0}", i);
                Console.Error.WriteLine("Errln {0}", i+10);
            }
            Console.Write("Line 10");
            Console.Error.Write("Errln 20");
        });
    }
}
