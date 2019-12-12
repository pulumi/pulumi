// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Immutable;
using Google.Protobuf.WellKnownTypes;
using Pulumi.Serialization;
using Xunit;

namespace Pulumi.Tests.Serialization
{
    public class RecursiveTypeTests : ConverterTests
    {
        [OutputType]
        public class RecursiveType
        {
            public readonly string Ref;
            public readonly ImmutableArray<RecursiveType> AdditionalItems;

            [OutputConstructor]
            public RecursiveType(string @ref, ImmutableArray<RecursiveType> additionalItems)
            {
                Ref = @ref;
                AdditionalItems = additionalItems;
            }
        }

        [Fact]
        public void SimpleCase()
        {
            var data = Converter.ConvertValue<RecursiveType>("", new Value
            {
                StructValue = new Struct
                {
                    Fields =
                    {
                        { "ref", new Value { StringValue = "a" } },
                        { "additionalItems", new Value
                            {
                                ListValue = new ListValue
                                {
                                    Values =
                                    {
                                        new Value
                                        {
                                            StructValue = new Struct
                                            {
                                                Fields =
                                                {
                                                    { "ref", new Value { StringValue = "b" } },
                                                    { "additionalItems", new Value { ListValue = new ListValue() } },
                                                }
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            });
            Assert.True(data.IsKnown);
            Assert.Equal("a", data.Value.Ref);
            Assert.Single(data.Value.AdditionalItems);

            Assert.Equal("b", data.Value.AdditionalItems[0].Ref);
            Assert.Empty(data.Value.AdditionalItems[0].AdditionalItems);
        }
    }
}
