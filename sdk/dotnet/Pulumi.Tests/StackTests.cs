// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Moq;
using Pulumi.Serialization;
using Pulumi.Testing;
using Xunit;

namespace Pulumi.Tests
{
    public class StackTests
    {
        private class ValidStack : Stack
        {
            [Output("foo")]
            public Output<string> ExplicitName { get; }

            [Output]
            public Output<string> ImplicitName { get; }

            public ValidStack()
            {
                this.ExplicitName = Output.Create("bar");
                this.ImplicitName = Output.Create("buzz");
            }
        }

        [Fact]
        public async Task ValidStackInstantiationSucceeds()
        {
            // Arrange
            Output<IDictionary<string, object?>>? output = null;

            var mock = new Mock<ITestContext>();
            mock.Setup(d => d.RegisterResourceOutputs(It.IsAny<Stack>(), It.IsAny<Output<IDictionary<string, object?>>>()))
                .Callback((Resource _, Output<IDictionary<string, object?>> o) => output = o);

            // Act
            var result = await Deployment.TestAsync<ValidStack>(mock.Object);

            // Assert
            Assert.False(result.HasErrors, "Running a valid stack should yield no errors");
            var stack = result.Resources.OfType<ValidStack>().FirstOrDefault();
            Assert.NotNull(stack);

            var outputData = await output!.DataTask;
            var outputs = outputData.Value;
            Assert.Equal(2, outputs.Count);
            Assert.Same(stack.ExplicitName, outputs["foo"]);
            Assert.Same(stack.ImplicitName, outputs["ImplicitName"]);
        }

        private class NullOutputStack : Stack
        {
            [Output("foo")]
            public Output<string>? Foo { get; }
        }

        [Fact]
        public async Task StackWithNullOutputsThrows()
        {
            var result = await Deployment.TestAsync<NullOutputStack>();
            Assert.True(result.HasErrors);
            Assert.Single(result.LoggedErrors);
            Assert.Contains("foo", result.LoggedErrors.First());
        }

        private class InvalidOutputTypeStack : Stack
        {
            [Output("foo")]
            public string Foo { get; }

            public InvalidOutputTypeStack()
            {
                this.Foo = "bar";
            }
        }

        [Fact]
        public async Task StackWithInvalidOutputTypeThrows()
        {
            var result = await Deployment.TestAsync<InvalidOutputTypeStack>();
            Assert.True(result.HasErrors);
            Assert.Single(result.LoggedErrors);
            Assert.Contains("foo", result.LoggedErrors.First());
        }
    }
}
