### TypeScript

```typescript
import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const function = new aws.lambda.Function("function", {
  architectures: ["string"],
  code: new pulumi.asset.FileAsset("./file.txt"),
  codeSigningConfigArn: "string",
  deadLetterConfig: {
    targetArn: "string",
  },
  description: "string",
  environment: {
    variables: {
      "string": "string"
    },
  },
  ephemeralStorage: {
    size: 0,
  },
  fileSystemConfig: {
    arn: "string",
    localMountPath: "string",
  },
  handler: "string",
  imageConfig: {
    commands: ["string"],
    entryPoints: ["string"],
    workingDirectory: "string",
  },
  imageUri: "string",
  kmsKeyArn: "string",
  layers: ["string"],
  memorySize: 0,
  name: "string",
  packageType: "string",
  publish: true|false,
  reservedConcurrentExecutions: 0,
  role: "string",
  runtime: "string",
  s3Bucket: "string",
  s3Key: "string",
  s3ObjectVersion: "string",
  sourceCodeHash: "string",
  tags: {
    "string": "string"
  },
  tagsAll: {
    "string": "string"
  },
  timeout: 0,
  tracingConfig: {
    mode: "string",
  },
  vpcConfig: {
    securityGroupIds: ["string"],
    subnetIds: ["string"],
    vpcId: "string",
  },
});
```

### Python

```python
import pulumi
import pulumi_aws as aws

function = aws.lambda_.Function("function",
  architectures=[
    "string",
  ],
  code=pulumi.FileAsset("./file.txt"),
  code_signing_config_arn="string",
  dead_letter_config=aws.lambda_.FunctionDeadLetterConfigArgs(
    target_arn="string",
  ),
  description="string",
  environment=aws.lambda_.FunctionEnvironmentArgs(
    variables={
      'string': "string"
    },
  ),
  ephemeral_storage=aws.lambda_.FunctionEphemeralStorageArgs(
    size=0,
  ),
  file_system_config=aws.lambda_.FunctionFileSystemConfigArgs(
    arn="string",
    local_mount_path="string",
  ),
  handler="string",
  image_config=aws.lambda_.FunctionImageConfigArgs(
    commands=[
      "string",
    ],
    entry_points=[
      "string",
    ],
    working_directory="string",
  ),
  image_uri="string",
  kms_key_arn="string",
  layers=[
    "string",
  ],
  memory_size=0,
  name="string",
  package_type="string",
  publish=True|False,
  reserved_concurrent_executions=0,
  role="string",
  runtime="string",
  s3_bucket="string",
  s3_key="string",
  s3_object_version="string",
  source_code_hash="string",
  tags={
    'string': "string"
  },
  tags_all={
    'string': "string"
  },
  timeout=0,
  tracing_config=aws.lambda_.FunctionTracingConfigArgs(
    mode="string",
  ),
  vpc_config=aws.lambda_.FunctionVpcConfigArgs(
    security_group_ids=[
      "string",
    ],
    subnet_ids=[
      "string",
    ],
    vpc_id="string",
  )
)
```

### C#

```csharp
using Pulumi;
using Aws = Pulumi.Aws;

var function = new Aws.Lambda.Function("function", new () 
{
  Architectures = new []
  {
    "string"
  },
  Code = new FileAsset("./file.txt"),
  CodeSigningConfigArn = "string",
  DeadLetterConfig = new Aws.Lambda.Inputs.FunctionDeadLetterConfigArgs
  {
    TargetArn = "string",
  },
  Description = "string",
  Environment = new Aws.Lambda.Inputs.FunctionEnvironmentArgs
  {
    Variables = {
      ["string"] = "string"
    },
  },
  EphemeralStorage = new Aws.Lambda.Inputs.FunctionEphemeralStorageArgs
  {
    Size = 0,
  },
  FileSystemConfig = new Aws.Lambda.Inputs.FunctionFileSystemConfigArgs
  {
    Arn = "string",
    LocalMountPath = "string",
  },
  Handler = "string",
  ImageConfig = new Aws.Lambda.Inputs.FunctionImageConfigArgs
  {
    Commands = new []
    {
      "string"
    },
    EntryPoints = new []
    {
      "string"
    },
    WorkingDirectory = "string",
  },
  ImageUri = "string",
  KmsKeyArn = "string",
  Layers = new []
  {
    "string"
  },
  MemorySize = 0,
  Name = "string",
  PackageType = "string",
  Publish = true|false,
  ReservedConcurrentExecutions = 0,
  Role = "string",
  Runtime = "string",
  S3Bucket = "string",
  S3Key = "string",
  S3ObjectVersion = "string",
  SourceCodeHash = "string",
  Tags = {
    ["string"] = "string"
  },
  TagsAll = {
    ["string"] = "string"
  },
  Timeout = 0,
  TracingConfig = new Aws.Lambda.Inputs.FunctionTracingConfigArgs
  {
    Mode = "string",
  },
  VpcConfig = new Aws.Lambda.Inputs.FunctionVpcConfigArgs
  {
    SecurityGroupIds = new []
    {
      "string"
    },
    SubnetIds = new []
    {
      "string"
    },
    VpcId = "string",
  },
});
```

### Go

