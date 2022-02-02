package refresher

import (
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)


func (c *Client) Login() (backend.Backend, error) {
	return httpstate.Login(c.Ctx, cmdutil.Diag(), c.URL, display.Options{})
}

func (c *Client) ListStacks(b backend.Backend) ([]backend.StackSummary, backend.ContinuationToken, error) {
	return b.ListStacks(c.Ctx, backend.ListStacksFilter{}, nil)
}
