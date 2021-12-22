package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

const (
	// computedValue is a magic number we emit for a value of a resource.Property value
	// whenever we need to serialize a resource.Computed. (Since the real/actual value
	// is not known.) This allows us to persist engine events and resource states that
	// indicate a value will changed... but is unknown what it will change to.
	computedValuePlaceholder = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
)

var resourceSchema *jsonschema.Schema
var propertyValueSchema *jsonschema.Schema

func init() {
	compiler := jsonschema.NewCompiler()
	compiler.LoadURL = func(s string) (io.ReadCloser, error) {
		var schema string
		switch s {
		case apitype.ResourceSchemaID:
			schema = apitype.ResourceSchema()
		case apitype.PropertyValueSchemaID:
			schema = apitype.PropertyValueSchema()
		default:
			return jsonschema.LoadURL(s)
		}
		return ioutil.NopCloser(strings.NewReader(schema)), nil
	}
	resourceSchema = compiler.MustCompile(apitype.ResourceSchemaID)
	propertyValueSchema = compiler.MustCompile(apitype.PropertyValueSchemaID)
}

// SerializeResource turns a resource into a structure suitable for serialization.
func SerializeResource(res *resource.State, enc config.Encrypter, showSecrets bool) (apitype.ResourceV3, error) {
	contract.Assert(res != nil)
	contract.Assertf(string(res.URN) != "", "Unexpected empty resource resource.URN")

	// Serialize all input and output properties recursively, and add them if non-empty.
	var inputs map[string]interface{}
	if inp := res.Inputs; inp != nil {
		sinp, err := SerializeProperties(inp, enc, showSecrets)
		if err != nil {
			return apitype.ResourceV3{}, err
		}
		inputs = sinp
	}
	var outputs map[string]interface{}
	if outp := res.Outputs; outp != nil {
		soutp, err := SerializeProperties(outp, enc, showSecrets)
		if err != nil {
			return apitype.ResourceV3{}, err
		}
		outputs = soutp
	}

	v3Resource := apitype.ResourceV3{
		URN:                     res.URN,
		Custom:                  res.Custom,
		Delete:                  res.Delete,
		ID:                      res.ID,
		Type:                    res.Type,
		Parent:                  res.Parent,
		Inputs:                  inputs,
		Outputs:                 outputs,
		Protect:                 res.Protect,
		External:                res.External,
		Dependencies:            res.Dependencies,
		InitErrors:              res.InitErrors,
		Provider:                res.Provider,
		PropertyDependencies:    res.PropertyDependencies,
		PendingReplacement:      res.PendingReplacement,
		AdditionalSecretOutputs: res.AdditionalSecretOutputs,
		Aliases:                 res.Aliases,
		ImportID:                res.ImportID,
	}

	if res.CustomTimeouts.IsNotEmpty() {
		v3Resource.CustomTimeouts = &res.CustomTimeouts
	}

	return v3Resource, nil
}

// SerializeProperties serializes a resource property bag so that it's suitable for serialization.
func SerializeProperties(props resource.PropertyMap, enc config.Encrypter,
	showSecrets bool) (map[string]interface{}, error) {
	dst := make(map[string]interface{})
	for _, k := range props.StableKeys() {
		v, err := SerializePropertyValue(props[k], enc, showSecrets)
		if err != nil {
			return nil, err
		}
		dst[string(k)] = v
	}
	return dst, nil
}

