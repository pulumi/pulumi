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

function serialize_pulumirpc_ResolvePackageRequest(arg) {
  if (!(arg instanceof pulumi_plugin_pb.ResolvePackageRequest)) {
    throw new Error('Expected argument of type pulumirpc.ResolvePackageRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ResolvePackageRequest(buffer_arg) {
  return pulumi_plugin_pb.ResolvePackageRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_ResolvePackageResponse(arg) {
  if (!(arg instanceof pulumi_plugin_pb.ResolvePackageResponse)) {
    throw new Error('Expected argument of type pulumirpc.ResolvePackageResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_ResolvePackageResponse(buffer_arg) {
  return pulumi_plugin_pb.ResolvePackageResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// PackageResolver is a service, hosted by the engine, that resolves a loosely-specified [](pulumirpc.PackageSpec) into
// a fully-resolved [](pulumirpc.PackageDependency). A `PackageSpec` may contain references that cannot be acted upon
// directly -- a registry reference such as `hashicorp/aws` paired with a version range such as `~>6.0`, or a Terraform
// module source, for instance. Resolution turns these into a concrete plugin (with a definite version and download
// location) and, where the spec carries parameters, runs parameterization so that the resulting parameterization
// *value* is baked into the result.
//
// Providers receive the address of a `PackageResolver` during [](pulumirpc.ResourceProvider.Handshake) so that they
// may resolve package references encountered while servicing later calls. This is what allows a provider serving a
// parameterized or dynamic package (e.g. Pulumi HCL bridging a Terraform module) to bake the packages it references
// while handling [](pulumirpc.ResourceProvider.Parameterize) and [](pulumirpc.ResourceProvider.Construct).
var PackageResolverService = exports.PackageResolverService = {
  // ResolvePackage resolves a single [](pulumirpc.PackageSpec) to a concrete [](pulumirpc.PackageDependency).
resolvePackage: {
    path: '/pulumirpc.PackageResolver/ResolvePackage',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_plugin_pb.ResolvePackageRequest,
    responseType: pulumi_plugin_pb.ResolvePackageResponse,
    requestSerialize: serialize_pulumirpc_ResolvePackageRequest,
    requestDeserialize: deserialize_pulumirpc_ResolvePackageRequest,
    responseSerialize: serialize_pulumirpc_ResolvePackageResponse,
    responseDeserialize: deserialize_pulumirpc_ResolvePackageResponse,
  },
};

exports.PackageResolverClient = grpc.makeGenericClientConstructor(PackageResolverService, 'PackageResolver');
