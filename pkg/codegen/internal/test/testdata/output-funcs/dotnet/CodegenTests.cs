using System;
using System.Collections.Immutable;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

using FluentAssertions;
using NUnit.Framework;
using Moq;

using Pulumi;
using Pulumi.Testing;

namespace Pulumi.MadeupPackage.Codegentest
{
    [TestFixture]
    public class UnitTests
    {
        [Test]
        public async Task ListStorageAccountKeysApplyWorks()
        {
            var r = await Run<ListStorageAccountKeysMocks,
                ListStorageAccountKeysTestStack,
                ListStorageAccountKeysResult>();

            r.Keys.Length.Should().Be(1);
            r.Keys[0]["accountName"].Should().Be("my-account-name");
            r.Keys[0]["resourceGroupName"].Should().Be("my-resource-group-name");
            r.Keys[0]["expand"].Should().Be("my-expand");
        }

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

        public class ListStorageAccountKeysTestStack : Stack, HasResult<ListStorageAccountKeysResult>
        {
            public ListStorageAccountKeysTestStack()
            {
                var args = new ListStorageAccountKeysApplyArgs
                {
                    AccountName = "my-account-name",
                    ResourceGroupName = "my-resource-group-name",
                    Expand = "my-expand",
                };
                this.Result = ListStorageAccountKeys.Apply(args);
            }

            public Output<ListStorageAccountKeysResult> Result { get; }
        }

        public class ListStorageAccountKeysTestStack2 : Stack, HasResult<ListStorageAccountKeysResult>
        {
            public ListStorageAccountKeysTestStack2()
            {
                var args = new ListStorageAccountKeysApplyArgs
                {
                    AccountName = "my-account-name",
                    ResourceGroupName = "my-resource-group-name"
                };
                this.Result = ListStorageAccountKeys.Apply(args);
            }

            public Output<ListStorageAccountKeysResult> Result { get; }
        }

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

        [Test]
        public async Task FuncWithDefaultValueApplyWorks()
        {
            var r = await Run<FuncWithDefaultValueMocks,
                FuncWithDefaultValueTestStack,
                FuncWithDefaultValueResult>();
            r.R.Should().Be("a=my-a;b=b-default;");
        }

        public class FuncWithDefaultValueTestStack : Stack, HasResult<FuncWithDefaultValueResult>
        {
            public FuncWithDefaultValueTestStack()
            {
                var args = new FuncWithDefaultValueApplyArgs
                {
                    A = "my-a",
                };
                this.Result = FuncWithDefaultValue.Apply(args);
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

        // Supporting code

        public static string ShowMockCallArgs(MockCallArgs args)
        {
            var sb = new StringBuilder();
            foreach (var pair in args.Args.OrderBy(p => p.Key))
            {
                sb.AppendFormat("{0}={1};", pair.Key, pair.Value);
            }
            return sb.ToString();
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
