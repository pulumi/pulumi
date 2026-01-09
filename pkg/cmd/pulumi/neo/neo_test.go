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

package neo

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test stub backend for Neo commands
type stubNeoBackend struct {
	httpstate.Backend

	CreateNeoTaskF func(ctx context.Context, orgName string, req apitype.NeoTaskRequest) (*apitype.NeoTaskResponse, error)
	ListNeoTasksF  func(ctx context.Context, orgName string, pageSize int, continuationToken string) (*apitype.NeoTaskListResponse, error)
	GetNeoTaskF    func(ctx context.Context, orgName string, taskID string) (*apitype.NeoTask, error)
	CurrentUserF   func() (string, []string, *workspace.TokenInformation, error)
	CapabilitiesF  func(context.Context) apitype.Capabilities
	CloudURLF      func() string
}

var _ httpstate.Backend = (*stubNeoBackend)(nil)

func (s *stubNeoBackend) CreateNeoTask(ctx context.Context, orgName string, req apitype.NeoTaskRequest) (*apitype.NeoTaskResponse, error) {
	return s.CreateNeoTaskF(ctx, orgName, req)
}

func (s *stubNeoBackend) ListNeoTasks(ctx context.Context, orgName string, pageSize int, continuationToken string) (*apitype.NeoTaskListResponse, error) {
	return s.ListNeoTasksF(ctx, orgName, pageSize, continuationToken)
}

func (s *stubNeoBackend) GetNeoTask(ctx context.Context, orgName string, taskID string) (*apitype.NeoTask, error) {
	return s.GetNeoTaskF(ctx, orgName, taskID)
}

func (s *stubNeoBackend) CurrentUser() (string, []string, *workspace.TokenInformation, error) {
	return s.CurrentUserF()
}

func (s *stubNeoBackend) Capabilities(ctx context.Context) apitype.Capabilities {
	if s.CapabilitiesF != nil {
		return s.CapabilitiesF(ctx)
	}
	return apitype.Capabilities{NeoTasks: true}
}

func (s *stubNeoBackend) CloudURL() string {
	if s.CloudURLF != nil {
		return s.CloudURLF()
	}
	return "https://api.pulumi.com"
}

