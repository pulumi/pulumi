// Copyright 2025, Pulumi Corporation.
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

// Package gitlfs implements a Git LFS storage backend for Pulumi state.
//
// This package provides a blob.Bucket implementation that stores Pulumi state
// files in a Git repository, using Git LFS (Large File Storage) for efficient
// handling of large state files.
//
// # URL Format
//
// The backend is accessed using the gitlfs:// URL scheme:
//
//	gitlfs://host/owner/repo[?ref=branch&path=subdir&lfs_threshold=bytes]
//
// Examples:
//
//	gitlfs://github.com/myorg/pulumi-state
//	gitlfs://gitlab.com/myorg/infra-state?ref=main
//	gitlfs://gitea.example.com/team/state?path=production
//
// # Query Parameters
//
//   - ref: Git branch to use (default: "main")
//   - path: Subdirectory within the repository for state files
//   - lfs_threshold: Size threshold in bytes for using LFS (default: 102400)
//
// # Authentication
//
// The backend supports multiple authentication methods, checked in order:
//
//  1. Environment variables:
//     - PULUMI_DIY_BACKEND_GITLFS_TOKEN or PULUMI_GITLFS_TOKEN: Bearer token
//     - PULUMI_DIY_BACKEND_GITLFS_USERNAME and PULUMI_DIY_BACKEND_GITLFS_PASSWORD:
//     Basic auth credentials
//
//  2. Git credential helper: Uses `git credential fill` to get stored credentials
//
// # Storage Layout
//
// State files are stored in the repository following the standard Pulumi layout:
//
//	.pulumi/
//	├── meta.yaml                    # Backend version metadata
//	├── stacks/                      # Stack checkpoints
//	│   └── <project>/
//	│       └── <stack>.json[.gz]    # May be LFS pointer if large
//	├── history/                     # Update history
//	└── backups/                     # Stack backups
//
// Files larger than the LFS threshold (default 100KB) are stored as LFS objects,
// with pointer files in the Git repository.
//
// # Git LFS Protocol
//
// This package implements the Git LFS Batch API for uploading and downloading
// large objects. It supports the basic transfer adapter and is compatible with:
//
//   - GitHub LFS
//   - GitLab LFS
//   - Self-hosted servers (Gitea, custom implementations)
//
// For more information about the Git LFS protocol, see:
// https://github.com/git-lfs/git-lfs/blob/main/docs/api/batch.md
//
// # Usage
//
// To use this backend, import it and use the gitlfs:// URL:
//
//	import (
//	    _ "github.com/pulumi/pulumi/pkg/v3/backend/diy/gitlfs"
//	)
//
//	// Then login with:
//	// pulumi login gitlfs://github.com/myorg/pulumi-state
//
// # Local Cache
//
// The backend maintains a local clone of the repository for performance.
// The cache is stored in:
//
//	~/.pulumi/gitlfs/<hash>/
//
// Where <hash> is derived from the repository URL.
//
// # Concurrency
//
// The backend uses Git's locking and push/pull mechanisms for concurrency control.
// On push conflicts, it automatically pulls and retries.
package gitlfs
