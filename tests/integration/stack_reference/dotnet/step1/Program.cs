// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using Pulumi;

class Program
{
    static Task<int> Main(string[] args)
    {
        return Deployment.RunAsync(async () =>
        {
            var config = new Config();
            var org = config.Require("org");
            var slug = $"{org}/{Deployment.Instance.ProjectName}/{Deployment.Instance.StackName}";
            var a = new StackReference(slug);

            var oldVal = (string[])await a.GetValueAsync("val");
            if (oldVal.Length != 2 || oldVal[0] != "a" || oldVal[1] != "b")
            {
                throw new Exception("Invalid result");
            }

            return new Dictionary<string, object>
            {
                { "val2", Output.CreateSecret(new[] { "a", "b" }) }
            };
        });
    }
}