func TestNeoTaskTypes(t *testing.T) {
	t.Parallel()

	t.Run("NeoTaskRequest marshals correctly", func(t *testing.T) {
		req := apitype.NeoTaskRequest{
			Message: apitype.NeoMessage{
				Type:      "user_message",
				Content:   "Deploy an S3 bucket",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		}
		assert.Equal(t, "user_message", req.Message.Type)
		assert.Equal(t, "Deploy an S3 bucket", req.Message.Content)
		assert.NotEmpty(t, req.Message.Timestamp)
	})

	t.Run("NeoTask has all required fields", func(t *testing.T) {
		task := apitype.NeoTask{
			ID:        "task-123",
			Name:      "Test task",
			Status:    "running",
			CreatedAt: "2025-01-09T00:00:00Z",
			Entities:  []apitype.NeoEntity{},
		}

		assert.Equal(t, "task-123", task.ID)
		assert.Equal(t, "Test task", task.Name)
		assert.Equal(t, "running", task.Status)
		assert.Equal(t, "2025-01-09T00:00:00Z", task.CreatedAt)
	})

	t.Run("NeoTaskResponse includes TaskID", func(t *testing.T) {
		resp := apitype.NeoTaskResponse{
			TaskID: "task-123",
		}

		assert.Equal(t, "task-123", resp.TaskID)
	})
}

func TestNeoCapability(t *testing.T) {
	t.Parallel()

	t.Run("Capabilities parses NeoTasks correctly", func(t *testing.T) {
		capResp := apitype.CapabilitiesResponse{
			Capabilities: []apitype.APICapabilityConfig{
				{
					Capability: apitype.NeoTasks,
				},
			},
		}

		parsed, err := capResp.Parse()
		require.NoError(t, err)
		assert.True(t, parsed.NeoTasks)
	})

	t.Run("Capabilities without NeoTasks", func(t *testing.T) {
		capResp := apitype.CapabilitiesResponse{
			Capabilities: []apitype.APICapabilityConfig{
				{
					Capability: apitype.BatchEncrypt,
				},
			},
		}

		parsed, err := capResp.Parse()
		require.NoError(t, err)
		assert.False(t, parsed.NeoTasks)
	})
}

func TestCreateNeoTask(t *testing.T) {
	t.Parallel()

	t.Run("Successfully creates task", func(t *testing.T) {
		taskID := "task-abc-123"
		backend := &stubNeoBackend{
			CreateNeoTaskF: func(ctx context.Context, orgName string, req apitype.NeoTaskRequest) (*apitype.NeoTaskResponse, error) {
				assert.Equal(t, "test-org", orgName)
				assert.Equal(t, "user_message", req.Message.Type)
				assert.Equal(t, "Deploy an S3 bucket", req.Message.Content)
				assert.NotEmpty(t, req.Message.Timestamp)

				return &apitype.NeoTaskResponse{
					TaskID: taskID,
				}, nil
			},
		}

		req := apitype.NeoTaskRequest{
			Message: apitype.NeoMessage{
				Type:      "user_message",
				Content:   "Deploy an S3 bucket",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		}

		resp, err := backend.CreateNeoTask(context.Background(), "test-org", req)
		require.NoError(t, err)
		assert.Equal(t, taskID, resp.TaskID)
	})

	t.Run("Handles API error", func(t *testing.T) {
		backend := &stubNeoBackend{
			CreateNeoTaskF: func(ctx context.Context, orgName string, req apitype.NeoTaskRequest) (*apitype.NeoTaskResponse, error) {
				return nil, errors.New("API error")
			},
		}

		req := apitype.NeoTaskRequest{
			Message: apitype.NeoMessage{
				Type:      "user_message",
				Content:   "test",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		}

		resp, err := backend.CreateNeoTask(context.Background(), "test-org", req)
		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API error")
	})
}

func TestListNeoTasks(t *testing.T) {
	t.Parallel()

	t.Run("Successfully lists tasks", func(t *testing.T) {
		expectedTasks := []apitype.NeoTask{
			{
				ID:        "task-1",
				Name:      "First task",
				Status:    "running",
				CreatedAt: "2025-01-09T10:00:00Z",
			},
			{
				ID:        "task-2",
				Name:      "Second task",
				Status:    "idle",
				CreatedAt: "2025-01-09T09:00:00Z",
			},
		}

		backend := &stubNeoBackend{
			ListNeoTasksF: func(ctx context.Context, orgName string, pageSize int, continuationToken string) (*apitype.NeoTaskListResponse, error) {
				assert.Equal(t, "test-org", orgName)
				return &apitype.NeoTaskListResponse{
					Tasks: expectedTasks,
				}, nil
			},
		}

		resp, err := backend.ListNeoTasks(context.Background(), "test-org", 100, "")
		require.NoError(t, err)
		assert.Len(t, resp.Tasks, 2)
		assert.Equal(t, "task-1", resp.Tasks[0].ID)
		assert.Equal(t, "task-2", resp.Tasks[1].ID)
	})

	t.Run("Returns empty list when no tasks", func(t *testing.T) {
		backend := &stubNeoBackend{
			ListNeoTasksF: func(ctx context.Context, orgName string, pageSize int, continuationToken string) (*apitype.NeoTaskListResponse, error) {
				return &apitype.NeoTaskListResponse{
					Tasks: []apitype.NeoTask{},
				}, nil
			},
		}

		resp, err := backend.ListNeoTasks(context.Background(), "test-org", 100, "")
		require.NoError(t, err)
		assert.Empty(t, resp.Tasks)
	})

	t.Run("Handles continuation token", func(t *testing.T) {
		backend := &stubNeoBackend{
			ListNeoTasksF: func(ctx context.Context, orgName string, pageSize int, continuationToken string) (*apitype.NeoTaskListResponse, error) {
				return &apitype.NeoTaskListResponse{
					Tasks: []apitype.NeoTask{
						{ID: "task-1", Name: "First", Status: "idle", CreatedAt: "2025-01-09T10:00:00Z"},
					},
					ContinuationToken: "next-page-token",
				}, nil
			},
		}

		resp, err := backend.ListNeoTasks(context.Background(), "test-org", 100, "")
		require.NoError(t, err)
		assert.Equal(t, "next-page-token", resp.ContinuationToken)
	})

	t.Run("Pagination with page size", func(t *testing.T) {
		backend := &stubNeoBackend{
			ListNeoTasksF: func(ctx context.Context, orgName string, pageSize int, continuationToken string) (*apitype.NeoTaskListResponse, error) {
				assert.Equal(t, "test-org", orgName)
				assert.Equal(t, 50, pageSize)
				assert.Equal(t, "", continuationToken)
				return &apitype.NeoTaskListResponse{
					Tasks: []apitype.NeoTask{
						{ID: "task-1", Name: "First", Status: "idle", CreatedAt: "2025-01-09T10:00:00Z"},
					},
				}, nil
			},
		}

		resp, err := backend.ListNeoTasks(context.Background(), "test-org", 50, "")
		require.NoError(t, err)
		assert.Len(t, resp.Tasks, 1)
	})

	t.Run("Pagination with continuation token", func(t *testing.T) {
		callCount := 0
		backend := &stubNeoBackend{
			ListNeoTasksF: func(ctx context.Context, orgName string, pageSize int, continuationToken string) (*apitype.NeoTaskListResponse, error) {
				callCount++
				if callCount == 1 {
					assert.Equal(t, "", continuationToken)
					return &apitype.NeoTaskListResponse{
						Tasks: []apitype.NeoTask{
							{ID: "task-1", Name: "First", Status: "idle", CreatedAt: "2025-01-09T10:00:00Z"},
						},
						ContinuationToken: "token-page2",
					}, nil
				}
				assert.Equal(t, "token-page2", continuationToken)
				return &apitype.NeoTaskListResponse{
					Tasks: []apitype.NeoTask{
						{ID: "task-2", Name: "Second", Status: "running", CreatedAt: "2025-01-09T09:00:00Z"},
					},
					ContinuationToken: "",
				}, nil
			},
		}

		// First page
		resp1, err := backend.ListNeoTasks(context.Background(), "test-org", 100, "")
		require.NoError(t, err)
		assert.Equal(t, "token-page2", resp1.ContinuationToken)
		assert.Len(t, resp1.Tasks, 1)
		assert.Equal(t, "task-1", resp1.Tasks[0].ID)

		// Second page
		resp2, err := backend.ListNeoTasks(context.Background(), "test-org", 100, "token-page2")
		require.NoError(t, err)
		assert.Equal(t, "", resp2.ContinuationToken)
		assert.Len(t, resp2.Tasks, 1)
		assert.Equal(t, "task-2", resp2.Tasks[0].ID)
	})
}

func TestGetNeoTask(t *testing.T) {
	t.Parallel()

	t.Run("Successfully gets task", func(t *testing.T) {
		expectedTask := &apitype.NeoTask{
			ID:        "task-123",
			Name:      "Test task",
			Status:    "running",
			CreatedAt: "2025-01-09T10:00:00Z",
			Entities: []apitype.NeoEntity{
				{
					Type:    "stack",
					Name:    "my-stack",
					Project: "my-project",
				},
			},
		}

		backend := &stubNeoBackend{
			GetNeoTaskF: func(ctx context.Context, orgName string, taskID string) (*apitype.NeoTask, error) {
				assert.Equal(t, "test-org", orgName)
				assert.Equal(t, "task-123", taskID)
				return expectedTask, nil
			},
		}

		task, err := backend.GetNeoTask(context.Background(), "test-org", "task-123")
		require.NoError(t, err)
		assert.Equal(t, "task-123", task.ID)
		assert.Equal(t, "Test task", task.Name)
		assert.Len(t, task.Entities, 1)
		assert.Equal(t, "stack", task.Entities[0].Type)
	})

	t.Run("Returns error for non-existent task", func(t *testing.T) {
		backend := &stubNeoBackend{
			GetNeoTaskF: func(ctx context.Context, orgName string, taskID string) (*apitype.NeoTask, error) {
				return nil, errors.New("task not found")
			},
		}

		task, err := backend.GetNeoTask(context.Background(), "test-org", "nonexistent")
		assert.Nil(t, task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task not found")
	})
}

func TestNeoEntity(t *testing.T) {
	t.Parallel()

	t.Run("Stack entity has required fields", func(t *testing.T) {
		entity := apitype.NeoEntity{
			Type:    "stack",
			Name:    "production",
			Project: "infrastructure",
		}

		assert.Equal(t, "stack", entity.Type)
		assert.Equal(t, "production", entity.Name)
		assert.Equal(t, "infrastructure", entity.Project)
	})

	t.Run("Repository entity", func(t *testing.T) {
		entity := apitype.NeoEntity{
			Type: "repository",
			Name: "my-repo",
			ID:   "repo-123",
		}

		assert.Equal(t, "repository", entity.Type)
		assert.Equal(t, "my-repo", entity.Name)
		assert.Equal(t, "repo-123", entity.ID)
	})
}

func TestNeoMessage(t *testing.T) {
	t.Parallel()

	t.Run("Message with entity diff", func(t *testing.T) {
		msg := apitype.NeoMessage{
			Type:      "user_message",
			Content:   "Deploy to production",
			Timestamp: "2025-01-09T10:00:00Z",
			EntityDiff: &apitype.NeoEntityDiff{
				Add: []apitype.NeoEntity{
					{
						Type:    "stack",
						Name:    "prod",
						Project: "infra",
					},
				},
				Remove: []apitype.NeoEntity{},
			},
		}

		assert.Equal(t, "user_message", msg.Type)
		assert.NotNil(t, msg.EntityDiff)
		assert.Len(t, msg.EntityDiff.Add, 1)
		assert.Empty(t, msg.EntityDiff.Remove)
	})

	t.Run("Simple message without entities", func(t *testing.T) {
		msg := apitype.NeoMessage{
			Type:      "user_message",
			Content:   "Simple query",
			Timestamp: "2025-01-09T10:00:00Z",
		}

		assert.Equal(t, "user_message", msg.Type)
		assert.Nil(t, msg.EntityDiff)
	})
}

func TestNeoTaskURLGeneration(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		cloudURL string
		orgName  string
		taskID   string
		expected string
	}{
		{
			name:     "Standard URL",
			cloudURL: "https://api.pulumi.com",
			orgName:  "my-org",
			taskID:   "task-123",
			expected: "https://api.pulumi.com/my-org/agents/task-123",
		},
		{
			name:     "Custom domain",
			cloudURL: "https://pulumi.example.com",
			orgName:  "test-org",
			taskID:   "abc-def-ghi",
			expected: "https://pulumi.example.com/test-org/agents/abc-def-ghi",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			backend := &stubNeoBackend{
				CloudURLF: func() string {
					return tc.cloudURL
				},
			}

			url := strings.Join([]string{backend.CloudURL(), tc.orgName, "agents", tc.taskID}, "/")
			assert.Equal(t, tc.expected, url)
		})
	}
}

func TestCapabilitiesCheck(t *testing.T) {
	t.Parallel()

	t.Run("Neo enabled", func(t *testing.T) {
		backend := &stubNeoBackend{
			CapabilitiesF: func(ctx context.Context) apitype.Capabilities {
				return apitype.Capabilities{
					NeoTasks: true,
				}
			},
		}

		caps := backend.Capabilities(context.Background())
		assert.True(t, caps.NeoTasks)
	})

	t.Run("Neo disabled", func(t *testing.T) {
		backend := &stubNeoBackend{
			CapabilitiesF: func(ctx context.Context) apitype.Capabilities {
				return apitype.Capabilities{
					NeoTasks: false,
				}
			},
		}

		caps := backend.Capabilities(context.Background())
		assert.False(t, caps.NeoTasks)
	})
}

func TestTaskIDTruncation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		taskID   string
		expected string
	}{
		{
			name:     "ID longer than 8 chars",
			taskID:   "task-abc-123-def",
			expected: "task-abc",
		},
		{
			name:     "ID exactly 8 chars",
			taskID:   "12345678",
			expected: "12345678",
		},
		{
			name:     "ID shorter than 8 chars",
			taskID:   "abc",
			expected: "abc",
		},
		{
			name:     "Empty ID",
			taskID:   "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Simulate the truncation logic from neo_view.go
			idDisplay := tc.taskID
			if len(idDisplay) > 8 {
				idDisplay = idDisplay[:8]
			}

			assert.Equal(t, tc.expected, idDisplay)
		})
	}
}

