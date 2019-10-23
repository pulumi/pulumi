// Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

using System;
using System.Threading.Tasks;
using Pulumi;

class Program
{
    static Task<int> Main(string[] args)
    {
        return Deployment.RunAsync(() => 
        {
            var config = new Config("config_basic_dotnet");

            // This value is plaintext and doesn't require encryption.
            var value = config.Require("aConfigValue");
            if (value != "this value is a value")
            {
                throw new Exception($"aConfigValue not the expected value; got {value}");
            }

            // This value is a secret
            var secret = config.Require("bEncryptedSecret");
            if (secret != "this super secret is encrypted")
            {
                throw new Exception($"bEncryptedSecret not the expected value; got {secret}");
            }
        });
    }
}