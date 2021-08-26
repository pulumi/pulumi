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
using static Pulumi.MadeupPackage.Codegentest.TestHelpers;

namespace Pulumi.MadeupPackage.Codegentest
{
    [TestFixture]
    public class UnitTests
    {

        [Test]
        public async Task FuncWithAllOptionalInputsOutputWorks()
        {
            Func<string,Func<FuncWithAllOptionalInputsOutputArgs?>,Task> check = (
                (expected, args) => Assert
                .Output(() => FuncWithAllOptionalInputs.Invoke(args()).Apply(x => x.R))
                .ResolvesTo(expected)
            );

            await check("a=null b=null", () => null);

            await check("a=null b=null", () => new FuncWithAllOptionalInputsOutputArgs());

            await check("a=my-a b=null", () => new FuncWithAllOptionalInputsOutputArgs
            {
                A = Out("my-a"),
            });

            await check("a=null b=my-b", () => new FuncWithAllOptionalInputsOutputArgs
            {
                B = Out("my-b"),
            });

            await check("a=my-a b=my-b", () => new FuncWithAllOptionalInputsOutputArgs
            {
                A = Out("my-a"),
                B = Out("my-b"),
            });
        }

        [Test]
        public async Task FuncWithDefaultValueOutputWorks()
        {
            Func<string,Func<FuncWithDefaultValueOutputArgs>,Task> check = (
                (expected, args) => Assert
                .Output(() => FuncWithDefaultValue.Invoke(args()).Apply(x => x.R))
                .ResolvesTo(expected)
            );

            // Since A is required, not passing it is an exception.
            // Perhaps this should be rejected by the typechecker
            // instead? Why is A optional statically?
            Func<Task> act = () => check("", () => new FuncWithDefaultValueOutputArgs());
            await act.Should().ThrowAsync<Exception>();

            // Check that default values from the schema work.
            await check("a=my-a b=b-default", () => new FuncWithDefaultValueOutputArgs()
            {
                A = Out("my-a")
            });

            await check("a=my-a b=my-b", () => new FuncWithDefaultValueOutputArgs()
            {
                A = Out("my-a"),
                B = Out("my-b")
            });
        }


// @pulumi.runtime.test
// def test_func_with_default_value(my_mocks):
//     # TODO[pulumi/pulumi/7815]: Defaults from schema not recognized.
//     return assert_function_matches_table(
//         funcWithDefaultValue.func_with_default_value_output,
//         [
//             ({}, 'a=None b=None', r),
//             ({'a': out('my-a')}, 'a=my-a b=None', r),
//             ({'a': out('my-a'), 'b': out('my-b')}, 'a=my-a b=my-b', r),
//         ])


        // public class FuncWithAllOptionalInputsOutputStack : Stack
        // {
        //     public FuncWithAllOptionalInputsOutputStack
        //     {

        //     }
        // }






        // [Test]
        // public async Task ListStorageAccountKeysApplyWorks()
        // {
        //     var r = await Run<ListStorageAccountKeysMocks,
        //         ListStorageAccountKeysTestStack,
        //         ListStorageAccountKeysResult>();

        //     r.Keys.Length.Should().Be(1);

        //     r.Keys[0].CreationTime.Should().Be("my-creation-time");
        //     r.Keys[0].KeyName.Should().Be("my-key-name");
        //     r.Keys[0].Permissions.Should().Be("my-resource-group-name");
        //     r.Keys[0].Value.Should().Be("my-value");
        // }

        /*
        [Test]
        public async Task ListStorageAccountKeysApplyWorksWithSkippedOptionalParams()
        {
            var r = await Run<ListStorageAccountKeysMocks,
                ListStorageAccountKeysTestStack2,
                ListStorageAccountKeysResult>();

            r.Keys.Length.Should().Be(1);
            r.Keys[0]["accountName"].Should().Be("my-account-name");
            r.Keys[0]["resourceGroupName"].Should().Be("my-resource-group-name");
            r.Keys[0].ContainsKey("expand").Should().Be(false);
        }
        */

