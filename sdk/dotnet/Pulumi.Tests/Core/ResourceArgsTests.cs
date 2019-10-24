// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Immutable;
using System.Threading.Tasks;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Core
{
    public class ComplexResourceArgs1 : ResourceArgs
    {
        [Input("s")] public Input<string> S { get; set; } = null!;
        [Input("array")] private InputList<bool> _array = null!;
        public InputList<bool> Array
        {
            get => _array ?? (_array = new InputList<bool>());
            set => _array = value;
        }
    }

    public class ResourceArgsTests : PulumiTest
    {
        [Fact]
        public void TestComplexResourceArgs1_NullValues()
        {
            var args = new ComplexResourceArgs1();
            var dictionary = args.ToDictionary();

            Assert.True(dictionary.TryGetValue("s", out var sValue));
            Assert.True(dictionary.TryGetValue("array", out var arrayValue));

            Assert.Null(sValue);
            Assert.Null(arrayValue);
        }

        [Fact]
        public async Task TestComplexResourceArgs1_SetField()
        {
            var args = new ComplexResourceArgs1
            {
                S = "val",
            };

            var dictionary = args.ToDictionary();

            Assert.True(dictionary.TryGetValue("s", out var sValue));
            Assert.True(dictionary.TryGetValue("array", out var arrayValue));

            Assert.NotNull(sValue);
            Assert.Null(arrayValue);

            var output = sValue.ToOutput();
            var data = await output.GetDataAsync();
            Assert.Equal("val", data.Value);
        }

        [Fact]
        public Task TestComplexResourceArgs1_SetProperty()
        {
            return RunInNormal(async () =>
            {
                var args = new ComplexResourceArgs1
                {
                    Array = { true },
                };

                var dictionary = args.ToDictionary();

                Assert.True(dictionary.TryGetValue("s", out var sValue));
                Assert.True(dictionary.TryGetValue("array", out var arrayValue));

                Assert.Null(sValue);
                Assert.NotNull(arrayValue);

                var output = arrayValue.ToOutput();
                var data = await output.GetDataAsync();
                AssertEx.SequenceEqual(
                    ImmutableArray<bool>.Empty.Add(true), (ImmutableArray<bool>)data.Value!);
            });
        }
    }
}
