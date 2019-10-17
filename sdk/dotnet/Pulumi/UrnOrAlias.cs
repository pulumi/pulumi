// Copyright 2016-2018, Pulumi Corporation

#nullable enable

namespace Pulumi
{
    public readonly struct UrnOrAlias
    {
        public readonly Urn? Urn;
        public readonly Alias? Alias;

        private UrnOrAlias(Urn? urn, Alias? alias)
        {
            Urn = urn;
            Alias = alias;
        }

        public static implicit operator UrnOrAlias(Urn urn)
            => new UrnOrAlias(urn, alias: null);

        public static implicit operator UrnOrAlias(Alias alias)
            => new UrnOrAlias(urn: null, alias);
    }
}