        public class ListStorageAccountKeysTestStack : Stack, HasResult<ListStorageAccountKeysResult>
        {
            public ListStorageAccountKeysTestStack()
            {
                var args = new ListStorageAccountKeysOutputArgs
                {
                    AccountName = "my-account-name",
                    ResourceGroupName = "my-resource-group-name",
                    Expand = "my-expand",
                };
                this.Result = ListStorageAccountKeys.Invoke(args);
            }

            public Output<ListStorageAccountKeysResult> Result { get; }
        }

        /*
        public class ListStorageAccountKeysTestStack2 : Stack, HasResult<ListStorageAccountKeysResult>
        {
            public ListStorageAccountKeysTestStack2()
            {
                var args = new ListStorageAccountKeysOutputArgs
                {
                    AccountName = "my-account-name",
                    ResourceGroupName = "my-resource-group-name"
                };
                this.Result = ListStorageAccountKeys.Invoke(args);
            }

            public Output<ListStorageAccountKeysResult> Result { get; }
        }
        */
        public class ListStorageAccountKeysMocks : IMocks
        {
            public Task<object> CallAsync(MockCallArgs args)
            {
                // Parse serialized `ListStorageAccountKeysArgs` from `args` into `keys`.
                var keysBuilder = ImmutableDictionary.CreateBuilder<string,string>();
                foreach (var pair in args.Args)
                {
                    keysBuilder.Add(pair.Key, (string)pair.Value);
                }
                var keys = ImmutableArray.Create(keysBuilder.ToImmutableDictionary());

                // Create serialized `ListStorageAccountKeysResult`.
                var dictBuilder = ImmutableDictionary.CreateBuilder<string,Object>();
                dictBuilder.Add("keys", keys);
                var result = dictBuilder.ToImmutableDictionary();

                return Task.FromResult((Object)result);
            }

