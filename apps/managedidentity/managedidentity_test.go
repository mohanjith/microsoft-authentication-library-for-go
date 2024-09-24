// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package managedidentity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/errors"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/internal/mock"
)

const (
	// test Resources
	resource              = "https://demo.azure.com"
	resourceDefaultSuffix = "https://demo.azure.com/.default"

	token = "fakeToken"
)

type SuccessfulResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresOn   int64  `json:"expires_on"`
	Resource    string `json:"resource"`
	TokenType   string `json:"token_type"`
}

type ErrorRespone struct {
	Err  string `json:"error"`
	Desc string `json:"error_description"`
}

func getSuccessfulResponse(resource string) ([]byte, error) {
	expiresOn := time.Now().Add(1 * time.Hour).Unix()
	response := SuccessfulResponse{
		AccessToken: token,
		ExpiresOn:   expiresOn,
		Resource:    resource,
		TokenType:   "Bearer",
	}
	jsonResponse, err := json.Marshal(response)
	return jsonResponse, err
}

func makeResponseWithErrorData(err string, desc string) ([]byte, error) {
	responseBody := ErrorRespone{
		Err:  err,
		Desc: desc,
	}
	jsonResponse, e := json.Marshal(responseBody)
	return jsonResponse, e
}

type resourceTestData struct {
	source   Source
	endpoint string
	resource string
	miType   ID
}

type errorTestData struct {
	code          int
	err           string
	desc          string
	correlationid string
}

func Test_SystemAssigned_Returns_AcquireToken_Failure(t *testing.T) {
	testCases := []errorTestData{
		{code: http.StatusNotFound,
			err:           "",
			desc:          "",
			correlationid: "121212"},
		{code: http.StatusNotImplemented,
			err:           "",
			desc:          "",
			correlationid: "121212"},
		{code: http.StatusServiceUnavailable,
			err:           "",
			desc:          "",
			correlationid: "121212"},
		{code: http.StatusBadRequest,
			err:           "invalid_request",
			desc:          "Identity not found",
			correlationid: "121212",
		},
	}

	for _, testCase := range testCases {
		t.Run(http.StatusText(testCase.code), func(t *testing.T) {
			fakeErrorClient := mock.Client{}
			responseBody, err := makeResponseWithErrorData(testCase.err, testCase.desc)
			if err != nil {
				t.Fatalf("error while forming json response : %s", err.Error())
			}
			fakeErrorClient.AppendResponse(mock.WithHTTPStatusCode(testCase.code),
				mock.WithBody(responseBody))
			client, err := New(SystemAssigned(), WithHTTPClient(&fakeErrorClient))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := client.AcquireToken(context.Background(), resource)
			if err == nil {
				t.Fatalf("should have encountered the error")
			}
			var callErr errors.CallErr
			if errors.As(err, &callErr) {
				if !strings.Contains(err.Error(), testCase.err) {
					t.Fatalf("expected message '%s' in error, got %q", testCase.err, callErr.Error())
				}
				if callErr.Resp.StatusCode != testCase.code {
					t.Fatalf("expected status code %d, got %d", testCase.code, callErr.Resp.StatusCode)
				}
			} else {
				t.Fatalf("expected error of type %T, got %T", callErr, err)
			}
			if resp.AccessToken != "" {
				t.Fatalf("accesstoken should be empty")
			}
		})
	}
}