```go
import (
  "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
  "github.com/pulumi/pulumi-aws/sdk/v3/go/aws/lambda"
)

function, err := lambda.NewFunction("function", &lambda.FunctionArgs{
  Architectures: pulumi.StringArray{
    pulumi.String("string")
  },
  Code: pulumi.NewFileArchive("./file.txt"),
  CodeSigningConfigArn: pulumi.String("string"),
  DeadLetterConfig: &lambda.FunctionDeadLetterConfigArgs{
    TargetArn: pulumi.String("string"),
  },
  Description: pulumi.String("string"),
  Environment: &lambda.FunctionEnvironmentArgs{
    Variables: pulumi.StringMap{
      "string": pulumi.String("string")
    },
  },
  EphemeralStorage: &lambda.FunctionEphemeralStorageArgs{
    Size: pulumi.Int(0),
  },
  FileSystemConfig: &lambda.FunctionFileSystemConfigArgs{
    Arn: pulumi.String("string"),
    LocalMountPath: pulumi.String("string"),
  },
  Handler: pulumi.String("string"),
  ImageConfig: &lambda.FunctionImageConfigArgs{
    Commands: pulumi.StringArray{
      pulumi.String("string")
    },
    EntryPoints: pulumi.StringArray{
      pulumi.String("string")
    },
    WorkingDirectory: pulumi.String("string"),
  },
  ImageUri: pulumi.String("string"),
  KmsKeyArn: pulumi.String("string"),
  Layers: pulumi.StringArray{
    pulumi.String("string")
  },
  MemorySize: pulumi.Int(0),
  Name: pulumi.String("string"),
  PackageType: pulumi.String("string"),
  Publish: pulumi.Bool(true|false),
  ReservedConcurrentExecutions: pulumi.Int(0),
  Role: pulumi.String("string"),
  Runtime: pulumi.String("string"),
  S3Bucket: pulumi.String("string"),
  S3Key: pulumi.String("string"),
  S3ObjectVersion: pulumi.String("string"),
  SourceCodeHash: pulumi.String("string"),
  Tags: pulumi.StringMap{
    "string": pulumi.String("string")
  },
  TagsAll: pulumi.StringMap{
    "string": pulumi.String("string")
  },
  Timeout: pulumi.Int(0),
  TracingConfig: &lambda.FunctionTracingConfigArgs{
    Mode: pulumi.String("string"),
  },
  VpcConfig: &lambda.FunctionVpcConfigArgs{
    SecurityGroupIds: pulumi.StringArray{
      pulumi.String("string")
    },
    SubnetIds: pulumi.StringArray{
      pulumi.String("string")
    },
    VpcId: pulumi.String("string"),
  },
})
```

### Java

```java
import com.pulumi.Pulumi;
import java.util.List;
import java.util.Map;

var function = new Function("function", FunctionArgs.builder()
  .architectures(List.of("string"))
  .code(new FileAsset("./file.txt"))
  .codeSigningConfigArn("string")
  .deadLetterConfig(FunctionDeadLetterConfigArgs.builder()
    .targetArn("string")
    .build())
  .description("string")
  .environment(FunctionEnvironmentArgs.builder()
    .variables(Map.ofEntries(
      Map.entry("string", "string")
    ))
    .build())
  .ephemeralStorage(FunctionEphemeralStorageArgs.builder()
    .size(0)
    .build())
  .fileSystemConfig(FunctionFileSystemConfigArgs.builder()
    .arn("string")
    .localMountPath("string")
    .build())
  .handler("string")
  .imageConfig(FunctionImageConfigArgs.builder()
    .commands(List.of("string"))
    .entryPoints(List.of("string"))
    .workingDirectory("string")
    .build())
  .imageUri("string")
  .kmsKeyArn("string")
  .layers(List.of("string"))
  .memorySize(0)
  .name("string")
  .packageType("string")
  .publish(true|false)
  .reservedConcurrentExecutions(0)
  .role("string")
  .runtime("string")
  .s3Bucket("string")
  .s3Key("string")
  .s3ObjectVersion("string")
  .sourceCodeHash("string")
  .tags(Map.ofEntries(
    Map.entry("string", "string")
  ))
  .tagsAll(Map.ofEntries(
    Map.entry("string", "string")
  ))
  .timeout(0)
  .tracingConfig(FunctionTracingConfigArgs.builder()
    .mode("string")
    .build())
  .vpcConfig(FunctionVpcConfigArgs.builder()
    .securityGroupIds(List.of("string"))
    .subnetIds(List.of("string"))
    .vpcId("string")
    .build())
  .build());
```

### YAML

```yaml
name: example
runtime: yaml
resources:
  function:
    type: aws:lambda:Function
    properties:
      architectures: ["string"]
      code: 
        Fn::FileAsset: ./file.txt
      codeSigningConfigArn: "string"
      deadLetterConfig: 
        targetArn: "string"
      description: "string"
      environment: 
        variables: 
          "string": "string"
      ephemeralStorage: 
        size: 0
      fileSystemConfig: 
        arn: "string"
        localMountPath: "string"
      handler: "string"
      imageConfig: 
        commands: ["string"]
        entryPoints: ["string"]
        workingDirectory: "string"
      imageUri: "string"
      kmsKeyArn: "string"
      layers: ["string"]
      memorySize: 0
      name: "string"
      packageType: "string"
      publish: true|false
      reservedConcurrentExecutions: 0
      role: "string"
      runtime: "string"
      s3Bucket: "string"
      s3Key: "string"
      s3ObjectVersion: "string"
      sourceCodeHash: "string"
      tags: 
        "string": "string"
      tagsAll: 
        "string": "string"
      timeout: 0
      tracingConfig: 
        mode: "string"
      vpcConfig: 
        securityGroupIds: ["string"]
        subnetIds: ["string"]
        vpcId: "string"
```

