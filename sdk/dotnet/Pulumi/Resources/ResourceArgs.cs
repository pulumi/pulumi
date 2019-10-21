// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System.Collections.Generic;
using System.Collections.Immutable;

namespace Pulumi
{
    /// <summary>
    /// Base type for all resource argument classes.
    /// </summary>
    public abstract class ResourceArgs
    {
        public static readonly ResourceArgs Empty = new EmptyResourceArgs();

        protected ResourceArgs()
        {
        }

        internal IDictionary<string, IInput> ToDictionary()
        {
            var dictionaryBuilder = ImmutableDictionary.CreateBuilder<string, IInput>();
            AddProperties(new PropertyBuilder(dictionaryBuilder));
            return dictionaryBuilder.ToImmutable();
        }

        protected abstract void AddProperties(PropertyBuilder builder);

        protected struct PropertyBuilder
        {
            private readonly ImmutableDictionary<string, IInput>.Builder _builder;

            internal PropertyBuilder(ImmutableDictionary<string, IInput>.Builder builder)
            {
                _builder = builder;
            }

            public void Add<T>(string name, Input<T> input)
                => _builder.Add(name, input);
        }

        private class EmptyResourceArgs : ResourceArgs
        {
            protected override void AddProperties(PropertyBuilder builder)
            {
            }
        }
    }
}
