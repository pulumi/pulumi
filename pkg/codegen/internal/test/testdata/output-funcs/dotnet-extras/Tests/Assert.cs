// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Linq;
using System.Threading.Tasks;

using FluentAssertions;

namespace Pulumi.Mypkg
{
    public static class Assert
    {
        public static OutputAssert<T> Output<T>(Func<Output<T>> builder)
        {
            return new OutputAssert<T>(builder);
        }
    }

    public class OutputAssert<T>
    {
        public OutputAssert(Func<Output<T>> builder)
        {
            this.Builder = builder;
        }

        public Func<Output<T>> Builder { get; private set; }

        public async Task DependsOn(string urn)
        {
            var mocks = new Mocks();
            var actual = await TestHelpers.Run(mocks, this.Builder);
            actual.Deps.Should().Contain(x => x.Contains(urn));
        }

        public async Task ResolvesTo(T expected)
        {
            var mocks = new Mocks();
            var actual = await TestHelpers.Run(mocks, this.Builder);
            actual.Result.Should().Be(expected);
        }
    }
}
