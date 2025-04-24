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
using Pulumi.Testing;

using Pulumi.Mypkg.Outputs;
using static Pulumi.Mypkg.TestHelpers;

namespace Pulumi.Mypkg
{
    [TestFixture]
    public class UnitTests
    {
        [Test]
        public async Task DependenciesPropagate()
        {
            await Assert.Output(() =>
            {
                var dep = new MockDep("res1");
                var args = new FuncWithAllOptionalInputsInvokeArgs
                {
                    A = dep.MockDepOutput,
                };
                return FuncWithAllOptionalInputs.Invoke(args).Apply(x => x.R);
            }).DependsOn("mockdep::res1");
        }

        [Test]
        public async Task FuncWithAllOptionalInputsOutputWorks()
        {
            Func<string,Func<FuncWithAllOptionalInputsInvokeArgs?>,Task> check = (
                (expected, args) => Assert
                .Output(() => FuncWithAllOptionalInputs.Invoke(args()).Apply(x => x.R))
                .ResolvesTo(expected)
            );

            await check("a=null b=null", () => null);

            await check("a=null b=null", () => new FuncWithAllOptionalInputsInvokeArgs());

            await check("a=my-a b=null", () => new FuncWithAllOptionalInputsInvokeArgs
            {
                A = Out("my-a"),
            });

            await check("a=null b=my-b", () => new FuncWithAllOptionalInputsInvokeArgs
            {
                B = Out("my-b"),
            });

            await check("a=my-a b=my-b", () => new FuncWithAllOptionalInputsInvokeArgs
            {
                A = Out("my-a"),
                B = Out("my-b"),
            });
        }

        [Test]
        public async Task FuncWithDefaultValueOutputWorks()
        {
            Func<string,Func<FuncWithDefaultValueInvokeArgs>,Task> check = (
                (expected, args) => Assert
                .Output(() => FuncWithDefaultValue.Invoke(args()).Apply(x => x.R))
                .ResolvesTo(expected)
            );

            // Since A is required, not passing it is an exception.
            Func<Task> act = () => check("", () => new FuncWithDefaultValueInvokeArgs());
            await act.Should().ThrowAsync<Exception>();

            // Check that default values from the schema work.
            await check("a=my-a b=b-default", () => new FuncWithDefaultValueInvokeArgs()
            {
                A = Out("my-a")
            });

            await check("a=my-a b=my-b", () => new FuncWithDefaultValueInvokeArgs()
            {
                A = Out("my-a"),
                B = Out("my-b")
            });
        }

        [Test]
        public async Task FuncWithDictParamOutputWorks()
        {
            Func<string,Func<FuncWithDictParamInvokeArgs>,Task> check = (
                (expected, args) => Assert
                .Output(() => FuncWithDictParam.Invoke(args()).Apply(x => x.R))
                .ResolvesTo(expected)
            );

            var map = new InputMap<string>();
            map.Add("K1", Out("my-k1"));
            map.Add("K2", Out("my-k2"));

            // Omitted value defaults to null.
            await check("a=null b=null", () => new FuncWithDictParamInvokeArgs());

            await check("a=[K1: my-k1, K2: my-k2] b=null", () => new FuncWithDictParamInvokeArgs()
            {
                A = map,
            });

            await check("a=[K1: my-k1, K2: my-k2] b=my-b", () => new FuncWithDictParamInvokeArgs()
            {
                A = map,
                B = Out("my-b"),
            });
        }

        [Test]
        public async Task FuncWithListParamOutputWorks()
        {
            Func<string,Func<FuncWithListParamInvokeArgs>,Task> check = (
                (expected, args) => Assert
                .Output(() => FuncWithListParam.Invoke(args()).Apply(x => x.R))
                .ResolvesTo(expected)
            );

            var lst = new InputList<string>();
            lst.Add("e1");
            lst.Add("e2");
            lst.Add("e3");

            // Similarly to dicts, omitted value defaults to null, not empty list.
            await check("a=null b=null", () => new FuncWithListParamInvokeArgs());

            await check("a=[e1, e2, e3] b=null", () => new FuncWithListParamInvokeArgs()
            {
                A = lst,
            });

            await check("a=[e1, e2, e3] b=my-b", () => new FuncWithListParamInvokeArgs()
            {
                A = lst,
                B = Out("my-b"),
            });
        }

        [Test]
        public async Task GetIntegrationRuntimeObjectMetadatumOuputWorks()
        {
            Func<string,Func<GetIntegrationRuntimeObjectMetadatumInvokeArgs>,Task> check = (
                (expected, args) => Assert
                .Output(() => GetIntegrationRuntimeObjectMetadatum.Invoke(args()).Apply(x => {
                    var nextLink = x.NextLink ?? "null";
                    var valueRepr = "null";
                    if (x.Value != null)
                    {
                        valueRepr = string.Join(", ", x.Value.Select(e => $"{e}"));
                    }
                    return $"nextLink={nextLink} value=[{valueRepr}]";
                 }))
                .ResolvesTo(expected)
            );

            await check("nextLink=my-next-link value=[factoryName: my-fn, integrationRuntimeName: my-irn, " +
                        "metadataPath: my-mp, resourceGroupName: my-rgn]",
                        () => new GetIntegrationRuntimeObjectMetadatumInvokeArgs()
                        {
                            FactoryName = Out("my-fn"),
                            IntegrationRuntimeName = Out("my-irn"),
                            MetadataPath = Out("my-mp"),
                            ResourceGroupName = Out("my-rgn")
                        });
        }

        [Test]
        public async Task TestListStorageAccountsOutputWorks()
        {
            Func<StorageAccountKeyResponse, string> showSAKR = (r) =>
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
    }
}
