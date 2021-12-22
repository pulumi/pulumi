package main

import (
	"encoding/json"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine/events"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/termutil"
)

func main() {
	op := os.Args[1]
	action := apitype.UpdateKind(os.Args[2])
	stack := tokens.QName(os.Args[3])
	proj := tokens.PackageName(os.Args[4])
	isPreview := termutil.IsTruthy(os.Args[5])
	opts := display.Options{
		Color:         colors.Always,
		IsInteractive: true,
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
	}

	events := make(chan events.Event)
	done := make(chan bool)

	go display.ShowProgressEvents(op, action, stack, proj, events, done, opts, isPreview)

	dec := json.NewDecoder(os.Stdin)

	for {
		var apiEvent apitype.EngineEvent
		if err := dec.Decode(&apiEvent); err != nil {
			return
		}

		event, err := display.ConvertJSONEvent(apiEvent)
		if err != nil {
			return
		}
		events <- event
	}
}
