package cloudplatform

import (
	"context"
	"fmt"
	"testing"
)

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
	org, err := p.GetOrg(context.Background(), "pulumi")
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%+v", org))
}
