#!/bin/bash
protoc -I=. --csharp_out ../dotnet/ --grpc_out ../dotnet/ --plugin=protoc-gen-grpc=/home/matell/.nuget/packages/grpc.tools/1.10.0/tools/linux_x64/grpc_csharp_plugin *.proto
