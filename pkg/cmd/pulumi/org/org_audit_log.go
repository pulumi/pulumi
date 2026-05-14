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

package org

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

func newOrgAuditLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "audit-log",
		Short:  "Inspect organization audit logs",
		Long:   "[EXPERIMENTAL] Inspect organization audit logs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newOrgAuditLogListCmd())
	cmd.AddCommand(newOrgAuditLogExportCmd())
	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23000]: Not yet implemented.
func newOrgAuditLogExportCmd() *cobra.Command {
	var (
		org       string
		format    string
		eventType string
		userLogin string
		startTime string
		token     string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "export",
		Short:  "Export audit log events for an organization",
		Long:   "[EXPERIMENTAL] Export audit log events for an organization.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&org, "org", "", "The organization to export audit logs for")
	cmd.Flags().StringVar(&format, "format", "csv", "The export format: csv or cef")
	cmd.Flags().StringVar(&eventType, "event-type", "", "Filter by event type")
	cmd.Flags().StringVar(&userLogin, "user", "", "Filter by user login")
	cmd.Flags().StringVar(&startTime, "start-time", "",
		"The upper bound of the time range (V1 semantics)")
	cmd.Flags().StringVar(&token, "continuation-token", "",
		"The continuation token for paginated retrieval")

	return cmd
}
