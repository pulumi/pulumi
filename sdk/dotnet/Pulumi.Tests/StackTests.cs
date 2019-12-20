// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using Moq;
using Pulumi.Serialization;
using Xunit;
using Xunit.Sdk;

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
            var (stack, outputs) = await Run<ValidStack>();
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
            try
            {
                var (stack, outputs) = await Run<NullOutputStack>();
            }
            catch (RunException ex)
            {
                Assert.Contains("foo", ex.Message);
                return;
            }

            throw new XunitException("Should not come here");
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
            try
            {
                var (stack, outputs) = await Run<InvalidOutputTypeStack>();
            }
            catch (RunException ex)
            {
                Assert.Contains("foo", ex.Message);
                return;
            }

            throw new XunitException("Should not come here");
        }

        private async Task<(T, IDictionary<string, object?>)> Run<T>() where T : Stack, new()
        {
            // Arrange
            Output<IDictionary<string, object?>>? outputs = null;

            var mock = new Mock<IDeploymentInternal>(MockBehavior.Strict);
            mock.Setup(d => d.ProjectName).Returns("TestProject");
            mock.Setup(d => d.StackName).Returns("TestStack");
            mock.SetupSet(content => content.Stack = It.IsAny<Stack>());
            mock.Setup(d => d.ReadOrRegisterResource(It.IsAny<Stack>(), It.IsAny<ResourceArgs>(), It.IsAny<ResourceOptions>()));
            mock.Setup(d => d.RegisterResourceOutputs(It.IsAny<Stack>(), It.IsAny<Output<IDictionary<string, object?>>>()))
                .Callback((Resource _, Output<IDictionary<string, object?>> o) => outputs = o);

            Deployment.Instance = mock.Object;

            // Act
            var stack = new T();
            stack.RegisterPropertyOutputs();

            // Assert
            Assert.NotNull(outputs);
            var values = await outputs!.DataTask;
            return (stack, values.Value);
        }
    }
}
