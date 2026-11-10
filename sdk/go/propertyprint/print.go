// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package propertyprint

import (
	"fmt"
	"io"
	"sort"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// Print the value in a human-readable JSON like format. If showSecrets is false, secret values will be redacted.
func Value(writer io.Writer, value property.Value, showSecrets bool) error {
	if value.Secret() && !showSecrets {
		_, err := writer.Write([]byte("<secret>"))
		return err
	}

	switch {
	case value.IsNull():
		_, err := writer.Write([]byte("null"))
		return err
	case value.IsBool():
		_, err := fmt.Fprintf(writer, "%v", value.AsBool())
		return err
	case value.IsNumber():
		_, err := fmt.Fprintf(writer, "%v", value.AsNumber())
		return err
	case value.IsString():
		_, err := fmt.Fprintf(writer, "%q", value.AsString())
		return err
	case value.IsAsset():
		a := value.AsAsset()
		var err error
		switch {
		case a.Text != "":
			_, err = fmt.Fprintf(writer, "asset(text=%q)", a.Text)
		case a.Path != "":
			_, err = fmt.Fprintf(writer, "asset(path=%q)", a.Path)
		case a.URI != "":
			_, err = fmt.Fprintf(writer, "asset(uri=%q)", a.URI)
		default:
			_, err = fmt.Fprintf(writer, "asset(hash=%v)", a.Hash)
		}
		return err
	case value.IsArchive():
		a := value.AsArchive()
		switch {
		case a.Path != "":
			_, err := fmt.Fprintf(writer, "archive(path=%q)", a.Path)
			return err
		case a.URI != "":
			_, err := fmt.Fprintf(writer, "archive(uri=%q)", a.URI)
			return err
		case a.Assets != nil:
			if _, err := writer.Write([]byte("archive(assets={")); err != nil {
				return err
			}
			keys := make([]string, 0, len(a.Assets))
			for k := range a.Assets {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for i, k := range keys {
				if i > 0 {
					if _, err := writer.Write([]byte(", ")); err != nil {
						return err
					}
				}
				if _, err := fmt.Fprintf(writer, "%q: ", k); err != nil {
					return err
				}
				switch av := a.Assets[k].(type) {
				case *asset.Asset:
					if err := Value(writer, property.New(av), showSecrets); err != nil {
						return err
					}
				case *archive.Archive:
					if err := Value(writer, property.New(av), showSecrets); err != nil {
						return err
					}
				default:
					if _, err := fmt.Fprintf(writer, "%v", av); err != nil {
						return err
					}
				}
			}
			_, err := writer.Write([]byte("})"))
			return err
		default:
			_, err := fmt.Fprintf(writer, "archive(hash=%v)", a.Hash)
			return err
		}
	case value.IsArray():
		_, err := writer.Write([]byte("["))
		if err != nil {
			return err
		}
		arr := value.AsArray()
		for i, elem := range arr.All {
			if i > 0 {
				_, err := writer.Write([]byte(", "))
				if err != nil {
					return err
				}
			}
			err := Value(writer, elem, showSecrets)
			if err != nil {
				return err
			}
		}
		_, err = writer.Write([]byte("]"))
		return err
	case value.IsMap():
		_, err := writer.Write([]byte("{"))
		if err != nil {
			return err
		}
		obj := value.AsMap()
		i := 0
		for k, v := range obj.AllStable {
			if i > 0 {
				_, err := writer.Write([]byte(", "))
				if err != nil {
					return err
				}
			}
			_, err := fmt.Fprintf(writer, "%q: ", k)
			if err != nil {
				return err
			}
			err = Value(writer, v, showSecrets)
			if err != nil {
				return err
			}
			i++
		}
		_, err = writer.Write([]byte("}"))
		return err
	}

	contract.Failf("unsupported property value: %v", value)
	return nil
}
