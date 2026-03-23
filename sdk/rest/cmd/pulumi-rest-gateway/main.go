// Copyright 2016-2026, Pulumi Corporation.
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

// pulumi-rest-gateway is an HTTP REST gateway for the Pulumi engine.
// It translates REST API calls into gRPC calls on the Pulumi ResourceMonitor,
// allowing any HTTP client to register and manage Pulumi resources without
// needing a full language SDK.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/pulumi/pulumi/sdk/rest/v3/restgateway"
)

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	flag.Parse()

	g := restgateway.NewGateway()

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Pulumi REST gateway listening on %s", addr)
	log.Printf("Endpoints:")
	log.Printf("  POST   /sessions                    - Create session")
	log.Printf("  POST   /sessions/{id}/resources      - Register resource")
	log.Printf("  POST   /sessions/{id}/invoke         - Invoke provider function")
	log.Printf("  POST   /sessions/{id}/logs           - Send log")
	log.Printf("  DELETE /sessions/{id}                - Close session")

	if err := http.ListenAndServe(addr, g.Handler()); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