            public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args)
            {
                throw new Exception("NewResourceAsync not impl..");
            }
        }
        /*

        [Test]
        public async Task FuncWithDefaultValueApplyWorks()
        {
            var r = await Run<FuncWithDefaultValueMocks,
                FuncWithDefaultValueTestStack,
                FuncWithDefaultValueResult>();
            var d = JsonSerializer.Deserialize<ImmutableDictionary<string,string>>(r.R);
            d["a"].Should().Be("my-a");
            d["b"].Should().Be("b-default");
        }

        public class FuncWithDefaultValueTestStack : Stack, HasResult<FuncWithDefaultValueResult>
        {
            public FuncWithDefaultValueTestStack()
            {
                var args = new FuncWithDefaultValueOutputArgs
                {
                    A = "my-a",
                };
                this.Result = FuncWithDefaultValue.Invoke(args);
            }

            public Output<FuncWithDefaultValueResult> Result { get; }
        }

        public class FuncWithDefaultValueMocks : IMocks
        {
            public Task<object> CallAsync(MockCallArgs args)
            {
                // Create serialized `FuncWithDefaultValueResult`.
                var dictBuilder = ImmutableDictionary.CreateBuilder<string,Object>();
                var argsRepr = ShowMockCallArgs(args);
                dictBuilder.Add("r", (Object)argsRepr);
                var result = dictBuilder.ToImmutableDictionary();
                return Task.FromResult((Object)result);
            }

            public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args)
            {
                throw new Exception("NewResourceAsync not impl..");
            }
        }

        [Test]
        public async Task FuncWithAllOptionalInputsApplyWorks()
        {
            var r = await Run<FuncWithAllOptionalInputsMocks,
                FuncWithAllOptionalInputsTestStack,
                FuncWithAllOptionalInputsResult>();
            r.R.Should().Be("{\"a\":\"my-a\"}");
        }

        public class FuncWithAllOptionalInputsTestStack : Stack, HasResult<FuncWithAllOptionalInputsResult>
        {
            public FuncWithAllOptionalInputsTestStack()
            {
                var args = new FuncWithAllOptionalInputsOutputArgs
                {
                    A = "my-a",
                };
                this.Result = FuncWithAllOptionalInputs.Invoke(args);
            }

            public Output<FuncWithAllOptionalInputsResult> Result { get; }
        }

        public class FuncWithAllOptionalInputsMocks : IMocks
        {
            public Task<object> CallAsync(MockCallArgs args)
            {
                // Create serialized `FuncWithAllOptionalInputsResult`.
                var dictBuilder = ImmutableDictionary.CreateBuilder<string,Object>();
                var argsRepr = ShowMockCallArgs(args);
                dictBuilder.Add("r", (Object)argsRepr);
                var result = dictBuilder.ToImmutableDictionary();
                return Task.FromResult((Object)result);
            }

            public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args)
            {
                throw new Exception("NewResourceAsync not impl..");
            }
        }

        [Test]
        public async Task FuncWithListParamApplyWorks()
        {
            var r = await Run<FuncWithListParamMocks,
                FuncWithListParamTestStack,
                FuncWithListParamResult>();
            r.R.Should().Be("{\"a\":[\"my-a1\",\"my-a2\",\"my-a3\"]}");
        }

        public class FuncWithListParamTestStack : Stack, HasResult<FuncWithListParamResult>
        {
            public FuncWithListParamTestStack()
            {
                var args = new FuncWithListParamOutputArgs
                {
                    A = {"my-a1", "my-a2", "my-a3"}
                };
                this.Result = FuncWithListParam.Invoke(args);
            }

            public Output<FuncWithListParamResult> Result { get; }
        }

        public class FuncWithListParamMocks : IMocks
        {
            public Task<object> CallAsync(MockCallArgs args)
            {
                // Create serialized `FuncWithListParamResult`.
                var dictBuilder = ImmutableDictionary.CreateBuilder<string,Object>();
                var argsRepr = ShowMockCallArgs(args);
                dictBuilder.Add("r", (Object)argsRepr);
                var result = dictBuilder.ToImmutableDictionary();
                return Task.FromResult((Object)result);
            }

            public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args)
            {
                throw new Exception("NewResourceAsync not impl..");
            }
        }

        [Test]
        public async Task FuncWithDictParamApplyWorks()
        {
            var r = await Run<FuncWithDictParamMocks,
                FuncWithDictParamTestStack,
                FuncWithDictParamResult>();
            var d = JsonSerializer.Deserialize<ImmutableDictionary<string,ImmutableDictionary<string,string>>>(r.R);
            d["a"]["one"].Should().Be("1");
            d["a"]["two"].Should().Be("2");
        }

        public class FuncWithDictParamTestStack : Stack, HasResult<FuncWithDictParamResult>
        {
            public FuncWithDictParamTestStack()
            {
                var args = new FuncWithDictParamOutputArgs
                {
                    A = new Dictionary<string, string>
                    {
                        { "one", "1" },
                        { "two", "2" }
                    }
                };
                this.Result = FuncWithDictParam.Invoke(args);
            }

            public Output<FuncWithDictParamResult> Result { get; }
        }

        public class FuncWithDictParamMocks : IMocks
        {
            public Task<object> CallAsync(MockCallArgs args)
            {
                // Create serialized `FuncWithDictParamResult`.
                var dictBuilder = ImmutableDictionary.CreateBuilder<string,Object>();
                var argsRepr = ShowMockCallArgs(args);
                dictBuilder.Add("r", (Object)argsRepr);
                var result = dictBuilder.ToImmutableDictionary();
                return Task.FromResult((Object)result);
            }

            public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args)
            {
                throw new Exception("NewResourceAsync not impl..");
            }
        }
        */

        // Supporting code

        public static string ShowMockCallArgs(MockCallArgs args)
        {
            return JsonSerializer.Serialize(args.Args);
        }

        interface HasResult<R>
        {
            Output<R> Result { get; }
        }

        static async Task<R> Run<M,S,R>() where S : Pulumi.Stack, HasResult<R>, new() where M : IMocks, new()
        {
            IMocks mocks = new M();
            var resources = await Deployment.TestAsync<S>(mocks, new TestOptions
            {
                IsPreview = false
            });
            var stack = (S)resources[0];
            HasResult<R> s = stack;
            var result = await AwaitOutput(s.Result);
            return result;
        }

        static Task<T> AwaitOutput<T>(Output<T> output)
        {
            var tcs = new TaskCompletionSource<T>();
            Task.Delay(1000).ContinueWith(_ =>
            {
                tcs.TrySetException(new Exception("Output resolution has timed out"));
            });
            output.Apply(v =>
            {
                tcs.TrySetResult(v);
                return v;
            });
            return tcs.Task;
        }
    }
}
