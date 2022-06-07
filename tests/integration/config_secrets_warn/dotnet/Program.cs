// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

using System;
using System.Collections.Generic;
using System.Linq;
using System.Text.Json;
using System.Threading.Tasks;
using Pulumi;

class Program
{
    static Task<int> Main(string[] args)
    {
        return Deployment.RunAsync(() =>
        {
            var config = new Config();

            config.Get("plainstr1");
            config.Require("plainstr2");
            config.GetSecret("plainstr3");
            config.RequireSecret("plainstr4");

            config.GetBoolean("plainbool1");
            config.RequireBoolean("plainbool2");
            config.GetSecretBoolean("plainbool3");
            config.RequireSecretBoolean("plainbool4");

            config.GetInt32("plainint1");
            config.RequireInt32("plainint2");
            config.GetSecretInt32("plainint3");
            config.RequireSecretInt32("plainint4");

            config.GetObject<JsonElement>("plainobj1");
            config.RequireObject<JsonElement>("plainobj2");
            config.GetSecretObject<JsonElement>("plainobj3");
            config.RequireSecretObject<JsonElement>("plainobj4");

            config.Get("str1");
            config.Require("str2");
            config.GetSecret("str3");
            config.RequireSecret("str4");

            config.GetBoolean("bool1");
            config.RequireBoolean("bool2");
            config.GetSecretBoolean("bool3");
            config.RequireSecretBoolean("bool4");

            config.GetInt32("int1");
            config.RequireInt32("int2");
            config.GetSecretInt32("int3");
            config.RequireSecretInt32("int4");

            config.GetObject<JsonElement>("obj1");
            config.RequireObject<JsonElement>("obj2");
            config.GetSecretObject<JsonElement>("obj3");
            config.RequireSecretObject<JsonElement>("obj4");

            config.GetObject<JsonElement>("parent1");
            config.RequireObject<JsonElement>("parent2");
            config.GetSecretObject<JsonElement>("parent1");
            config.RequireSecretObject<JsonElement>("parent2");

            config.GetObject<JsonElement>("names1");
            config.RequireObject<JsonElement>("names2");
            config.GetSecretObject<JsonElement>("names1");
            config.RequireSecretObject<JsonElement>("names2");
        });
    }
}
