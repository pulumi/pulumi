// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016-2023, Pulumi Corporation.
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
var pulumi_language_pb = require('./language_pb.js');
var pulumi_codegen_hcl_pb = require('./codegen/hcl_pb.js');
var pulumi_plugin_pb = require('./plugin_pb.js');
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');

function serialize_google_protobuf_Empty(arg) {
  if (!(arg instanceof google_protobuf_empty_pb.Empty)) {
    throw new Error('Expected argument of type google.protobuf.Empty');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_google_protobuf_Empty(buffer_arg) {
  return google_protobuf_empty_pb.Empty.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_AboutRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.AboutRequest)) {
    throw new Error('Expected argument of type pulumirpc.AboutRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_AboutRequest(buffer_arg) {
  return pulumi_language_pb.AboutRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_AboutResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.AboutResponse)) {
    throw new Error('Expected argument of type pulumirpc.AboutResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_AboutResponse(buffer_arg) {
  return pulumi_language_pb.AboutResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GeneratePackageRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.GeneratePackageRequest)) {
    throw new Error('Expected argument of type pulumirpc.GeneratePackageRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GeneratePackageRequest(buffer_arg) {
  return pulumi_language_pb.GeneratePackageRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GeneratePackageResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.GeneratePackageResponse)) {
    throw new Error('Expected argument of type pulumirpc.GeneratePackageResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GeneratePackageResponse(buffer_arg) {
  return pulumi_language_pb.GeneratePackageResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GenerateProgramRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.GenerateProgramRequest)) {
    throw new Error('Expected argument of type pulumirpc.GenerateProgramRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GenerateProgramRequest(buffer_arg) {
  return pulumi_language_pb.GenerateProgramRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GenerateProgramResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.GenerateProgramResponse)) {
    throw new Error('Expected argument of type pulumirpc.GenerateProgramResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GenerateProgramResponse(buffer_arg) {
  return pulumi_language_pb.GenerateProgramResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GenerateProjectRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.GenerateProjectRequest)) {
    throw new Error('Expected argument of type pulumirpc.GenerateProjectRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GenerateProjectRequest(buffer_arg) {
  return pulumi_language_pb.GenerateProjectRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GenerateProjectResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.GenerateProjectResponse)) {
    throw new Error('Expected argument of type pulumirpc.GenerateProjectResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GenerateProjectResponse(buffer_arg) {
  return pulumi_language_pb.GenerateProjectResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetProgramDependenciesRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.GetProgramDependenciesRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetProgramDependenciesRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetProgramDependenciesRequest(buffer_arg) {
  return pulumi_language_pb.GetProgramDependenciesRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetProgramDependenciesResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.GetProgramDependenciesResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetProgramDependenciesResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetProgramDependenciesResponse(buffer_arg) {
  return pulumi_language_pb.GetProgramDependenciesResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetRequiredPackagesRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.GetRequiredPackagesRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetRequiredPackagesRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetRequiredPackagesRequest(buffer_arg) {
  return pulumi_language_pb.GetRequiredPackagesRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetRequiredPackagesResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.GetRequiredPackagesResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetRequiredPackagesResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetRequiredPackagesResponse(buffer_arg) {
  return pulumi_language_pb.GetRequiredPackagesResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetRequiredPluginsRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.GetRequiredPluginsRequest)) {
    throw new Error('Expected argument of type pulumirpc.GetRequiredPluginsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetRequiredPluginsRequest(buffer_arg) {
  return pulumi_language_pb.GetRequiredPluginsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_GetRequiredPluginsResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.GetRequiredPluginsResponse)) {
    throw new Error('Expected argument of type pulumirpc.GetRequiredPluginsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_GetRequiredPluginsResponse(buffer_arg) {
  return pulumi_language_pb.GetRequiredPluginsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_InstallDependenciesRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.InstallDependenciesRequest)) {
    throw new Error('Expected argument of type pulumirpc.InstallDependenciesRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_InstallDependenciesRequest(buffer_arg) {
  return pulumi_language_pb.InstallDependenciesRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_InstallDependenciesResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.InstallDependenciesResponse)) {
    throw new Error('Expected argument of type pulumirpc.InstallDependenciesResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_InstallDependenciesResponse(buffer_arg) {
  return pulumi_language_pb.InstallDependenciesResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_LanguageHandshakeRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.LanguageHandshakeRequest)) {
    throw new Error('Expected argument of type pulumirpc.LanguageHandshakeRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_LanguageHandshakeRequest(buffer_arg) {
  return pulumi_language_pb.LanguageHandshakeRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_LanguageHandshakeResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.LanguageHandshakeResponse)) {
    throw new Error('Expected argument of type pulumirpc.LanguageHandshakeResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_LanguageHandshakeResponse(buffer_arg) {
  return pulumi_language_pb.LanguageHandshakeResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_LinkRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.LinkRequest)) {
    throw new Error('Expected argument of type pulumirpc.LinkRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_LinkRequest(buffer_arg) {
  return pulumi_language_pb.LinkRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_LinkResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.LinkResponse)) {
    throw new Error('Expected argument of type pulumirpc.LinkResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_LinkResponse(buffer_arg) {
  return pulumi_language_pb.LinkResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_PackRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.PackRequest)) {
    throw new Error('Expected argument of type pulumirpc.PackRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_PackRequest(buffer_arg) {
  return pulumi_language_pb.PackRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_PackResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.PackResponse)) {
    throw new Error('Expected argument of type pulumirpc.PackResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_PackResponse(buffer_arg) {
  return pulumi_language_pb.PackResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_PluginInfo(arg) {
  if (!(arg instanceof pulumi_plugin_pb.PluginInfo)) {
    throw new Error('Expected argument of type pulumirpc.PluginInfo');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_PluginInfo(buffer_arg) {
  return pulumi_plugin_pb.PluginInfo.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunPluginRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.RunPluginRequest)) {
    throw new Error('Expected argument of type pulumirpc.RunPluginRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunPluginRequest(buffer_arg) {
  return pulumi_language_pb.RunPluginRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunPluginResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.RunPluginResponse)) {
    throw new Error('Expected argument of type pulumirpc.RunPluginResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunPluginResponse(buffer_arg) {
  return pulumi_language_pb.RunPluginResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.RunRequest)) {
    throw new Error('Expected argument of type pulumirpc.RunRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunRequest(buffer_arg) {
  return pulumi_language_pb.RunRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RunResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.RunResponse)) {
    throw new Error('Expected argument of type pulumirpc.RunResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RunResponse(buffer_arg) {
  return pulumi_language_pb.RunResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RuntimeOptionsRequest(arg) {
  if (!(arg instanceof pulumi_language_pb.RuntimeOptionsRequest)) {
    throw new Error('Expected argument of type pulumirpc.RuntimeOptionsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RuntimeOptionsRequest(buffer_arg) {
  return pulumi_language_pb.RuntimeOptionsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_RuntimeOptionsResponse(arg) {
  if (!(arg instanceof pulumi_language_pb.RuntimeOptionsResponse)) {
    throw new Error('Expected argument of type pulumirpc.RuntimeOptionsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_pulumirpc_RuntimeOptionsResponse(buffer_arg) {
  return pulumi_language_pb.RuntimeOptionsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// The LanguageRuntime service defines a standard interface for [language hosts/runtimes](languages). At a high level, a
// language runtime provides the ability to execute programs, install and query dependencies, and generate code for a
// specific language.
var LanguageRuntimeService = exports.LanguageRuntimeService = {
  // `Handshake` is the first call made by the engine to a language host. It is used to pass the engine's address to
// the language host so that it may establish its own connections back, and to establish protocol configuration that
// will be used to communicate between the two parties.
handshake: {
    path: '/pulumirpc.LanguageRuntime/Handshake',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_language_pb.LanguageHandshakeRequest,
    responseType: pulumi_language_pb.LanguageHandshakeResponse,
    requestSerialize: serialize_pulumirpc_LanguageHandshakeRequest,
    requestDeserialize: deserialize_pulumirpc_LanguageHandshakeRequest,
    responseSerialize: serialize_pulumirpc_LanguageHandshakeResponse,
    responseDeserialize: deserialize_pulumirpc_LanguageHandshakeResponse,
  },
  // `GetRequiredPlugins` computes the complete set of anticipated [plugins](plugins) required by a Pulumi program.
// Among other things, it is intended to be used to pre-install plugins before running a program with
// [](pulumirpc.LanguageRuntime.Run), to avoid the need to install them on-demand in response to [resource
// registrations](resource-registration) sent back from the running program to the engine.
//
// :::{important}
// The use of `GetRequiredPlugins` is deprecated in favour of [](pulumirpc.LanguageRuntime.GetRequiredPackages),
// which returns more granular information about which plugins are required by which packages.
// :::
getRequiredPlugins: {
    path: '/pulumirpc.LanguageRuntime/GetRequiredPlugins',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_language_pb.GetRequiredPluginsRequest,
    responseType: pulumi_language_pb.GetRequiredPluginsResponse,
    requestSerialize: serialize_pulumirpc_GetRequiredPluginsRequest,
    requestDeserialize: deserialize_pulumirpc_GetRequiredPluginsRequest,
    responseSerialize: serialize_pulumirpc_GetRequiredPluginsResponse,
    responseDeserialize: deserialize_pulumirpc_GetRequiredPluginsResponse,
  },
  // `GetRequiredPackages` computes the complete set of anticipated [packages](pulumirpc.PackageDependency) required
// by a program. It is used to pre-install packages before running a program with [](pulumirpc.LanguageRuntime.Run),
// to avoid the need to install them on-demand in response to [resource registrations](resource-registration) sent
// back from the running program to the engine. Moreover, when importing resources into a stack, it is used to
// determine which plugins are required to service the import of a given resource, since given the presence of
// [parameterized providers](parameterized-providers), it is not in general true that a package name corresponds 1:1
// with a plugin name. It replaces [](pulumirpc.LanguageRuntime.GetRequiredPlugins) in the face of [parameterized
// providers](parameterized-providers), which as mentioned above can enable multiple instances of the same plugin to
// provide multiple packages.
getRequiredPackages: {
    path: '/pulumirpc.LanguageRuntime/GetRequiredPackages',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_language_pb.GetRequiredPackagesRequest,
    responseType: pulumi_language_pb.GetRequiredPackagesResponse,
    requestSerialize: serialize_pulumirpc_GetRequiredPackagesRequest,
    requestDeserialize: deserialize_pulumirpc_GetRequiredPackagesRequest,
    responseSerialize: serialize_pulumirpc_GetRequiredPackagesResponse,
    responseDeserialize: deserialize_pulumirpc_GetRequiredPackagesResponse,
  },
  // `Run` executes a Pulumi program, returning information about whether or not the program produced an error.
run: {
    path: '/pulumirpc.LanguageRuntime/Run',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_language_pb.RunRequest,
    responseType: pulumi_language_pb.RunResponse,
    requestSerialize: serialize_pulumirpc_RunRequest,
    requestDeserialize: deserialize_pulumirpc_RunRequest,
    responseSerialize: serialize_pulumirpc_RunResponse,
    responseDeserialize: deserialize_pulumirpc_RunResponse,
  },
  // `GetPluginInfo` returns information about the [plugin](plugins) implementing this language runtime.
getPluginInfo: {
    path: '/pulumirpc.LanguageRuntime/GetPluginInfo',
    requestStream: false,
    responseStream: false,
    requestType: google_protobuf_empty_pb.Empty,
    responseType: pulumi_plugin_pb.PluginInfo,
    requestSerialize: serialize_google_protobuf_Empty,
    requestDeserialize: deserialize_google_protobuf_Empty,
    responseSerialize: serialize_pulumirpc_PluginInfo,
    responseDeserialize: deserialize_pulumirpc_PluginInfo,
  },
  // `InstallDependencies` accepts a request specifying a Pulumi project and program that can be executed with
// [](pulumirpc.LanguageRuntime.Run) and installs the dependencies for that program (e.g. by running `npm install`
// for NodeJS, or `pip install` for Python). Since dependency installation could take a while, and callers may wish
// to report on its progress, this method returns a stream of [](pulumirpc.InstallDependenciesResponse) messages
// containing information about standard error and output.
installDependencies: {
    path: '/pulumirpc.LanguageRuntime/InstallDependencies',
    requestStream: false,
    responseStream: true,
    requestType: pulumi_language_pb.InstallDependenciesRequest,
    responseType: pulumi_language_pb.InstallDependenciesResponse,
    requestSerialize: serialize_pulumirpc_InstallDependenciesRequest,
    requestDeserialize: deserialize_pulumirpc_InstallDependenciesRequest,
    responseSerialize: serialize_pulumirpc_InstallDependenciesResponse,
    responseDeserialize: deserialize_pulumirpc_InstallDependenciesResponse,
  },
  // `RuntimeOptionsPrompts` accepts a request specifying a Pulumi project and returns a list of additional prompts to
// ask during `pulumi new`.
runtimeOptionsPrompts: {
    path: '/pulumirpc.LanguageRuntime/RuntimeOptionsPrompts',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_language_pb.RuntimeOptionsRequest,
    responseType: pulumi_language_pb.RuntimeOptionsResponse,
    requestSerialize: serialize_pulumirpc_RuntimeOptionsRequest,
    requestDeserialize: deserialize_pulumirpc_RuntimeOptionsRequest,
    responseSerialize: serialize_pulumirpc_RuntimeOptionsResponse,
    responseDeserialize: deserialize_pulumirpc_RuntimeOptionsResponse,
  },
  // `About` returns information about the language runtime being used.
about: {
    path: '/pulumirpc.LanguageRuntime/About',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_language_pb.AboutRequest,
    responseType: pulumi_language_pb.AboutResponse,
    requestSerialize: serialize_pulumirpc_AboutRequest,
    requestDeserialize: deserialize_pulumirpc_AboutRequest,
    responseSerialize: serialize_pulumirpc_AboutResponse,
    responseDeserialize: deserialize_pulumirpc_AboutResponse,
  },
  // `GetProgramDependencies` computes the set of language-level dependencies (e.g. NPM packages for NodeJS, or Maven
// libraries for Java) required by a program.
getProgramDependencies: {
    path: '/pulumirpc.LanguageRuntime/GetProgramDependencies',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_language_pb.GetProgramDependenciesRequest,
    responseType: pulumi_language_pb.GetProgramDependenciesResponse,
    requestSerialize: serialize_pulumirpc_GetProgramDependenciesRequest,
    requestDeserialize: deserialize_pulumirpc_GetProgramDependenciesRequest,
    responseSerialize: serialize_pulumirpc_GetProgramDependenciesResponse,
    responseDeserialize: deserialize_pulumirpc_GetProgramDependenciesResponse,
  },
  // `RunPlugin` is used to execute a program written in this host's language that implements a Pulumi
// [plugin](plugins). It it is plugins what [](pulumirpc.LanguageRuntime.Run) is to programs. Since a plugin is not
// expected to terminate until instructed/for a long time, this method returns a stream of
// [](pulumirpc.RunPluginResponse) messages containing information about standard error and output, as well as the
// exit code of the plugin when it does terminate.
runPlugin: {
    path: '/pulumirpc.LanguageRuntime/RunPlugin',
    requestStream: false,
    responseStream: true,
    requestType: pulumi_language_pb.RunPluginRequest,
    responseType: pulumi_language_pb.RunPluginResponse,
    requestSerialize: serialize_pulumirpc_RunPluginRequest,
    requestDeserialize: deserialize_pulumirpc_RunPluginRequest,
    responseSerialize: serialize_pulumirpc_RunPluginResponse,
    responseDeserialize: deserialize_pulumirpc_RunPluginResponse,
  },
  // `GenerateProgram` generates code in this host's language that implements the given [PCL](pcl) program. Unlike
// [](pulumirpc.LanguageRuntime.GenerateProject), this method *only* generates program code, and does not e.g.
// generate a `package.json` for a NodeJS project that details how to run that code.
// [](pulumirpc.LanguageRuntime.GenerateProject), this method underpins ["programgen"](programgen) and the main
// functionality powering `pulumi convert`.
generateProgram: {
    path: '/pulumirpc.LanguageRuntime/GenerateProgram',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_language_pb.GenerateProgramRequest,
    responseType: pulumi_language_pb.GenerateProgramResponse,
    requestSerialize: serialize_pulumirpc_GenerateProgramRequest,
    requestDeserialize: deserialize_pulumirpc_GenerateProgramRequest,
    responseSerialize: serialize_pulumirpc_GenerateProgramResponse,
    responseDeserialize: deserialize_pulumirpc_GenerateProgramResponse,
  },
  // `GenerateProject` generates code in this host's language that implements the given [PCL](pcl) program and wraps
// it in some language-specific notion of a "project", where a project is a buildable or runnable artifact. In this
// sense, `GenerateProject`'s output is a superset of that of [](pulumirpc.LanguageRuntime.GenerateProgram). For
// instance, when generating a NodeJS project, this method might generate a corresponding `package.json` file, as
// well as the relevant NodeJS program code. Along with [](pulumirpc.LanguageRuntime.GenerateProgram), this method
// underpins ["programgen"](programgen) and the main functionality powering `pulumi convert`.
generateProject: {
    path: '/pulumirpc.LanguageRuntime/GenerateProject',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_language_pb.GenerateProjectRequest,
    responseType: pulumi_language_pb.GenerateProjectResponse,
    requestSerialize: serialize_pulumirpc_GenerateProjectRequest,
    requestDeserialize: deserialize_pulumirpc_GenerateProjectRequest,
    responseSerialize: serialize_pulumirpc_GenerateProjectResponse,
    responseDeserialize: deserialize_pulumirpc_GenerateProjectResponse,
  },
  // `GeneratePackage` generates code in this host's language that implements an [SDK](sdkgen) ("sdkgen") for the
// given Pulumi package, as specified by a [schema](schema).
generatePackage: {
    path: '/pulumirpc.LanguageRuntime/GeneratePackage',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_language_pb.GeneratePackageRequest,
    responseType: pulumi_language_pb.GeneratePackageResponse,
    requestSerialize: serialize_pulumirpc_GeneratePackageRequest,
    requestDeserialize: deserialize_pulumirpc_GeneratePackageRequest,
    responseSerialize: serialize_pulumirpc_GeneratePackageResponse,
    responseDeserialize: deserialize_pulumirpc_GeneratePackageResponse,
  },
  // `Pack` accepts a request specifying a generated SDK package and packs it into a language-specific artifact. For
// instance, in the case of Java, it might produce a JAR file from a list of `.java` sources; in the case of NodeJS,
// a `.tgz` file might be produced from a list of `.js` sources; and so on. Presently, `Pack` is primarily used in
// [language conformance tests](language-conformance-tests), though it is intended to be used more widely in future
// to standardise e.g. provider publishing workflows.
pack: {
    path: '/pulumirpc.LanguageRuntime/Pack',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_language_pb.PackRequest,
    responseType: pulumi_language_pb.PackResponse,
    requestSerialize: serialize_pulumirpc_PackRequest,
    requestDeserialize: deserialize_pulumirpc_PackRequest,
    responseSerialize: serialize_pulumirpc_PackResponse,
    responseDeserialize: deserialize_pulumirpc_PackResponse,
  },
  // `Link` links local dependencies into a project (program or plugin). The dependencies can be binary artifacts such
// as wheel or tar.gz files, or source directories. `Link` will update the language specific project files, such as
// `package.json`, `pyproject.toml`, `go.mod`, etc, to include the dependency. `Link` returns instructions for the
// user on how to use the linked package in the project.
link: {
    path: '/pulumirpc.LanguageRuntime/Link',
    requestStream: false,
    responseStream: false,
    requestType: pulumi_language_pb.LinkRequest,
    responseType: pulumi_language_pb.LinkResponse,
    requestSerialize: serialize_pulumirpc_LinkRequest,
    requestDeserialize: deserialize_pulumirpc_LinkRequest,
    responseSerialize: serialize_pulumirpc_LinkResponse,
    responseDeserialize: deserialize_pulumirpc_LinkResponse,
  },
  // `Cancel` signals the language runtime to gracefully shut down and abort any ongoing operations.
// Operations aborted in this way will return an error.
cancel: {
    path: '/pulumirpc.LanguageRuntime/Cancel',
    requestStream: false,
    responseStream: false,
    requestType: google_protobuf_empty_pb.Empty,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_google_protobuf_Empty,
    requestDeserialize: deserialize_google_protobuf_Empty,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
};

exports.LanguageRuntimeClient = grpc.makeGenericClientConstructor(LanguageRuntimeService, 'LanguageRuntime');