// SerializePropertyValue serializes a resource property value so that it's suitable for serialization.
func SerializePropertyValue(prop resource.PropertyValue, enc config.Encrypter,
	showSecrets bool) (interface{}, error) {
	// Serialize nulls as nil.
	if prop.IsNull() {
		return nil, nil
	}

	// A computed value marks something that will be determined at a later time. (e.g. the result of
	// a computation that we don't perform during a preview operation.) We serialize a magic constant
	// to record its existence.
	if prop.IsComputed() || prop.IsOutput() {
		return computedValuePlaceholder, nil
	}

	// For arrays, make sure to recurse.
	if prop.IsArray() {
		srcarr := prop.ArrayValue()
		dstarr := make([]interface{}, len(srcarr))
		for i, elem := range prop.ArrayValue() {
			selem, err := SerializePropertyValue(elem, enc, showSecrets)
			if err != nil {
				return nil, err
			}
			dstarr[i] = selem
		}
		return dstarr, nil
	}

	// Also for objects, recurse and use naked properties.
	if prop.IsObject() {
		return SerializeProperties(prop.ObjectValue(), enc, showSecrets)
	}

	// For assets, we need to serialize them a little carefully, so we can recover them afterwards.
	if prop.IsAsset() {
		return prop.AssetValue().Serialize(), nil
	} else if prop.IsArchive() {
		return prop.ArchiveValue().Serialize(), nil
	}

	// We serialize resource references using a map-based representation similar to assets, archives, and secrets.
	if prop.IsResourceReference() {
		ref := prop.ResourceReferenceValue()
		serialized := map[string]interface{}{
			resource.SigKey:  resource.ResourceReferenceSig,
			"urn":            string(ref.URN),
			"packageVersion": ref.PackageVersion,
		}
		if id, hasID := ref.IDString(); hasID {
			serialized["id"] = id
		}
		return serialized, nil
	}

	if prop.IsSecret() {
		// Since we are going to encrypt property value, we can elide encrypting sub-elements. We'll mark them as
		// "secret" so we retain that information when deserializaing the overall structure, but there is no
		// need to double encrypt everything.
		value, err := SerializePropertyValue(prop.SecretValue().Element, config.NopEncrypter, showSecrets)
		if err != nil {
			return nil, err
		}
		bytes, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("encoding serialized property value: %w", err)
		}
		plaintext := string(bytes)

		// If the encrypter is a cachingCrypter, call through its encryptSecret method, which will look for a matching
		// *resource.Secret + plaintext in its cache in order to avoid re-encrypting the value.
		var ciphertext string
		if cachingCrypter, ok := enc.(*cachingCrypter); ok {
			ciphertext, err = cachingCrypter.encryptSecret(prop.SecretValue(), plaintext)
		} else {
			ciphertext, err = enc.EncryptValue(plaintext)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt secret value: %w", err)
		}
		contract.AssertNoErrorf(err, "marshalling underlying secret value to JSON")

		secret := apitype.SecretV1{
			Sig: resource.SecretSig,
		}

		if showSecrets {
			secret.Plaintext = plaintext
		} else {
			secret.Ciphertext = ciphertext
		}

		return secret, nil
	}

	// All others are returned as-is.
	return prop.V, nil
}

// DeserializeResource turns a serialized resource back into its usual form.
func DeserializeResource(res apitype.ResourceV3, dec config.Decrypter, enc config.Encrypter) (*resource.State, error) {
	// Deserialize the resource properties, if they exist.
	inputs, err := DeserializeProperties(res.Inputs, dec, enc)
	if err != nil {
		return nil, err
	}
	outputs, err := DeserializeProperties(res.Outputs, dec, enc)
	if err != nil {
		return nil, err
	}

	if res.URN == "" {
		return nil, fmt.Errorf("resource missing required 'urn' field")
	}

	if res.Type == "" {
		return nil, fmt.Errorf("resource '%s' missing required 'type' field", res.URN)
	}

	if !res.Custom && res.ID != "" {
		return nil, fmt.Errorf("resource '%s' has 'custom' false but non-empty ID", res.URN)
	}

	return resource.NewState(
		res.Type, res.URN, res.Custom, res.Delete, res.ID,
		inputs, outputs, res.Parent, res.Protect, res.External, res.Dependencies, res.InitErrors, res.Provider,
		res.PropertyDependencies, res.PendingReplacement, res.AdditionalSecretOutputs, res.Aliases, res.CustomTimeouts,
		res.ImportID), nil
}

// DeserializeProperties deserializes an entire map of deploy properties into a resource property map.
func DeserializeProperties(props map[string]interface{}, dec config.Decrypter,
	enc config.Encrypter) (resource.PropertyMap, error) {
	result := make(resource.PropertyMap)
	for k, prop := range props {
		desprop, err := DeserializePropertyValue(prop, dec, enc)
		if err != nil {
			return nil, err
		}
		result[resource.PropertyKey(k)] = desprop
	}
	return result, nil
}

