// Copyright 2016-2019, Pulumi Corporation

namespace Pulumi.Serialization
{
    internal static class Constants
    {
        /// <summary>
        /// Unknown values are encoded as a distinguished string value.
        /// </summary>
        public const string UnknownValue = "04da6b54-80e4-46f7-96ec-b56ff0331ba9";

        /// <summary>
        /// SpecialSigKey is sometimes used to encode type identity inside of a map. See sdk/go/common/resource/properties.go.
        /// </summary>
        public const string SpecialSigKey = "4dabf18193072939515e22adb298388d";

        /// <summary>
        /// SpecialAssetSig is a randomly assigned hash used to identify assets in maps. See sdk/go/common/resource/asset.go.
        /// </summary>
        public const string SpecialAssetSig = "c44067f5952c0a294b673a41bacd8c17";

        /// <summary>
        /// SpecialArchiveSig is a randomly assigned hash used to identify archives in maps. See sdk/go/common/resource/asset.go.
        /// </summary>
        public const string SpecialArchiveSig = "0def7320c3a5731c473e5ecbe6d01bc7";

        /// <summary>
        /// SpecialSecretSig is a randomly assigned hash used to identify secrets in maps. See sdk/go/common/resource/properties.go.
        /// </summary>
        public const string SpecialSecretSig = "1b47061264138c4ac30d75fd1eb44270";

        /// <summary>
        /// SpecialResourceSig is a randomly assigned hash used to identify resources in maps. See sdk/go/common/resource/properties.go.
        /// </summary>
        public const string SpecialResourceSig = "5cf8f73096256a8f31e491e813e4eb8e";

        public const string SecretValueName = "value";

        public const string AssetTextName = "text";
        public const string ArchiveAssetsName = "assets";

        public const string AssetOrArchivePathName = "path";
        public const string AssetOrArchiveUriName = "uri";

        public const string ResourceUrnName = "urn";
        public const string ResourceIdName = "id";
        public const string ResourceVersionName = "packageVersion";

        public const string IdPropertyName = "id";
        public const string UrnPropertyName = "urn";
    }
}
