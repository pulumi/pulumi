// Copyright 2016-2018, Pulumi Corporation.
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

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/graphql-go/graphql"
	"github.com/spf13/cobra"
)

func newStackQueryCmd() *cobra.Command {
	var stackName string
	cmd := &cobra.Command{
		Use:   "query",
		Args:  cmdutil.ExactArgs(0),
		Short: "Enter an interactive environment for querying a stack.",
		Long:  "TODO",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := backend.DisplayOptions{
				Color: cmdutil.GetGlobalColorization(),
			}
			s, err := requireStack(stackName, false, opts)
			if err != nil {
				return err
			}

			snap, err := s.Snapshot(commandContext())
			if err != nil {
				return err
			}

			snapshot = stack.SerializeDeployment(snap)

			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query: rootQuery,
			})
			if err != nil {
				return errors.Wrapf(err, "failed to create graphql schema")
			}

			http.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
				var result *graphql.Result
				if r.Method == "POST" {
					var graphQLQuery struct {
						Query         string            `json:"query"`
						OperationName string            `json:"operationName"`
						Variables     map[string]string `json:"variables"`
					}

					err := json.NewDecoder(r.Body).Decode(&graphQLQuery)
					if err != nil {
						w.WriteHeader(http.StatusBadRequest)
						return
					}

					variableValues := make(map[string]interface{})
					for key, value := range graphQLQuery.Variables {
						variableValues[key] = value
					}

					result = graphql.Do(graphql.Params{
						Schema:         schema,
						OperationName:  graphQLQuery.OperationName,
						RequestString:  graphQLQuery.Query,
						VariableValues: variableValues,
						Context:        r.Context(),
					})
				} else {
					result = graphql.Do(graphql.Params{
						Schema:        schema,
						RequestString: r.URL.Query().Get("query"),
						Context:       r.Context(),
					})
				}

				json.NewEncoder(w).Encode(result)
			})

			fmt.Println("GraphQL server listening on http://localhost:8080")
			return http.ListenAndServe(":8080", nil)
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "", "The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

var snapshot *apitype.DeploymentV2

func findResourceByURN(urn resource.URN) apitype.ResourceV2 {
	for _, res := range snapshot.Resources {
		if res.URN == urn {
			return res
		}
	}

	return apitype.ResourceV2{}
}

var manifestType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Manifest",
	Fields: graphql.Fields{
		"time": &graphql.Field{
			Type: graphql.String,
		},
		"magic": &graphql.Field{
			Type: graphql.String,
		},
		"version": &graphql.Field{
			Type: graphql.String,
		},
	},
})

type parentEdgeData struct {
	Parent *apitype.ResourceV2 `json:"node"`
}

var parentEdgeType = graphql.NewObject(graphql.ObjectConfig{
	Name:   "ParentEdge",
	Fields: graphql.Fields{},
})

type dependencyEdgeData struct {
	Dependencies []*apitype.ResourceV2 `json:"nodes"`
}

var dependencyEdgeType = graphql.NewObject(graphql.ObjectConfig{
	Name:   "DependencyEdge",
	Fields: graphql.Fields{},
})

var resourceType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Resource",
	Fields: graphql.Fields{
		"urn":     &graphql.Field{Type: graphql.ID},
		"custom":  &graphql.Field{Type: graphql.Boolean},
		"delete":  &graphql.Field{Type: graphql.Boolean},
		"id":      &graphql.Field{Type: graphql.ID},
		"type":    &graphql.Field{Type: graphql.String},
		"inputs":  &graphql.Field{Type: graphql.String},
		"outputs": &graphql.Field{Type: graphql.String},
		"parent": &graphql.Field{
			Type: parentEdgeType,
			Resolve: func(params graphql.ResolveParams) (interface{}, error) {
				res, ok := params.Source.(apitype.ResourceV2)
				contract.Assert(ok)
				if res.Parent != "" {
					parent := findResourceByURN(res.Parent)
					return parentEdgeData{&parent}, nil
				}

				return nil, nil
			},
		},
		"protect":  &graphql.Field{Type: graphql.Boolean},
		"external": &graphql.Field{Type: graphql.Boolean},
		"dependencies": &graphql.Field{
			Type: dependencyEdgeType,
			Resolve: func(params graphql.ResolveParams) (interface{}, error) {
				res, ok := params.Source.(apitype.ResourceV2)
				if !ok {
					return dependencyEdgeData{nil}, nil
				}

				var deps []*apitype.ResourceV2
				for _, resURN := range res.Dependencies {
					depRes := findResourceByURN(resURN)
					deps = append(deps, &depRes)
				}

				return dependencyEdgeData{deps}, nil
			},
		},
	},
})

var deploymentType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Deployment",
	Fields: graphql.Fields{
		"manifest": &graphql.Field{
			Type:        manifestType,
			Description: "Gets the manifest of a deployment.",
			Resolve: func(params graphql.ResolveParams) (interface{}, error) {
				return snapshot.Manifest, nil
			},
		},
		"resources": &graphql.Field{
			Type:        graphql.NewList(resourceType),
			Description: "Gets the resources of a deployment.",
			Resolve: func(params graphql.ResolveParams) (interface{}, error) {
				return snapshot.Resources, nil
			},
		},
		"resource": &graphql.Field{
			Type:        resourceType,
			Description: "Gets a single resource.",
			Args: graphql.FieldConfigArgument{
				"urn": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.ID),
				},
			},
			Resolve: func(params graphql.ResolveParams) (interface{}, error) {
				urn, ok := params.Args["urn"].(string)
				contract.Assert(ok)
				res := findResourceByURN(resource.URN(urn))
				return res, nil
			},
		},
	},
})

var rootQuery = graphql.NewObject(graphql.ObjectConfig{
	Name: "RootQuery",
	Fields: graphql.Fields{
		"deployment": &graphql.Field{
			Type:        deploymentType,
			Description: "Gets the most recent deployment associated with this stack.",
			Resolve: func(params graphql.ResolveParams) (interface{}, error) {
				return snapshot, nil
			},
		},
	},
})

func init() {
	parentEdgeType.AddFieldConfig("node", &graphql.Field{
		Type:        resourceType,
		Description: "Gets the parent of this resource.",
	})
	dependencyEdgeType.AddFieldConfig("nodes", &graphql.Field{
		Type:        graphql.NewList(resourceType),
		Description: "Gets all dependencies of this resource.",
	})
}
