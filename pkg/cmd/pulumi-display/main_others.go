//go:build !js

package main

import (
	"encoding/json"
	"log"
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

	r := NewRenderer(op, action, stack, proj, isPreview, display.Options{
		Color:         colors.Always,
		IsInteractive: true,
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
	})

	dec := json.NewDecoder(os.Stdin)
	for {
		var apiEvent apitype.EngineEvent
		if err := dec.Decode(&apiEvent); err != nil {
			log.Fatal(err)
		}

		event, err := display.ConvertJSONEvent(apiEvent)
		if err != nil {
			log.Fatal(err)
		}
		r.Render(event)

		if event.Type == events.CancelEvent {
			break
		}
	}
	if err := r.Close(); err != nil {
		log.Fatal(err)
	}
}
