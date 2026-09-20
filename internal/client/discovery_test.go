package client

import (
	"context"
	"net/http"
	"testing"
)

func TestGetOrganizationProfile(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/organization", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodGet) || !requireAPIKey(t, w, r) {
			return
		}
		jsonResponse(w, http.StatusOK, APIResponse[OrganizationInfo]{
			Success:   true,
			RequestID: "req-organization",
			Data: OrganizationInfo{
				OrganizationID:   "o-1234567890abcdef",
				OrganizationName: "Example Organization",
				AWSExternalID:    "194819e8-e41d-afcb-7269-bcead94c1b55",
			},
		})
	})

	organization, err := client.GetOrganizationProfile(context.Background())
	if err != nil {
		t.Fatalf("GetOrganizationProfile: %v", err)
	}
	if organization.OrganizationID != "o-1234567890abcdef" {
		t.Errorf("unexpected organization ID %q", organization.OrganizationID)
	}
	if organization.AWSExternalID != "194819e8-e41d-afcb-7269-bcead94c1b55" {
		t.Errorf("unexpected AWS external ID %q", organization.AWSExternalID)
	}
}

func TestListCloudProviderRegions(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	mux.HandleFunc("/v1/cloud-providers/aws/regions", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(t, w, r, http.MethodGet) || !requireAPIKey(t, w, r) {
			return
		}
		if got := r.URL.Query().Get("deploymentMode"); got != "BYOC" {
			t.Errorf("expected deploymentMode=BYOC, got %q", got)
		}
		jsonResponse(w, http.StatusOK, APIResponse[[]CloudProviderRegion]{
			Success:   true,
			RequestID: "req-regions",
			Data: []CloudProviderRegion{{
				Region:                   "us-east-1",
				DisplayName:              "US East (N. Virginia)",
				EndpointServiceID:        "vpce-svc-00c31b6d080ccb272",
				EndpointServiceName:      "com.amazonaws.vpce.us-east-1.vpce-svc-00c31b6d080ccb272",
				MultiAZSupported:         true,
				SupportedDeploymentModes: []string{"BYOC"},
				Zones:                    []CloudProviderZone{{Zone: "us-east-1a", DisplayName: "US East (N. Virginia) A"}},
			}},
		})
	})

	regions, err := client.ListCloudProviderRegions(context.Background(), "aws", "BYOC")
	if err != nil {
		t.Fatalf("ListCloudProviderRegions: %v", err)
	}
	if len(regions) != 1 {
		t.Fatalf("expected one region, got %d", len(regions))
	}
	region := regions[0]
	if region.Region != "us-east-1" || region.EndpointServiceName == "" {
		t.Errorf("unexpected region: %#v", region)
	}
	if len(region.Zones) != 1 || region.Zones[0].Zone != "us-east-1a" {
		t.Errorf("unexpected zones: %#v", region.Zones)
	}
}
