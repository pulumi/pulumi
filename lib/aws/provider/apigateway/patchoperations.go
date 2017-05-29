package apigateway

import (
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/pulumi/lumi/pkg/resource"

	awsapigateway "github.com/aws/aws-sdk-go/service/apigateway"
)

// patchOperations is a utility function to compute a set of PatchOperations to perform based on the diffs
// for a provided map of properties and new values.  Any provided ignoreProps are skipped from being considered
// as sources of patch operations.
func patchOperations(diff *resource.ObjectDiff, ignoreProps ...resource.PropertyKey) (
	[]*awsapigateway.PatchOperation, error) {

	ignores := map[resource.PropertyKey]bool{}
	for _, p := range ignoreProps {
		ignores[p] = true
	}

	return patchOperationsForObject("", diff, ignores)
}

func patchOperationsForObject(path string, diff *resource.ObjectDiff, ignores map[resource.PropertyKey]bool) (
	[]*awsapigateway.PatchOperation, error) {

	var ops []*awsapigateway.PatchOperation
	if diff == nil {
		return ops, nil
	}
	for _, name := range diff.Keys() {
		if ignores != nil && ignores[name] {
			continue
		}
		if diff.Added(name) {
			v, err := jsonStringify(diff.Adds[name])
			if err != nil {
				return nil, err
			}
			ops = append(ops, &awsapigateway.PatchOperation{
				Op:    aws.String("add"),
				Path:  aws.String(path + "/" + string(name)),
				Value: v,
			})
		}
		if diff.Updated(name) {
			update := diff.Updates[name]
			arrayOps, err := patchOperationsForValue(path+"/"+string(name), &update)
			if err != nil {
				return nil, err
			}
			for _, op := range arrayOps {
				ops = append(ops, op)
			}
		}
		if diff.Deleted(name) {
			ops = append(ops, &awsapigateway.PatchOperation{
				Op:   aws.String("remove"),
				Path: aws.String(path + "/" + string(name)),
			})
		}
	}
	return ops, nil
}

func patchOperationsForValue(path string, diff *resource.ValueDiff) ([]*awsapigateway.PatchOperation, error) {
	var ops []*awsapigateway.PatchOperation
	if diff.Array != nil {
		return patchOperationsForArray(path, diff.Array)
	} else if diff.Object != nil {
		return patchOperationsForObject(path, diff.Object, nil)
	} else {
		v, err := jsonStringify(diff.New)
		if err != nil {
			return nil, err
		}
		ops = append(ops, &awsapigateway.PatchOperation{
			Op:    aws.String("replace"),
			Path:  aws.String(path),
			Value: v,
		})
	}
	return ops, nil
}

func patchOperationsForArray(path string, diff *resource.ArrayDiff) ([]*awsapigateway.PatchOperation, error) {
	var ops []*awsapigateway.PatchOperation
	if diff == nil {
		return ops, nil
	}
	for i, add := range diff.Adds {
		addOp, err := newOp("add", path+"/"+strconv.Itoa(i), &add)
		if err != nil {
			return nil, err
		}
		ops = append(ops, addOp)
	}
	for i, update := range diff.Updates {
		arrayOps, err := patchOperationsForValue(path+"/"+strconv.Itoa(i), &update)
		if err != nil {
			return nil, err
		}
		for _, op := range arrayOps {
			ops = append(ops, op)
		}
	}
	for i := range diff.Deletes {
		deleteOp, err := newOp("delete", path+"/"+strconv.Itoa(i), nil)
		if err != nil {
			return nil, err
		}
		ops = append(ops, deleteOp)
	}
	return ops, nil
}

func newOp(op string, path string, i *resource.PropertyValue) (*awsapigateway.PatchOperation, error) {
	patchOp := awsapigateway.PatchOperation{
		Op:   aws.String(op),
		Path: aws.String(path),
	}
	if i != nil {
		v, err := jsonStringify(*i)
		if err != nil {
			return nil, err
		}
		patchOp.Value = v
	}
	return &patchOp, nil
}

func jsonStringify(i resource.PropertyValue) (*string, error) {
	var s string
	switch v := i.V.(type) {
	case nil:
		return nil, nil
	case bool:
		if v {
			s = "true"
		} else {
			s = "false"
		}
	case float64:
		s = strconv.Itoa(int(v))
	case string:
		s = v
	case []resource.PropertyValue:
		s = "["
		isFirst := true
		for _, pv := range v {
			pvj, err := jsonStringify(pv)
			if pvj == nil {
				tmp := "null"
				pvj = &tmp
			}
			if err != nil {
				return nil, err
			}
			if !isFirst {
				s += ", "
			}
			s += *pvj
		}
		s += "]"
	case resource.PropertyMap:
		s = "{"
		isFirst := true
		for pk, pv := range v {
			pvj, err := jsonStringify(pv)
			if pvj != nil {
				if err != nil {
					return nil, err
				}
				if !isFirst {
					s += ", "
				}
				s += "\"" + string(pk) + "\": " + *pvj
			}
		}
		s += "}"
	case resource.URN:
		s = string(v)
	default:
		return nil, fmt.Errorf("unexpected diff type %v", v)
	}
	return &s, nil
}
