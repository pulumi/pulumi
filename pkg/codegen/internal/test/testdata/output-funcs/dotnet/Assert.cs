// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Threading.Tasks;

using FluentAssertions;

namespace Pulumi.MadeupPackage.Codegentest
{
    public static class Assert
    {
        public static OutputAssert<T> Output<T>(Func<Output<T>> builder)
        {
            return new OutputAssert<T>(builder);
        }

        // public static void OutputResolvesTo<T1,T2>(T1 expected, Func<Output<T1>> outputBuilder, Func<T1,T2> converter)
        // {
        //     throw new Exception("OutputResolvesTo not implemented");
        // }
    }

    public class OutputAssert<T>
    {
        public OutputAssert(Func<Output<T>> builder)
        {
            this.Builder = builder;
        }

        public Func<Output<T>> Builder { get; private set; }

        public async Task ResolvesTo(T expected)
        {
            var mocks = new Mocks();
            var actual = await TestHelpers.Run(mocks, this.Builder);
            actual.Should().Be(expected);
        }
    }
}
