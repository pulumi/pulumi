// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
'use strict';
var grpc = require('@grpc/grpc-js');
var pulumi_plugin_pb = require('./plugin_pb.js');

function serialize_pulumirpc_PackageDependency(arg) {
  if (!(arg instanceof pulumi_plugin_pb.PackageDependency)) {
    throw new Error('Expected argument of type pulumirpc.PackageDependency');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_PackageDependency(buffer_arg) {
  return pulumi_plugin_pb.PackageDependency.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_PackageSpec(arg) {
  if (!(arg instanceof pulumi_plugin_pb.PackageSpec)) {
    throw new Error('Expected argument of type pulumirpc.PackageSpec');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_PackageSpec(buffer_arg) {
  return pulumi_plugin_pb.PackageSpec.deserializeBinary(new Uint8Array(buffer_arg));
}


// `PackageResolver` resolves a [](pulumirpc.PackageSpec) -- a user-supplied package reference such as a name,
// registry coordinate, git URL, or local path, optionally carrying a version and parameters -- into a concrete,
// downloadable [](pulumirpc.PackageDependency). The engine exposes this service to resource providers as part of the
// provider handshake so they can resolve packages the same way the CLI does.
//
// This is currently unstable and experimental.
var PackageResolverService = exports.PackageResolverService = {
  // `ResolvePackage` resolves the given package specification to a concrete package dependency.
resolvePackage: {
    path: '/pulumirpc.PackageResolver/ResolvePackage',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_plugin_pb.PackageSpec,
    responseType: pulumi_plugin_pb.PackageDependency,
    requestSerialize: serialize_pulumirpc_PackageSpec,
    requestDeserialize: deserialize_pulumirpc_PackageSpec,
    responseSerialize: serialize_pulumirpc_PackageDependency,
    responseDeserialize: deserialize_pulumirpc_PackageDependency,
  },
};

exports.PackageResolverClient = grpc.makeGenericClientConstructor(PackageResolverService, 'PackageResolver');
