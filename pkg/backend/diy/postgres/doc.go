// Copyright 2016-2024, Pulumi Corporation.
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

// Package postgres provides a PostgreSQL-based backend implementation for Pulumi state storage.
//
// This package automatically registers the PostgreSQL bucket provider with the default
// blob.URLMux during initialization, allowing DIY backends to work seamlessly with PostgreSQL URLs.
//
// Example usage:
//
//	import (
//	    "context"
//	    "os"
//
//	    "github.com/pulumi/pulumi/pkg/v3/backend/diy"
//	    _ "github.com/pulumi/pulumi/pkg/v3/backend/diy/postgres" // Import to register PostgreSQL provider
//	    "github.com/pulumi/pulumi/pkg/v3/diag"
//	)
//
//	func main() {
//	    // Initialize a PostgreSQL backend
//	    ctx := context.Background()
//	    postgresURL := "postgres://username:password@hostname:5432/database"
//	    backend, err := diy.New(ctx, diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{}), postgresURL, nil)
//	    if err != nil {
//	        panic(err)
//	    }
//
//	    // Now use the backend to manage Pulumi stacks
//	    // ...
//	}
//
// The PostgreSQL backend stores Pulumi state in a single table, with keys
// formatted to represent a file-like path structure matching the expected
// Pulumi storage layout.
package postgres
