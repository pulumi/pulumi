package provider

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

func IgnoreChanges(currentArgs, newArgs interface{}, ignoreChanges []string) error {
	// Round-trip through the marshaler to keep things simple with respect to type differences.
	current, err := Marshal(currentArgs)
	if err != nil {
		return err
	}
	new, err := Marshal(newArgs)
	if err != nil {
		return err
	}

	currentObject, newObject := resource.NewObjectProperty(current), resource.NewObjectProperty(new)

	var invalidPaths []string
	for _, ignoreChange := range ignoreChanges {
		path, err := resource.ParsePropertyPath(ignoreChange)
		if err != nil {
			continue
		}

		currentValue, hasCurrent := path.Get(currentObject)
		_, hasNew := path.Get(newObject)

		var ok bool
		switch {
		case hasCurrent && hasNew:
			ok = path.Set(newObject, currentValue)
			contract.Assert(ok)
		case hasCurrent && !hasNew:
			ok = path.Set(newObject, currentValue)
		case !hasCurrent && hasNew:
			ok = path.Delete(newObject)
		default:
			ok = true
		}
		if !ok {
			invalidPaths = append(invalidPaths, ignoreChange)
		}
	}
	if len(invalidPaths) != 0 {
		return fmt.Errorf("cannot ignore changes to the following properties because one or more elements of "+
			"the path are missing: %q", strings.Join(invalidPaths, ", "))
	}

	return Unmarshal(newObject.ObjectValue(), newArgs)
}
