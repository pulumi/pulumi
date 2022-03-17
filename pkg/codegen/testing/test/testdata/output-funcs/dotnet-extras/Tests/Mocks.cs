// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading.Tasks;

using Pulumi.Testing;

namespace Pulumi.Mypkg
{
    public class Mocks : IMocks
    {
        public Task<object> CallAsync(MockCallArgs args)
        {

            if (args.Token == "mypkg::funcWithAllOptionalInputs"
                || args.Token == "mypkg::funcWithDefaultValue")
            {
                var dictBuilder = ImmutableDictionary.CreateBuilder<string,Object>();
                var a = args.Args.GetValueOrDefault("a", "null");
                var b = args.Args.GetValueOrDefault("b", "null");
                dictBuilder.Add("r", $"a={a} b={b}");
                var result = dictBuilder.ToImmutableDictionary();
                return Task.FromResult((Object)result);
            }

            if (args.Token == "mypkg::funcWithDictParam")
            {
                string aString = "null";
                if (args.Args.ContainsKey("a"))
                {
                    var a = (ImmutableDictionary<string,object>)args.Args["a"];
                    aString = string.Join(", ", a.Keys.OrderBy(k => k).Select(k => $"{k}: {a[k]}"));
                    aString = $"[{aString}]";
                }
                var b = args.Args.GetValueOrDefault("b", "null");
                var dictBuilder = ImmutableDictionary.CreateBuilder<string,Object>();
                dictBuilder.Add("r", $"a={aString} b={b}");
                var result = dictBuilder.ToImmutableDictionary();
                return Task.FromResult((Object)result);
            }

            if (args.Token == "mypkg::funcWithListParam")
            {
                string aString = "null";
                if (args.Args.ContainsKey("a"))
                {
                    var a = (ImmutableArray<object>)args.Args["a"];
                    aString = string.Join(", ", a.OrderBy(k => k).Select(e => $"{e}"));
                    aString = $"[{aString}]";
                }
                var b = args.Args.GetValueOrDefault("b", "null");
                var dictBuilder = ImmutableDictionary.CreateBuilder<string,Object>();
                dictBuilder.Add("r", $"a={aString} b={b}");
                var result = dictBuilder.ToImmutableDictionary();
                return Task.FromResult((Object)result);
            }

            if (args.Token == "mypkg::getIntegrationRuntimeObjectMetadatum")
            {
                var argsString = string.Join(", ", args.Args.Keys.OrderBy(k => k).Select(k => $"{k}: {args.Args[k]}"));
                var dictBuilder = ImmutableDictionary.CreateBuilder<string,Object>();
                var arrayBuilder = ImmutableArray.CreateBuilder<object>();
                arrayBuilder.Add(argsString);
                dictBuilder.Add("value", arrayBuilder.ToImmutableArray());
                dictBuilder.Add("nextLink", "my-next-link");
                var result = dictBuilder.ToImmutableDictionary();
                return Task.FromResult((Object)result);
            }

            if (args.Token == "mypkg::listStorageAccountKeys")
            {
                var argsString = string.Join(", ", args.Args.Keys.OrderBy(k => k).Select(k => $"{k}: {args.Args[k]}"));
                var dictBuilder = ImmutableDictionary.CreateBuilder<string,Object>();
                var arrayBuilder = ImmutableArray.CreateBuilder<object>();
                var elemBuilder = ImmutableDictionary.CreateBuilder<string,Object>();
                elemBuilder.Add("creationTime", "my-creation-time");
                elemBuilder.Add("keyName", "my-key-name");
                elemBuilder.Add("permissions", "my-permissions");
                elemBuilder.Add("value", "[" + argsString + "]");
                arrayBuilder.Add(elemBuilder.ToImmutableDictionary());
                dictBuilder.Add("keys", arrayBuilder.ToImmutableArray());
                var result = dictBuilder.ToImmutableDictionary();
                return Task.FromResult((Object)result);
            }

            throw new Exception($"CallAsync not implemented for {args.Token}..");
        }

        public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args)
        {
            if (args.Type == "mypkg::mockdep")
            {
                var dictBuilder = ImmutableDictionary.CreateBuilder<string,Object>();
                dictBuilder.Add("mockDepOutput", "mock-dep-output-value");
                var result = (object)(dictBuilder.ToImmutableDictionary());
                return Task.FromResult<(string? id, object state)>((id: "mockDep#1", state: result));
            }
            throw new Exception($"NewResourceAsync not implemented for {args.Type}..");
        }
    }
}