func Test_SystemAssigned_Returns_Token_Success(t *testing.T) {
	testCases := []resourceTestData{
		{source: DefaultToIMDS, endpoint: imdsEndpoint, resource: resource, miType: SystemAssigned()},
		{source: DefaultToIMDS, endpoint: imdsEndpoint, resource: resourceDefaultSuffix, miType: SystemAssigned()},
		{source: DefaultToIMDS, endpoint: imdsEndpoint, resource: resource, miType: UserAssignedClientID("clientId")},
		{source: DefaultToIMDS, endpoint: imdsEndpoint, resource: resourceDefaultSuffix, miType: UserAssignedResourceID("resourceId")},
		{source: DefaultToIMDS, endpoint: imdsEndpoint, resource: resourceDefaultSuffix, miType: UserAssignedObjectID("objectId")},
	}
	for _, testCase := range testCases {

		t.Run(string(testCase.source), func(t *testing.T) {
			var localUrl *url.URL
			mockClient := mock.Client{}
			responseBody, err := getSuccessfulResponse(resource)
			if err != nil {
				t.Fatalf("error while forming json response : %s", err.Error())
			}
			mockClient.AppendResponse(mock.WithHTTPStatusCode(http.StatusOK), mock.WithBody(responseBody), mock.WithCallback(func(r *http.Request) {
				localUrl = r.URL
			}))
			client, err := New(testCase.miType, WithHTTPClient(&mockClient))

			if err != nil {
				t.Fatal(err)
			}
			result, err := client.AcquireToken(context.Background(), testCase.resource)
			if !strings.HasPrefix(localUrl.String(), testCase.endpoint) {
				t.Fatalf("url request is not on %s got %s", testCase.endpoint, localUrl)
			}
			if !strings.Contains(localUrl.String(), testCase.miType.value()) {
				t.Fatalf("url request does not contain the %s got %s", testCase.endpoint, localUrl)
			}
			query := localUrl.Query()

			if query.Get(apiVersionQuerryParameterName) != imdsAPIVersion {
				t.Fatalf("api-version not on %s got %s", imdsAPIVersion, query.Get(apiVersionQuerryParameterName))
			}
			if query.Get(resourceQuerryParameterName) != strings.TrimSuffix(testCase.resource, "/.default") {
				t.Fatal("suffix /.default was not removed.")
			}
			switch i := testCase.miType.(type) {
			case UserAssignedClientID:
				if query.Get(miQueryParameterClientId) != i.value() {
					t.Fatalf("resource client-id is incorrect, wanted %s got %s", i.value(), query.Get(miQueryParameterClientId))
				}
			case UserAssignedResourceID:
				if query.Get(miQueryParameterResourceId) != i.value() {
					t.Fatalf("resource resource-id is incorrect, wanted %s got %s", i.value(), query.Get(miQueryParameterResourceId))
				}
			case UserAssignedObjectID:
				if query.Get(miQueryParameterObjectId) != i.value() {
					t.Fatalf("resource objectiid is incorrect, wanted %s got %s", i.value(), query.Get(miQueryParameterObjectId))
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.AccessToken != token {
				t.Fatalf("wanted %q, got %q", token, result.AccessToken)
			}

		})
	}

	// Testing createIMDSAuthRequest
	tests := []struct {
		name     string
		id       ID
		resource string
		claims   string
		wantErr  bool
	}{
		{
			name:     "System Assigned",
			id:       SystemAssigned(),
			resource: "https://management.azure.com",
		},
		{
			name:     "System Assigned",
			id:       SystemAssigned(),
			resource: "https://management.azure.com/.default",
		},
		{
			name:     "Client ID",
			id:       UserAssignedClientID("test-client-id"),
			resource: "https://storage.azure.com",
		},
		{
			name:     "Resource ID",
			id:       UserAssignedResourceID("test-resource-id"),
			resource: "https://vault.azure.net",
		},
		{
			name:     "Object ID",
			id:       UserAssignedObjectID("test-object-id"),
			resource: "https://graph.microsoft.com",
		},
		{
			name:     "With Claims",
			id:       SystemAssigned(),
			resource: "https://management.azure.com",
			claims:   "test-claims",
		},
	}
	// testing IMDSAuthRequest Creation method.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := createIMDSAuthRequest(context.Background(), tt.id, tt.resource, tt.claims)
			if tt.wantErr {
				if err == nil {
					t.Fatal(err)
				}
				return
			}
			if req == nil {
				t.Fatal("createIMDSAuthRequest() returned nil request")
			}
			if req.Method != http.MethodGet {
				t.Fatal("createIMDSAuthRequest() method is not GET")
			}
			if got := req.URL.String(); !strings.HasPrefix(got, imdsEndpoint) {
				t.Fatalf("wanted %q, got %q", imdsEndpoint, got)
			}
			query := req.URL.Query()

			if query.Get(apiVersionQuerryParameterName) != "2018-02-01" {
				t.Fatal("createIMDSAuthRequest() api-version missmatch")
			}
			if query.Get(resourceQuerryParameterName) != strings.TrimSuffix(tt.resource, "/.default") {
				t.Fatal("createIMDSAuthRequest() resource does not ahve suffix removed ")
			}
			switch i := tt.id.(type) {
			case UserAssignedClientID:
				if query.Get(miQueryParameterClientId) != i.value() {
					t.Fatal("createIMDSAuthRequest() resource client-id is incorrect")
				}
			case UserAssignedResourceID:
				if query.Get(miQueryParameterResourceId) != i.value() {
					t.Fatal("createIMDSAuthRequest() resource resource-id is incorrect")
				}
			case UserAssignedObjectID:
				if query.Get(miQueryParameterObjectId) != i.value() {
					t.Fatal("createIMDSAuthRequest() resource objectiid is incorrect")
				}
			case systemAssignedValue: // not adding anything
			default:
				t.Fatal("createIMDSAuthRequest() unsupported type")

			}

		})
	}

}

func TestCreatingIMDSClient(t *testing.T) {
	tests := []struct {
		name    string
		id      ID
		wantErr bool
	}{
		{
			name: "System Assigned",
			id:   SystemAssigned(),
		},
		{
			name: "Client ID",
			id:   UserAssignedClientID("test-client-id"),
		},
		{
			name: "Resource ID",
			id:   UserAssignedResourceID("test-resource-id"),
		},
		{
			name: "Object ID",
			id:   UserAssignedObjectID("test-object-id"),
		},
		{
			name:    "Empty Client ID",
			id:      UserAssignedClientID(""),
			wantErr: true,
		},
		{
			name:    "Empty Resource ID",
			id:      UserAssignedResourceID(""),
			wantErr: true,
		},
		{
			name:    "Empty Object ID",
			id:      UserAssignedObjectID(""),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.id)
			if tt.wantErr {
				if err == nil {
					t.Fatal("client New() should return a error but did not.")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if client.miType.value() != tt.id.value() {
				t.Fatal("client New() did not assign a correct value to type.")
			}
		})
	}
}