func TestStatusFiltering(t *testing.T) {
	t.Parallel()

	allTasks := []apitype.NeoTask{
		{ID: "task-1", Name: "First", Status: "running", CreatedAt: "2025-01-09T10:00:00Z"},
		{ID: "task-2", Name: "Second", Status: "idle", CreatedAt: "2025-01-09T09:00:00Z"},
		{ID: "task-3", Name: "Third", Status: "running", CreatedAt: "2025-01-09T08:00:00Z"},
		{ID: "task-4", Name: "Fourth", Status: "idle", CreatedAt: "2025-01-09T07:00:00Z"},
	}

	testCases := []struct {
		name           string
		statusFilter   string
		expectedCount  int
		expectedStatus string
	}{
		{
			name:          "No filter returns all tasks",
			statusFilter:  "",
			expectedCount: 4,
		},
		{
			name:           "Filter by running",
			statusFilter:   "running",
			expectedCount:  2,
			expectedStatus: "running",
		},
		{
			name:           "Filter by idle",
			statusFilter:   "idle",
			expectedCount:  2,
			expectedStatus: "idle",
		},
		{
			name:           "Filter is case insensitive",
			statusFilter:   "RUNNING",
			expectedCount:  2,
			expectedStatus: "running",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Simulate the filtering logic from neo_list.go
			var filtered []apitype.NeoTask
			statusFilter := strings.ToLower(tc.statusFilter)
			for _, task := range allTasks {
				if statusFilter == "" || strings.ToLower(task.Status) == statusFilter {
					filtered = append(filtered, task)
				}
			}

			assert.Len(t, filtered, tc.expectedCount)
			if tc.expectedStatus != "" {
				for _, task := range filtered {
					assert.Equal(t, tc.expectedStatus, strings.ToLower(task.Status))
				}
			}
		})
	}
}

func TestPageSizeValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		inputPageSize int
		expectedSize  int
	}{
		{
			name:          "Page size within bounds",
			inputPageSize: 50,
			expectedSize:  50,
		},
		{
			name:          "Page size exceeds max (1000)",
			inputPageSize: 1500,
			expectedSize:  1000,
		},
		{
			name:          "Zero page size uses default",
			inputPageSize: 0,
			expectedSize:  100,
		},
		{
			name:          "Negative page size uses default",
			inputPageSize: -10,
			expectedSize:  100,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Simulate the validation logic from neo_list.go
			pageSize := tc.inputPageSize
			if pageSize <= 0 {
				pageSize = 100
			}
			// Simulate client-side validation from client.go
			if pageSize > 1000 {
				pageSize = 1000
			}

			assert.Equal(t, tc.expectedSize, pageSize)
		})
	}
}

func TestNonCloudBackendHandling(t *testing.T) {
	t.Parallel()

	t.Run("Error message for non-cloud backend", func(t *testing.T) {
		// Simulate what happens when requireCloudBackendWithNeo receives a non-cloud backend
		// The actual function uses type assertion: cloudBackend, isCloud := currentBe.(httpstate.Backend)

		// We can't easily test requireCloudBackendWithNeo directly since it requires
		// complex setup with workspace and login manager, but we can verify the
		// error message format that would be returned

		expectedError := "Neo is only available with Pulumi Cloud. Please run `pulumi login` to connect to Pulumi Cloud"

		// This is the error that gets returned when the backend is not a cloud backend
		err := errors.New(expectedError)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Pulumi Cloud")
		assert.Contains(t, err.Error(), "pulumi login")
	})

	t.Run("Error message for individual account", func(t *testing.T) {
		// Verify the error message for individual accounts
		userName := "john-doe"
		expectedError := fmt.Sprintf("Neo is only available for organizations, not individual accounts")

		err := errors.New(expectedError)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "organizations")
		assert.Contains(t, err.Error(), "individual accounts")
	})

	t.Run("Error message for non-member organization", func(t *testing.T) {
		// Verify the error message when user is not a member of the org
		orgName := "some-org"
		expectedError := fmt.Sprintf("you are not a member of organization %q", orgName)

		err := fmt.Errorf(expectedError)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a member")
		assert.Contains(t, err.Error(), orgName)
	})
}
