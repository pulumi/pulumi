//go:build js

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"syscall/js"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

var terminal = js.Global().Get("terminal")
var uint8Array = js.Global().Get("Uint8Array")
var promise = js.Global().Get("Promise")

func newPromise(handler func(resolve, reject js.Value)) js.Value {
	return promise.New(js.FuncOf(func(_ js.Value, args []js.Value) interface{} {
		handler(args[0], args[1])
		return nil
	}))
}

func catch(x interface{}, err *error) {
	if x != nil {
		e, ok := x.(error)
		if !ok {
			panic(x)
		}
		*err = e
	}
}

type jsWriter struct {
	write js.Value
}

func (w *jsWriter) Write(b []byte) (n int, err error) {
	defer func() { catch(recover(), &err) }()

	buf := uint8Array.New(len(b))
	js.CopyBytesToJS(buf, b)
	nv := w.write.Invoke(buf)
	return nv.Int(), nil
}

func newRenderer(_ js.Value, args []js.Value) interface{} {
	op := args[0].String()
	action := apitype.UpdateKind(args[1].String())
	stack := tokens.QName(args[2].String())
	proj := tokens.PackageName(args[3].String())
	isPreview := args[4].Bool()

	var write js.Value
	if len(args) >= 6 {
		write = args[5]
	} else {
		write = terminal.Get("write")
	}

	writer := jsWriter{write: write}
	r := NewRenderer(op, action, stack, proj, isPreview, display.Options{
		Color:         colors.Always,
		IsInteractive: true,
		Stdout:        &writer,
		Stderr:        &writer,
	})

	render := js.FuncOf(func(_ js.Value, args []js.Value) interface{} {
		var apiEvent apitype.EngineEvent
		if err := json.NewDecoder(strings.NewReader(args[0].String())).Decode(&apiEvent); err != nil {
			panic(fmt.Errorf("decoding event: %w", err))
		}

		event, err := display.ConvertJSONEvent(apiEvent)
		if err != nil {
			panic(fmt.Errorf("converting event: %w", err))
		}

		return newPromise(func(resolve, reject js.Value) {
			go func() {
				r.Render(event)
				resolve.Invoke()
			}()
		})
	})

	close := js.FuncOf(func(_ js.Value, args []js.Value) interface{} {
		return newPromise(func(resolve, reject js.Value) {
			go func() {
				if err := r.Close(); err != nil {
					reject.Invoke(err.Error())
				}
				resolve.Invoke()
			}()
		})
		return nil
	})

	return js.ValueOf(map[string]interface{}{
		"render": render,
		"close":  close,
	})
}

func main() {
	c := make(chan bool)

	js.Global().Set("Renderer", js.FuncOf(newRenderer))

	<-c
}
