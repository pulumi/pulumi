package main

import (
	"example.com/pulumi-testconfigprovider/sdk/go/testconfigprovider"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)
func main() {
pulumi.Run(func(ctx *pulumi.Context) error {
// The schema provider covers interesting schema shapes.
schemaprov, err := testconfigprovider.NewProvider(ctx, "schemaprov", &testconfigprovider.ProviderArgs{
S1: pulumi.String(""),
S2: pulumi.String("x"),
S3: pulumi.String("{}"),
I1: pulumi.Int(0),
I2: pulumi.Int(42),
N1: pulumi.Float64(0),
N2: pulumi.Float64(42.42),
B1: pulumi.Bool(true),
B2: pulumi.Bool(false),
Ls1: pulumi.StringArray{
},
Ls2: pulumi.StringArray{
pulumi.String(""),
pulumi.String("foo"),
},
Li1: pulumi.IntArray{
pulumi.Int(1),
pulumi.Int(2),
},
Ms1: pulumi.StringMap{
},
Ms2: pulumi.StringMap{
"key1": pulumi.String("value1"),
"key2": pulumi.String("value2"),
},
Mi1: pulumi.IntMap{
"key1": pulumi.Int(0),
"key2": pulumi.Int(42),
},
Os1: &testconfigprovider.Ts1Args{
},
Os2: &testconfigprovider.Ts2Args{
X: pulumi.String("x-value"),
},
Oi1: &testconfigprovider.Ti1Args{
X: pulumi.Int(42),
},
})
if err != nil {
return err
}
_, err = testconfigprovider.NewConfigGetter(ctx, "schemaconf", nil, pulumi.Provider(schemaprov))
if err != nil {
return err
}
// The program_secret_provider covers scenarios where user passes secret values to the provider.
programsecretprov, err := testconfigprovider.NewProvider(ctx, "programsecretprov", &testconfigprovider.ProviderArgs{
S1: testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
S: pulumi.String("SECRET"),
}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (string, error) {
return invoke.S, nil
}).(pulumi.StringOutput),
I1: testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
I: pulumi.Int(1234567890),
}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (int, error) {
return invoke.I, nil
}).(pulumi.IntOutput),
N1: testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
N: pulumi.Float64(123456.789),
}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (float64, error) {
return invoke.N, nil
}).(pulumi.Float64Output),
B1: testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
B: pulumi.Bool(true),
}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (bool, error) {
return invoke.B, nil
}).(pulumi.BoolOutput),
Ls1: testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
Ls: pulumi.StringArray{
pulumi.String("SECRET"),
pulumi.String("SECRET2"),
},
}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) ([]string, error) {
return invoke.Ls, nil
}).(pulumi.StringArrayOutput),
Ls2: pulumi.StringArray{
pulumi.String("VALUE"),
testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
S: pulumi.String("SECRET"),
}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (string, error) {
return invoke.S, nil
}).(pulumi.StringOutput),
},
Ms1: testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
Ms: pulumi.StringMap{
"key1": pulumi.String("SECRET"),
"key2": pulumi.String("SECRET2"),
},
}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (map[string]string, error) {
return invoke.Ms, nil
}).(pulumi.Map[string]stringOutput),
Ms2: pulumi.StringMap{
"key1": pulumi.String("value1"),
"key2": testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
S: pulumi.String("SECRET"),
}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (string, error) {
return invoke.S, nil
}).(pulumi.StringOutput),
},
Os1: testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
Os: %!v(PANIC=Format method: runtime error: slice bounds out of range [3:2]),
}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (testconfigprovider.Ts, error) {
return testconfigprovider.Ts(invoke.Os), nil
}).(testconfigprovider.TsOutput),
Os2: &testconfigprovider.Ts2Args{
X: testconfigprovider.ToSecretOutput(ctx, testconfigprovider.ToSecretOutputArgs{
S: pulumi.String("SECRET"),
}, nil).ApplyT(func(invoke testconfigprovider.ToSecretResult) (string, error) {
return invoke.S, nil
}).(pulumi.StringOutput),
},
})
if err != nil {
return err
}
_, err = testconfigprovider.NewConfigGetter(ctx, "programsecretconf", nil, pulumi.Provider(programsecretprov))
if err != nil {
return err
}
return nil
})
}
