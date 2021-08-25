// Copyright 2016-2021, Pulumi Corporation

// NOTE: currently this is manually pasted from pulumi-azure-native to
// enable running unit tests over the generated code in this folder.
// It would be better to generate the code as part of the codegen
// test from the schema.

using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Threading.Tasks;
using Pulumi.Serialization;

namespace Pulumi.MadeupPackage.Codegentest.Outputs
{

    [OutputType]
    public sealed class StorageAccountKeyResponse
    {
        /// <summary>
        /// Creation time of the key, in round trip date format.
        /// </summary>
        public readonly string CreationTime;
        /// <summary>
        /// Name of the key.
        /// </summary>
        public readonly string KeyName;
        /// <summary>
        /// Permissions for the key -- read-only or full permissions.
        /// </summary>
        public readonly string Permissions;
        /// <summary>
        /// Base 64-encoded value of the key.
        /// </summary>
        public readonly string Value;

        [OutputConstructor]
        private StorageAccountKeyResponse(
            string creationTime,

            string keyName,

            string permissions,

            string value)
        {
            CreationTime = creationTime;
            KeyName = keyName;
            Permissions = permissions;
            Value = value;
        }
    }
}
