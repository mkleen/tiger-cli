package mcp

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/timescale/tiger-cli/internal/api"
	"github.com/timescale/tiger-cli/internal/api/mocks"
	"github.com/timescale/tiger-cli/internal/common"
	"github.com/timescale/tiger-cli/internal/config"
)

// withFetchExtensions stubs common.FetchExtensions — otherwise the point where
// the tool would open a real database connection — asserting the arguments
// the handler passed down and returning extensions and err in their place.
func withFetchExtensions(want common.FetchExtensionsArgs, extensions []common.Extension, err error) runOption {
	return withSetup(func(t *testing.T) {
		original := common.FetchExtensions
		common.FetchExtensions = func(_ context.Context, _ *config.Config, _ api.Service, got common.FetchExtensionsArgs) ([]common.Extension, error) {
			t.Helper()
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("FetchExtensions args mismatch (-want +got):\n%s", diff)
			}
			return extensions, err
		}
		t.Cleanup(func() { common.FetchExtensions = original })
	})
}

func TestDBExtensionsTool(t *testing.T) {
	args := map[string]any{"service_id": "e6ue9697jf"}

	setupGetWithStatus := func(status api.DeployStatus) func(m *mocks.MockClientWithResponsesInterface) {
		return func(m *mocks.MockClientWithResponsesInterface) {
			expectGetService(m, "e6ue9697jf", sampleService(func(s *api.Service) { s.Status = status }))
		}
	}

	// The fetch is stubbed for the success cases below, so they reach the
	// handler's own output without a live database.
	extensions := []common.Extension{
		{Name: "timescaledb", Version: "2.21.0", Schema: "public", Description: "Enables scalable inserts and complex queries for time-series data"},
		{Name: "timescaledb_toolkit", Version: "1.21.0", Schema: "public", Description: "Library of analytical hyperfunctions, time-series pipelining, and other SQL utilities"},
		{Name: "vector", Version: "0.8.0", Schema: "public"},
	}
	extensionsOutput := []any{
		map[string]any{"name": "timescaledb", "version": "2.21.0", "schema": "public", "description": "Enables scalable inserts and complex queries for time-series data"},
		map[string]any{"name": "timescaledb_toolkit", "version": "1.21.0", "schema": "public", "description": "Library of analytical hyperfunctions, time-series pipelining, and other SQL utilities"},
		map[string]any{"name": "vector", "version": "0.8.0", "schema": "public", "description": ""},
	}

	runToolTests(t, []toolTest{
		{
			name:    "not logged in",
			tool:    toolDBExtensions,
			args:    args,
			opts:    []runOption{withNotLoggedIn()},
			wantErr: notLoggedInMsg,
		},
		{
			// The service_id pattern is enforced by the SDK's schema validation,
			// before the handler runs, so no API call is registered.
			name:    "service id fails schema validation",
			tool:    toolDBExtensions,
			args:    map[string]any{"service_id": "not-an-id"},
			wantErr: `validating "arguments": validating root: validating /properties/service_id: pattern: "not-an-id" does not match regular expression "^[a-z0-9]{10}$"`,
		},
		{
			name: "service lookup network error",
			tool: toolDBExtensions,
			args: args,
			mock: func(m *mocks.MockClientWithResponsesInterface) {
				m.EXPECT().GetServiceWithResponse(validCtx, testProjectID, "e6ue9697jf").
					Return(nil, errors.New("connection refused"))
			},
			wantErr: "failed to fetch service details: connection refused",
		},
		{
			name: "service lookup API error",
			tool: toolDBExtensions,
			args: args,
			mock: func(m *mocks.MockClientWithResponsesInterface) {
				m.EXPECT().GetServiceWithResponse(validCtx, testProjectID, "e6ue9697jf").
					Return(&api.GetServiceResponse{
						HTTPResponse: httpResponse(http.StatusNotFound),
						JSON4XX:      &api.ClientError{Message: new("service not found")},
					}, nil)
			},
			wantErr: "service not found",
		},
		{
			name: "nil response body",
			tool: toolDBExtensions,
			args: args,
			mock: func(m *mocks.MockClientWithResponsesInterface) {
				m.EXPECT().GetServiceWithResponse(validCtx, testProjectID, "e6ue9697jf").
					Return(&api.GetServiceResponse{HTTPResponse: httpResponse(http.StatusOK)}, nil)
			},
			wantErr: "empty response from API",
		},
		{
			name: "read replica",
			tool: toolDBExtensions,
			args: map[string]any{"service_id": "u8me885b93"},
			mock: func(m *mocks.MockClientWithResponsesInterface) {
				expectGetService(m, "u8me885b93", sampleReplica())
			},
			wantErr: "'u8me885b93' is a read replica; list the extensions on its primary service 'e6ue9697jf' instead",
		},
		{
			name:    "service paused",
			tool:    toolDBExtensions,
			args:    args,
			mock:    setupGetWithStatus(api.DeployStatusPAUSED),
			wantErr: pausedMsg,
		},
		{
			name:    "service not ready",
			tool:    toolDBExtensions,
			args:    args,
			mock:    setupGetWithStatus(api.DeployStatusQUEUED),
			wantErr: notReadyMsg,
		},
		{
			name:    "ready service without an endpoint",
			tool:    toolDBExtensions,
			args:    args,
			mock:    setupGetWithStatus(api.DeployStatusREADY),
			wantErr: noEndpointMsg,
		},
		{
			// A fetch failure passes through handleDatabaseError unchanged.
			name:    "fetch fails",
			tool:    toolDBExtensions,
			args:    args,
			opts:    []runOption{withFetchExtensions(common.FetchExtensionsArgs{Role: "tsdbadmin"}, nil, errors.New("failed to connect to database: no route to host"))},
			mock:    setupGetWithStatus(api.DeployStatusREADY),
			wantErr: "failed to connect to database: no route to host",
		},
		{
			name:       "returns the extensions with the parameter defaults",
			tool:       toolDBExtensions,
			args:       args,
			opts:       []runOption{withFetchExtensions(common.FetchExtensionsArgs{Role: "tsdbadmin"}, extensions, nil)},
			mock:       setupGetWithStatus(api.DeployStatusREADY),
			wantOutput: map[string]any{"extensions": extensionsOutput},
		},
		{
			name:       "passes the role down to the fetch",
			tool:       toolDBExtensions,
			args:       map[string]any{"service_id": "e6ue9697jf", "role": "readonly"},
			opts:       []runOption{withFetchExtensions(common.FetchExtensionsArgs{Role: "readonly"}, extensions, nil)},
			mock:       setupGetWithStatus(api.DeployStatusREADY),
			wantOutput: map[string]any{"extensions": extensionsOutput},
		},
	})
}
