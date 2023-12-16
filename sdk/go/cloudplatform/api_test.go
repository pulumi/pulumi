package cloudplatform

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
)

var testOrg = "pulumi"

func TestNewClient(t *testing.T) {
	_, err := NewPulumiAPI(nil)
	if err != nil {
		t.Error(err)
	}
}

func TestGetOrg(t *testing.T) {
	p, err := NewPulumiAPI(nil)
	if err != nil {
		t.Error(err)
	}
	org, err := p.GetOrg(context.Background(), testOrg)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%+v", org))
}

func TestCreateGetStack(t *testing.T) {

	stackName := fmt.Sprintf("test-%08d", rand.Intn(100000000))
	p, err := NewPulumiAPI(nil)
	if err != nil {
		t.Error(err)
	}
	ctx := context.Background()
	err = p.GetStack(ctx, testOrg, "automation-jobs", stackName)
	if err != nil {
		err = p.CreateStack(ctx, testOrg, "automation-jobs", stackName)
		t.Log(err)
		if err != nil {
			t.Error(err)
		}
	}

	err = p.DeleteStack(ctx, testOrg, "automation-jobs", stackName)
	if err != nil {
		t.Error(err)
	}
}
