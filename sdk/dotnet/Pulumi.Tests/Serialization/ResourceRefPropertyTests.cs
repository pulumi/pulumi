// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Linq;
using System.Threading.Tasks;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;
using Pulumi.Testing;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class ResourceRefPropertyTests : ConverterTests
    {
        public sealed class MyArgs : ResourceArgs
        {
        }

        [ResourceType("test:index:resource", null)]
        private class MyCustomResource : CustomResource
        {
            public MyCustomResource(string name, MyArgs? args, CustomResourceOptions? options = null) : base("test:index:resource", name, args ?? new MyArgs(), options)
            {
            }
        }

        [ResourceType("test:index:component", null)]
        private class MyComponentResource : ComponentResource
        {
            public MyComponentResource(string name, MyArgs? args, ComponentResourceOptions? options = null) : base("test:index:component", name, args ?? new MyArgs(), options)
            {
            }
        }

        private class MissingCustomResource : CustomResource
        {
            public MissingCustomResource(string name, MyArgs? args, CustomResourceOptions? options = null) : base("test:missing:resource", name, args ?? new MyArgs(), options)
            {
            }
        }

        private class MissingComponentResource : ComponentResource
        {
            public MissingComponentResource(string name, MyArgs? args, ComponentResourceOptions? options = null) : base("test:missing:component", name, args ?? new MyArgs(), options)
            {
            }
        }

        public class MyStack : Stack
        {
            public MyStack()
            {
                new MyCustomResource("test", null);
                new MyComponentResource("test", null);
            }
        }

        class MyMocks : IMocks
        {
            private readonly bool _isPreview;

            public MyMocks(bool isPreview)
            {
                this._isPreview = isPreview;
            }

            public Task<object> CallAsync(MockCallArgs args)
            {
                throw new Exception($"Unknown function {args.Token}");
            }

            public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args)
            {
                switch (args.Type)
                {
                    case "test:index:resource":
                    case "test:missing:resource":
                        return Task.FromResult<(string?, object)>((this._isPreview ? null : "id", new Dictionary<string, object>()));
                    case "test:index:component":
                    case "test:missing:component":
                        return Task.FromResult<(string?, object)>((null, new Dictionary<string, object>()));
                    default:
                        throw new Exception($"Unknown resource {args.Type}");
                }
            }
        }

        static Task<ImmutableArray<Resource>> RunAsync<T>(bool isPreview) where T : Stack, new()
        {
            return Deployment.TestAsync<T>(new MyMocks(isPreview), new TestOptions { IsPreview = isPreview });
        }

        static Value CreateCustomResourceReference(string urn, string id)
            => new Value
            {
                StructValue = new Struct
                {
                    Fields =
                    {
                        { Constants.SpecialSigKey, new Value { StringValue = Constants.SpecialResourceSig } },
                        { "urn", new Value { StringValue = urn } },
                        { "id", new Value { StringValue = id } },
                    }
                }
            };

        static Value CreateComponentResourceReference(string urn)
            => new Value
            {
                StructValue = new Struct
                {
                    Fields =
                    {
                        { Constants.SpecialSigKey, new Value { StringValue = Constants.SpecialResourceSig } },
                        { "urn", new Value { StringValue = urn } },
                    }
                }
            };


        [Theory]
        [InlineData(false)]
        [InlineData(true)]
        public async Task SerializeCustomResource(bool isPreview)
        {
            var resources = await RunAsync<MyStack>(isPreview);
            var res = resources.OfType<MyCustomResource>().FirstOrDefault();
            Assert.NotNull(res);

            var urn = (await res!.Urn.DataTask).Value;
            var id = (await res.Id.DataTask).Value;

            var v = await SerializeToValueAsync(res);
            Assert.Equal(CreateCustomResourceReference(urn, id), v);
        }

        [Theory]
        [InlineData(false)]
        [InlineData(true)]
        public async Task SerializeCustomResourceDownlevel(bool isPreview)
        {
            var resources = await RunAsync<MyStack>(isPreview);
            var res = resources.OfType<MyCustomResource>().FirstOrDefault();
            Assert.NotNull(res);

            var id = await SerializeToValueAsync(res!.Id);

            var v = await SerializeToValueAsync(res, false);
            Assert.Equal(id, v);
        }

        public class DeserializeCustomResourceStack : Stack
        {
            [Output("values")]
            public Output<ImmutableDictionary<string, string>> Values { get; private set; }

            public DeserializeCustomResourceStack()
            {
                var res = new MyCustomResource("test", null);

                var urn = res.Urn.DataTask.Result.Value;
                var id = res.Id.DataTask.Result.Value;

                var v = DeserializeValue<MyCustomResource>(CreateCustomResourceReference(urn, ""));

                this.Values = Output.Create(new Dictionary<string, string> {
                    { "expectedUrn", urn },
                    { "expectedId", id },
                    { "actualUrn", v.Urn.DataTask.Result.Value },
                    { "actualId", v.Id.DataTask.Result.Value },
                }.ToImmutableDictionary());
            }
        }

        [Theory]
        [InlineData(false)]
        [InlineData(true)]
        public async Task DeserializeCustomResource(bool isPreview)
        {
            var resources = await RunAsync<DeserializeCustomResourceStack>(isPreview);

            var stack = resources.OfType<DeserializeCustomResourceStack>().FirstOrDefault();
            Assert.NotNull(stack);

            var values = (await stack!.Values.DataTask).Value;
            Assert.Equal(values["expectedUrn"], values["actualUrn"]);
            Assert.Equal(values["expectedId"], values["actualId"]);
        }

        public class DeserializeMissingCustomResourceStack : Stack
        {
            [Output("values")]
            public Output<ImmutableDictionary<string, string>> Values { get; private set; }

            public DeserializeMissingCustomResourceStack()
            {
                var res = new MissingCustomResource("test", null);

                var urn = res.Urn.DataTask.Result.Value;

                var v = DeserializeValue<Resource>(CreateCustomResourceReference(urn, ""));

                this.Values = Output.Create(new Dictionary<string, string> {
                    { "expectedUrn", urn },
                    { "actualUrn", v.Urn.DataTask.Result.Value },
                }.ToImmutableDictionary());
            }
        }

        [Theory]
        [InlineData(false)]
        [InlineData(true)]
        public async Task DeserializeMissingCustomResource(bool isPreview)
        {
            var resources = await RunAsync<DeserializeMissingCustomResourceStack>(isPreview);

            var stack = resources.OfType<DeserializeMissingCustomResourceStack>().FirstOrDefault();
            Assert.NotNull(stack);

            var values = (await stack!.Values.DataTask).Value;
            Assert.Equal(values["expectedUrn"], values["actualUrn"]);
        }

        [Theory]
        [InlineData(false)]
        [InlineData(true)]
        public async Task SerializeComponentResource(bool isPreview)
        {
            var resources = await RunAsync<MyStack>(isPreview);
            var res = resources.OfType<MyComponentResource>().FirstOrDefault();
            Assert.NotNull(res);

            var urn = (await res!.Urn.DataTask).Value;

            var v = await SerializeToValueAsync(res);
            Assert.Equal(CreateComponentResourceReference(urn), v);
        }

        [Theory]
        [InlineData(false)]
        [InlineData(true)]
        public async Task SerializeComponentResourceDownlevel(bool isPreview)
        {
            var resources = await RunAsync<MyStack>(isPreview);
            var res = resources.OfType<MyComponentResource>().FirstOrDefault();
            Assert.NotNull(res);

            var urn = await SerializeToValueAsync(res!.Urn);

            var v = await SerializeToValueAsync(res, false);
            Assert.Equal(urn, v);
        }

        public class DeserializeComponentResourceStack : Stack
        {
            [Output("values")]
            public Output<ImmutableDictionary<string, string>> Values { get; private set; }

            public DeserializeComponentResourceStack()
            {
                var res = new MyComponentResource("test", null);

                var urn = res.Urn.DataTask.Result.Value;

                var v = DeserializeValue<MyComponentResource>(CreateComponentResourceReference(urn));

                this.Values = Output.Create(new Dictionary<string, string> {
                    { "expectedUrn", urn },
                    { "actualUrn", v.Urn.DataTask.Result.Value },
                }.ToImmutableDictionary());
            }
        }

        [Theory]
        [InlineData(false)]
        [InlineData(true)]
        public async Task DeserializeComponentResource(bool isPreview)
        {
            var resources = await RunAsync<DeserializeComponentResourceStack>(isPreview);

            var stack = resources.OfType<DeserializeComponentResourceStack>().FirstOrDefault();
            Assert.NotNull(stack);

            var values = (await stack!.Values.DataTask).Value;
            Assert.Equal(values["expectedUrn"], values["actualUrn"]);
        }

        public class DeserializeMissingComponentResourceStack : Stack
        {
            [Output("values")]
            public Output<ImmutableDictionary<string, string>> Values { get; private set; }

            public DeserializeMissingComponentResourceStack()
            {
                var res = new MissingComponentResource("test", null);

                var urn = res.Urn.DataTask.Result.Value;

                var v = DeserializeValue<Resource>(CreateComponentResourceReference(urn));

                this.Values = Output.Create(new Dictionary<string, string> {
                    { "expectedUrn", urn },
                    { "actualUrn", v.Urn.DataTask.Result.Value },
                }.ToImmutableDictionary());
            }
        }

        [Theory]
        [InlineData(false)]
        [InlineData(true)]
        public async Task DeserializeMissingComponentResource(bool isPreview)
        {
            var resources = await RunAsync<DeserializeMissingComponentResourceStack>(isPreview);

            var stack = resources.OfType<DeserializeMissingComponentResourceStack>().FirstOrDefault();
            Assert.NotNull(stack);

            var values = (await stack!.Values.DataTask).Value;
            Assert.Equal(values["expectedUrn"], values["actualUrn"]);
        }
    }
}
