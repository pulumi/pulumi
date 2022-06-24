// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Immutable;
using System.Threading.Tasks;
using Xunit;

namespace Pulumi.Tests.Core
{
    public class ResourceArgsTests : PulumiTest
    {
        #region ComplexResourceArgs1

        public class ComplexResourceArgs1 : ResourceArgs
        {
            [Input("s")] public Input<string> S { get; set; } = null!;
            [Input("array")] private InputList<bool> _array = null!;
            public InputList<bool> Array
            {
                // ReSharper disable once ConstantNullCoalescingCondition
                get => _array ??= new InputList<bool>();
                set => _array = value;
            }
        }

        [Fact]
        public async Task TestComplexResourceArgs1_NullValues()
        {
            var args = new ComplexResourceArgs1();
            var dictionary = await args.ToDictionaryAsync();

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

            var dictionary = await args.ToDictionaryAsync().ConfigureAwait(false);

            Assert.True(dictionary.TryGetValue("s", out var sValue));
            Assert.True(dictionary.TryGetValue("array", out var arrayValue));

            Assert.NotNull(sValue);
            Assert.Null(arrayValue);

            var output = ((IInput)sValue!).ToOutput();
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

                var dictionary = await args.ToDictionaryAsync().ConfigureAwait(false);

                Assert.True(dictionary.TryGetValue("s", out var sValue));
                Assert.True(dictionary.TryGetValue("array", out var arrayValue));

                Assert.Null(sValue);
                Assert.NotNull(arrayValue);

                var output = ((IInput)arrayValue!).ToOutput();
                var data = await output.GetDataAsync();
                AssertEx.SequenceEqual(
                    ImmutableArray<bool>.Empty.Add(true), (ImmutableArray<bool>)data.Value!);
            });
        }

        #endregion

        #region JsonResourceArgs1

        public class JsonResourceArgs1 : ResourceArgs
        {
            [Input("array", json: true)] private InputList<bool> _array = null!;
            public InputList<bool> Array
            {
                // ReSharper disable once ConstantNullCoalescingCondition
                get => _array ??= new InputList<bool>();
                set => _array = value;
            }

            [Input("map", json: true)] private InputMap<int> _map = null!;
            public InputMap<int> Map
            {
                // ReSharper disable once ConstantNullCoalescingCondition
                get => _map ??= new InputMap<int>();
                set => _map = value;
            }
        }

        [Fact]
        public async Task TestJsonMap()
        {
            var args = new JsonResourceArgs1
            {
                Array = { true, false },
                Map =
                {
                    { "k1", 1 },
                    { "k2", 2 },
                },
            };
            var dictionary = await args.ToDictionaryAsync();

            Assert.True(dictionary.TryGetValue("array", out var arrayValue));
            Assert.True(dictionary.TryGetValue("map", out var mapValue));

            Assert.NotNull(arrayValue);
            Assert.NotNull(mapValue);

            Assert.Equal("[ true, false ]", arrayValue);
            Assert.Equal("{ \"k1\": 1, \"k2\": 2 }", mapValue);
        }

        #endregion
    }
}
