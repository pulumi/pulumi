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

            if (args.Token == "mypkg::getAmiIds")
            {
                // NOTE: only subset of possible fields are tested here in the smoke-test.
                var filters = (ImmutableArray<object>)args.Args["filters"];
                var filterString = string.Join(
                    ", ",
                    filters
                    .Select(f => (ImmutableDictionary<string,object>)f)
                    .Select(d =>
                    {
                        var name = (string)d["name"];
                        var values = string.Join(", ", ((ImmutableArray<object>)d["values"]).Select(x => (string)x));
                        return $"name={name} values=[{values}]";
                    }));
                filterString = $"[{filterString}]";
                var owners = (ImmutableArray<object>)args.Args["owners"];
                var ownersString = string.Join(", ", owners.Select(x => (string)x));
                var sortAscending = (bool)args.Args["sortAscending"];
                var nameRegex = (string)args.Args["nameRegex"];
                var dictBuilder = ImmutableDictionary.CreateBuilder<string,Object>();
                dictBuilder.Add("id", $"my-id [owners: {ownersString}] [filters: {filterString}]");
                dictBuilder.Add("nameRegex", nameRegex);
                dictBuilder.Add("sortAscending", sortAscending);
                var result = dictBuilder.ToImmutableDictionary();
                return Task.FromResult((Object)result);
            }

            throw new Exception($"CallAsync not implemented for {args.Token}..");
        }

        public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args)
        {
            throw new Exception("NewResourceAsync not implemented..");
        }
    }
}
