// Copyright 2016 Pulumi, Inc. All rights reserved.

package mapper

import (
	"fmt"
	"reflect"
)

func ErrMissing(ty reflect.Type, field string) error {
	return fmt.Errorf("Missing required %v field `%v`", ty, field)
}

func ErrWrongType(ty reflect.Type, field string, expect reflect.Type, actual reflect.Type) error {
	return fmt.Errorf("%v `%v` must be a `%v`, got `%v`", ty, field, expect, actual)
}
