package plugin

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestAnnotateSecrets(t *testing.T) {
	from := resource.PropertyMap{
		"stringValue": resource.MakeSecret(resource.NewStringProperty("hello")),
		"numberValue": resource.MakeSecret(resource.NewNumberProperty(1.00)),
		"boolValue":   resource.MakeSecret(resource.NewBoolProperty(true)),
		"secretArrayValue": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("b"),
			resource.NewStringProperty("c"),
		})),
		"secretObjectValue": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.NewStringProperty("bValue"),
			"c": resource.NewStringProperty("cValue"),
		})),
		"objectWithSecretValue": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.MakeSecret(resource.NewStringProperty("bValue")),
			"c": resource.NewStringProperty("cValue"),
		}),
	}

	to := resource.PropertyMap{
		"stringValue": resource.NewStringProperty("hello"),
		"numberValue": resource.NewNumberProperty(1.00),
		"boolValue":   resource.NewBoolProperty(true),
		"secretArrayValue": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("b"),
			resource.NewStringProperty("c"),
		}),
		"secretObjectValue": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.NewStringProperty("bValue"),
			"c": resource.NewStringProperty("cValue"),
		}),
		"objectWithSecretValue": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.NewStringProperty("bValue"),
			"c": resource.NewStringProperty("cValue"),
		}),
	}

	annotateSecrets(to, from)

	assert.Truef(t, reflect.DeepEqual(to, from), "objects should be deeply equal")
}

func TestAnnotateSecretsDifferentProperties(t *testing.T) {
	// ensure that if from and and to have different shapes, values on from are not put into to, values on to which
	// are not present in from stay in to, but any secretness is propigated for shared keys.

	from := resource.PropertyMap{
		"stringValue": resource.MakeSecret(resource.NewStringProperty("hello")),
		"numberValue": resource.MakeSecret(resource.NewNumberProperty(1.00)),
		"boolValue":   resource.MakeSecret(resource.NewBoolProperty(true)),
		"secretObjectValue": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.NewStringProperty("bValue"),
			"c": resource.NewStringProperty("cValue"),
		})),
		"objectWithSecretValue": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.MakeSecret(resource.NewStringProperty("bValue")),
			"c": resource.NewStringProperty("cValue"),
		}),
		"extraFromValue": resource.NewStringProperty("extraFromValue"),
	}

	to := resource.PropertyMap{
		"stringValue": resource.NewStringProperty("hello"),
		"numberValue": resource.NewNumberProperty(1.00),
		"boolValue":   resource.NewBoolProperty(true),
		"secretObjectValue": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.NewStringProperty("bValue"),
			"c": resource.NewStringProperty("cValue"),
		})),
		"objectWithSecretValue": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("aValue"),
			"b": resource.NewStringProperty("bValue"),
			"c": resource.NewStringProperty("cValue"),
		}),
		"extraToValue": resource.NewStringProperty("extraToValue"),
	}

	annotateSecrets(to, from)

	for key, val := range to {
		fromVal, fromHas := from[key]
		if !fromHas {
			continue
		}

		assert.Truef(t, reflect.DeepEqual(fromVal, val), "expected properties %s to be deeply equal", key)
	}

	_, has := to["extraFromValue"]
	assert.Falsef(t, has, "to should not have a key named extraFromValue, it was not present before annotating secrets")

	_, has = to["extraToValue"]
	assert.True(t, has, "to should have a key named extraToValue, even though it was not in the from value")
}

func TestAnnotateSecretsArrays(t *testing.T) {
	from := resource.PropertyMap{
		"secretArray": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("b"),
			resource.NewStringProperty("c"),
		})),
		"arrayWithSecrets": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.MakeSecret(resource.NewStringProperty("b")),
			resource.NewStringProperty("c"),
		}),
	}

	to := resource.PropertyMap{
		"secretArray": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("b"),
			resource.NewStringProperty("c"),
		}),
		"arrayWithSecrets": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("c"),
			resource.NewStringProperty("b"),
		}),
	}

	expected := resource.PropertyMap{
		"secretArray": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("b"),
			resource.NewStringProperty("c"),
		})),
		"arrayWithSecrets": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("a"),
			resource.NewStringProperty("c"),
			resource.NewStringProperty("b"),
		})),
	}

	annotateSecrets(to, from)

	assert.Truef(t, reflect.DeepEqual(to, expected), "did not match expected after annotation")
}

func TestNestedSecret(t *testing.T) {
	from := resource.PropertyMap{
		"secretString": resource.MakeSecret(resource.NewStringProperty("shh")),
		"secretArray": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("hello"),
			resource.MakeSecret(resource.NewStringProperty("shh")),
			resource.NewStringProperty("goodbye")}),
		"secretMap": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
			"b": resource.NewStringProperty("b"),
		})),
		"deepSecretMap": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
			"b": resource.MakeSecret(resource.NewStringProperty("b")),
		}),
	}

	to := resource.PropertyMap{
		"secretString": resource.NewStringProperty("shh"),
		"secretArray": resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("shh"),
			resource.NewStringProperty("hello"),
			resource.NewStringProperty("goodbye")}),
		"secretMap": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
			"b": resource.NewStringProperty("b"),
		})),
		"deepSecretMap": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
			"b": resource.NewStringProperty("b"),
			// Note the additional property here, which we expect to be kept when annotating.
			"c": resource.NewStringProperty("c"),
		}),
	}

	expected := resource.PropertyMap{
		"secretString": resource.MakeSecret(resource.NewStringProperty("shh")),
		// The entire array has been marked a secret because it contained a secret member in from. Since arrays
		// are often used for sets, we didn't try to apply the secretness to a specific member of the array, like
		// we would have with maps (where we can use the keys to correlate related properties)
		"secretArray": resource.MakeSecret(resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewStringProperty("shh"),
			resource.NewStringProperty("hello"),
			resource.NewStringProperty("goodbye")})),
		"secretMap": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
			"b": resource.NewStringProperty("b"),
		})),
		"deepSecretMap": resource.NewObjectProperty(resource.PropertyMap{
			"a": resource.NewStringProperty("a"),
			"b": resource.MakeSecret(resource.NewStringProperty("b")),
			"c": resource.NewStringProperty("c"),
		}),
	}

	annotateSecrets(to, from)

	assert.Truef(t, reflect.DeepEqual(to, expected), "did not match expected after annotation")
}
