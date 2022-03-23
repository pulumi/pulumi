// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;

using FluentAssertions;
using NUnit.Framework;
using Moq;

using Pulumi;
using Pulumi.Serialization;
using Pulumi.Testing;

using Pulumi.Mypkg.Outputs;
using static Pulumi.Mypkg.TestHelpers;

namespace Pulumi.Mypkg
{
    [TestFixture]
    public class UnitTests
    {

        [Test]
        public async Task TestListStorageAccountsOutputWorks()
        {
            Func<StorageAccountKeyResponseResult, string> showSAKR = (r) =>
                $"CreationTime={r.CreationTime} KeyName={r.KeyName} Permissions={r.Permissions} Value={r.Value}";

            Func<string,Func<ListStorageAccountKeysInvokeArgs>,Task> check = (
                (expected, args) => Assert
                .Output(() => ListStorageAccountKeys.Invoke(args()).Apply(x => {
                    return "[" + string.Join(", ", x.Keys.Select(k => showSAKR(k))) + "]";
                })).ResolvesTo(expected)
            );

            await check("[CreationTime=my-creation-time KeyName=my-key-name Permissions=my-permissions" +
                        " Value=[accountName: my-an, expand: my-expand, resourceGroupName: my-rgn]]",
                        () => new ListStorageAccountKeysInvokeArgs()
                        {
                            AccountName = Out("my-an"),
                            ResourceGroupName = Out("my-rgn"),
                            Expand = Out("my-expand")
                        });
        }

        [Test]
        public async Task TestGetAmiIdsWorks()
        {
            Func<GetAmiIdsResult, string> showGetAmiIdsResult = (r) =>
                $"Id={r.Id} NameRegex={r.NameRegex} SortAscending={r.SortAscending}";

            Func<string,Func<GetAmiIdsInvokeArgs>,Task> check = (expected, args) =>
                Assert.Output(() => GetAmiIds.Invoke(args()).Apply(x => showGetAmiIdsResult(x)))
                .ResolvesTo(expected);

            var owners = new InputList<string>();
            owners.Add(Out("owner-1"));
            owners.Add(Out("owner-2"));

            Func<int, Inputs.GetAmiIdsFilterInputArgs> makeFilter = (int i) =>
            {
                var values = new InputList<string>();
                values.Add($"value-{i}-1");
                values.Add($"value-{i}-2");
                var f = new Inputs.GetAmiIdsFilterInputArgs() {
                    Name = Out($"filter-{i}-name"),
                    Values = values
                };
                return f;
            };

            var filters = new InputList<Inputs.GetAmiIdsFilterInputArgs>();
            filters.Add(makeFilter(1));
            filters.Add(makeFilter(2));

            var expectOwners = "[owners: owner-1, owner-2]";
            var expectFilters = "[name=filter-1-name values=[value-1-1, value-1-2], name=filter-2-name values=[value-2-1, value-2-2]]";
            var expect = $"Id=my-id {expectOwners} [filters: {expectFilters}] NameRegex=[a-z] SortAscending=True";

            await check(expect,
                        () => new GetAmiIdsInvokeArgs()
                        {
                            NameRegex = Out("[a-z]"),
                            SortAscending = Out(true),
                            Owners = owners,
                            Filters = filters
                        });
        }
    }
}
