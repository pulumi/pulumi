package plugin

import (
	b64 "encoding/base64"
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"
	proto "github.com/golang/protobuf/proto"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func Test7132(t *testing.T) {
	reqStr := "Cithd3M6aWFtL2dldFBvbGljeURvY3VtZW50OmdldFBvbGljeURvY3VtZW50En8KfQoKc3RhdGVtZW50cxJvMm0KayppCjcKCXJlc291cmNlcxIqMigKJhokMDRkYTZiNTQtODBlNC00NmY3LTk2ZWMtYjU2ZmYwMzMxYmE5Ci4KB2FjdGlvbnMSIzIhCh8aHXNlY3JldHNtYW5hZ2VyOkdldFNlY3JldFZhbHVlKAE="
	req := readRequest(t, reqStr)

	fmt.Printf("req.GetArgs(): ")
	spew.Dump(req.GetArgs())

	propertyMap1, err := UnmarshalProperties(req.GetArgs(),
		MarshalOptions{
			Label:         "label",
			KeepUnknowns:  true,
			KeepSecrets:   true,
			KeepResources: true,
		})

	if err != nil {
		t.Error(err)
	}

	fmt.Printf("propertyMap1: ")
	spew.Dump(propertyMap1)

	args2, err := MarshalProperties(propertyMap1, MarshalOptions{
		Label: "label",
	})
	if err != nil {
		t.Error(err)
	}

	fmt.Printf("args2: ")
	spew.Dump(args2)

	args3 := imitateNetworkPass(t, &pulumirpc.InvokeRequest{
		Tok:             req.Tok,
		Args:            args2,
		AcceptResources: req.AcceptResources,
	}).GetArgs()

	fmt.Printf("args3: ")
	spew.Dump(args3)

	propertyMap2, err := UnmarshalProperties(args3,
		MarshalOptions{
			Label:         "label",
			KeepUnknowns:  true,
			KeepSecrets:   true,
			KeepResources: true,
		})

	fmt.Printf("propertyMap2: ")
	spew.Dump(propertyMap2)

	if err != nil {
		t.Error(err)
	}
}

func imitateNetworkPass(t *testing.T, req *pulumirpc.InvokeRequest) *pulumirpc.InvokeRequest {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		t.Error(err)
		return nil
	}
	ir := new(pulumirpc.InvokeRequest)
	if err := proto.Unmarshal(reqBytes, ir); err != nil {
		t.Error(err)
		return nil
	}
	return ir

}

func readRequest(t *testing.T, base64 string) *pulumirpc.InvokeRequest {
	reqBytes, err := b64.StdEncoding.DecodeString(base64)
	if err != nil {
		t.Error(err)
	}

	ir := new(pulumirpc.InvokeRequest)
	if err := proto.Unmarshal(reqBytes, ir); err != nil {
		t.Error(err)
	}
	return ir
}