// DeserializePropertyValue deserializes a single deploy property into a resource property value.
func DeserializePropertyValue(v interface{}, dec config.Decrypter,
	enc config.Encrypter) (resource.PropertyValue, error) {
	if v != nil {
		switch w := v.(type) {
		case bool:
			return resource.NewBoolProperty(w), nil
		case float64:
			return resource.NewNumberProperty(w), nil
		case string:
			if w == computedValuePlaceholder {
				return resource.MakeComputed(resource.NewStringProperty("")), nil
			}
			return resource.NewStringProperty(w), nil
		case []interface{}:
			var arr []resource.PropertyValue
			for _, elem := range w {
				ev, err := DeserializePropertyValue(elem, dec, enc)
				if err != nil {
					return resource.PropertyValue{}, err
				}
				arr = append(arr, ev)
			}
			return resource.NewArrayProperty(arr), nil
		case map[string]interface{}:
			obj, err := DeserializeProperties(w, dec, enc)
			if err != nil {
				return resource.PropertyValue{}, err
			}

			// This could be an asset or archive; if so, recover its type.
			objmap := obj.Mappable()
			if sig, hasSig := objmap[resource.SigKey]; hasSig {
				switch sig {
				case resource.AssetSig:
					asset, isasset, err := resource.DeserializeAsset(objmap)
					if err != nil {
						return resource.PropertyValue{}, err
					}
					contract.Assert(isasset)
					return resource.NewAssetProperty(asset), nil
				case resource.ArchiveSig:
					archive, isarchive, err := resource.DeserializeArchive(objmap)
					if err != nil {
						return resource.PropertyValue{}, err
					}
					contract.Assert(isarchive)
					return resource.NewArchiveProperty(archive), nil
				case resource.SecretSig:
					ciphertext, cipherOk := objmap["ciphertext"].(string)
					plaintext, plainOk := objmap["plaintext"].(string)
					if (!cipherOk && !plainOk) || (plainOk && cipherOk) {
						return resource.PropertyValue{}, errors.New(
							"malformed secret value: one of `ciphertext` or `plaintext` must be supplied")
					}

					if plainOk {
						encryptedText, err := enc.EncryptValue(plaintext)
						if err != nil {
							return resource.PropertyValue{}, fmt.Errorf("encrypting secret value: %w", err)
						}
						ciphertext = encryptedText

					} else {
						unencryptedText, err := dec.DecryptValue(ciphertext)
						if err != nil {
							return resource.PropertyValue{}, fmt.Errorf("decrypting secret value: %w", err)
						}
						plaintext = unencryptedText
					}

					var elem interface{}

					if err := json.Unmarshal([]byte(plaintext), &elem); err != nil {
						return resource.PropertyValue{}, err
					}
					ev, err := DeserializePropertyValue(elem, config.NopDecrypter, enc)
					if err != nil {
						return resource.PropertyValue{}, err
					}
					prop := resource.MakeSecret(ev)
					// If the decrypter is a cachingCrypter, insert the plain- and ciphertext into the cache with the
					// new *resource.Secret as the key.
					if cachingCrypter, ok := dec.(*cachingCrypter); ok {
						cachingCrypter.insert(prop.SecretValue(), plaintext, ciphertext)
					}
					return prop, nil
				case resource.ResourceReferenceSig:
					var packageVersion string
					if packageVersionV, ok := objmap["packageVersion"]; ok {
						packageVersion, ok = packageVersionV.(string)
						if !ok {
							return resource.PropertyValue{},
								errors.New("malformed resource reference: packageVersion must be a string")
						}
					}

					urnStr, ok := objmap["urn"].(string)
					if !ok {
						return resource.PropertyValue{}, errors.New("malformed resource reference: missing urn")
					}
					urn := resource.URN(urnStr)

					// deserializeID handles two cases, one of which arose from a bug in a refactoring of resource.ResourceReference.
					// This bug caused the raw ID PropertyValue to be serialized as a map[string]interface{}. In the normal case, the
					// ID is serialized as a string.
					deserializeID := func() (string, bool, error) {
						idV, ok := objmap["id"]
						if !ok {
							return "", false, nil
						}

						switch idV := idV.(type) {
						case string:
							return idV, true, nil
						case map[string]interface{}:
							switch v := idV["V"].(type) {
							case nil:
								// This happens for component resource references, which do not have an associated ID.
								return "", false, nil
							case string:
								// This happens for custom resource references, which do have an associated ID.
								return v, true, nil
							case map[string]interface{}:
								// This happens for custom resource references with an unknown ID. In this case, the ID should be
								// deserialized as the empty string.
								return "", true, nil
							}
						}
						return "", false, errors.New("malformed resource reference: id must be a string")
					}

					id, hasID, err := deserializeID()
					if err != nil {
						return resource.PropertyValue{}, err
					}
					if hasID {
						return resource.MakeCustomResourceReference(urn, resource.ID(id), packageVersion), nil
					}
					return resource.MakeComponentResourceReference(urn, packageVersion), nil
				default:
					return resource.PropertyValue{}, fmt.Errorf("unrecognized signature '%v' in property map", sig)
				}
			}

			// Otherwise, it's just a weakly typed object map.
			return resource.NewObjectProperty(obj), nil
		default:
			contract.Failf("Unrecognized property type %T: %v", v, reflect.ValueOf(v))
		}
	}

	return resource.NewNullProperty(), nil
}
