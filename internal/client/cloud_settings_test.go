package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestCloudSettingCredentialLifecycle(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	credential := CloudSettingCredential{
		CredentialID:            123,
		Name:                    "analytics-credential",
		CloudProvider:           "aws",
		Region:                  "us-east-1",
		BucketName:              "analytics-bucket",
		DataCredentialARN:       "arn:aws:iam::123456789012:role/data",
		DeploymentCredentialARN: "arn:aws:iam::123456789012:role/deploy",
		ExternalID:              "external-id",
		WarehouseCount:          1,
		WarehouseIDs:            []string{"WH-001"},
	}

	mux.HandleFunc("/v1/cloud-settings/aws/credentials", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req CreateCloudSettingCredentialRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decoding credential request: %v", err)
			}
			if req.BucketName != credential.BucketName || req.DataCredentialARN != credential.DataCredentialARN || req.DeploymentCredentialARN != credential.DeploymentCredentialARN {
				t.Fatalf("unexpected credential request: %#v", req)
			}
			jsonResponse(w, http.StatusCreated, APIResponse[CreateCloudSettingCredentialResult]{
				Success: true, RequestID: "req-create-credential",
				Data: CreateCloudSettingCredentialResult{CredentialID: credential.CredentialID},
			})
		case http.MethodGet:
			if r.URL.Query().Get("page") != "1" || r.URL.Query().Get("size") != "20" || r.URL.Query().Get("region") != credential.Region {
				t.Fatalf("unexpected credential list query: %s", r.URL.RawQuery)
			}
			jsonResponse(w, http.StatusOK, PageResponse[CloudSettingCredential]{
				Success: true, RequestID: "req-list-credentials", Data: []CloudSettingCredential{credential}, Page: 1, Size: 20, Total: 1,
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/cloud-settings/aws/credentials/123", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			jsonResponse(w, http.StatusOK, APIResponse[CloudSettingCredential]{Success: true, RequestID: "req-get-credential", Data: credential})
		case http.MethodDelete:
			jsonResponse(w, http.StatusOK, APIResponse[struct{}]{Success: true, RequestID: "req-delete-credential", Data: struct{}{}})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	created, err := client.CreateCloudSettingCredential(context.Background(), "aws", &CreateCloudSettingCredentialRequest{
		Name:                    credential.Name,
		Region:                  credential.Region,
		BucketName:              credential.BucketName,
		DataCredentialARN:       credential.DataCredentialARN,
		DeploymentCredentialARN: credential.DeploymentCredentialARN,
	})
	if err != nil || created.CredentialID != credential.CredentialID {
		t.Fatalf("CreateCloudSettingCredential: result=%#v error=%v", created, err)
	}

	got, err := client.GetCloudSettingCredential(context.Background(), "aws", credential.CredentialID)
	if err != nil || got.ExternalID != credential.ExternalID {
		t.Fatalf("GetCloudSettingCredential: result=%#v error=%v", got, err)
	}

	list, err := client.ListCloudSettingCredentials(context.Background(), "aws", &ListCloudSettingCredentialsOptions{Page: 1, Size: 20, Region: credential.Region})
	if err != nil || list.Total != 1 || len(list.Data) != 1 || list.Data[0].CredentialID != credential.CredentialID {
		t.Fatalf("ListCloudSettingCredentials: result=%#v error=%v", list, err)
	}

	if err := client.DeleteCloudSettingCredential(context.Background(), "aws", credential.CredentialID); err != nil {
		t.Fatalf("DeleteCloudSettingCredential: %v", err)
	}
}

func TestCloudSettingNetworkConfigLifecycle(t *testing.T) {
	ts, mux := newTestServer(t)
	defer ts.Close()
	client := newTestClient(t, ts)

	endpointID := "vpce-0123456789abcdef0"
	zoneMappings := []CloudSettingZoneMapping{
		{ZoneID: "us-east-1a", SubnetID: "subnet-0123456789abcdef0"},
		{ZoneID: "us-east-1b", SubnetID: "subnet-1123456789abcdef0"},
		{ZoneID: "us-east-1c", SubnetID: "subnet-2123456789abcdef0"},
	}
	network := CloudSettingNetworkConfig{
		NetworkConfigID: 456,
		Name:            "analytics-network",
		CloudProvider:   "aws",
		Region:          "us-east-1",
		VPCID:           "vpc-0123456789abcdef0",
		ZoneMappings:    zoneMappings,
		SecurityGroupID: "sg-0123456789abcdef0",
		EndpointID:      endpointID,
		WarehouseCount:  1,
		WarehouseIDs:    []string{"WH-001"},
	}

	mux.HandleFunc("/v1/cloud-settings/aws/network-configs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("reading network request: %v", err)
			}
			var req CreateCloudSettingNetworkConfigRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("decoding network request: %v", err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(body, &raw); err != nil {
				t.Fatalf("decoding raw network request: %v", err)
			}
			if _, ok := raw["subnetId"]; ok {
				t.Fatalf("unexpected root subnetId in request: %s", body)
			}
			if _, ok := raw["subnetIds"]; ok {
				t.Fatalf("unexpected root subnetIds in request: %s", body)
			}
			if req.CredentialID != 123 || (len(req.ZoneMappings) != 1 && len(req.ZoneMappings) != 3) || req.SecurityGroupID != network.SecurityGroupID || req.EndpointID == nil || *req.EndpointID != endpointID {
				t.Fatalf("unexpected network request: %#v", req)
			}
			for i, mapping := range req.ZoneMappings {
				if mapping != zoneMappings[i] {
					t.Fatalf("unexpected zone mapping %d: %#v", i, mapping)
				}
			}
			jsonResponse(w, http.StatusCreated, APIResponse[CreateCloudSettingNetworkConfigResult]{
				Success: true, RequestID: "req-create-network",
				Data: CreateCloudSettingNetworkConfigResult{NetworkConfigID: network.NetworkConfigID},
			})
		case http.MethodGet:
			if r.URL.Query().Get("page") != "1" || r.URL.Query().Get("size") != "20" || r.URL.Query().Get("region") != network.Region {
				t.Fatalf("unexpected network list query: %s", r.URL.RawQuery)
			}
			jsonResponse(w, http.StatusOK, PageResponse[CloudSettingNetworkConfig]{
				Success: true, RequestID: "req-list-networks", Data: []CloudSettingNetworkConfig{network}, Page: 1, Size: 20, Total: 1,
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/cloud-settings/aws/network-configs/456", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			jsonResponse(w, http.StatusOK, APIResponse[CloudSettingNetworkConfig]{Success: true, RequestID: "req-get-network", Data: network})
		case http.MethodDelete:
			jsonResponse(w, http.StatusOK, APIResponse[struct{}]{Success: true, RequestID: "req-delete-network", Data: struct{}{}})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	for _, count := range []int{1, 3} {
		created, err := client.CreateCloudSettingNetworkConfig(context.Background(), "aws", &CreateCloudSettingNetworkConfigRequest{
			Name:            network.Name,
			CredentialID:    123,
			ZoneMappings:    zoneMappings[:count],
			SecurityGroupID: network.SecurityGroupID,
			EndpointID:      &endpointID,
		})
		if err != nil || created.NetworkConfigID != network.NetworkConfigID {
			t.Fatalf("CreateCloudSettingNetworkConfig with %d mappings: result=%#v error=%v", count, created, err)
		}
	}

	got, err := client.GetCloudSettingNetworkConfig(context.Background(), "aws", network.NetworkConfigID)
	if err != nil || got.VPCID != network.VPCID || len(got.ZoneMappings) != 3 || got.ZoneMappings[2] != zoneMappings[2] {
		t.Fatalf("GetCloudSettingNetworkConfig: result=%#v error=%v", got, err)
	}

	list, err := client.ListCloudSettingNetworkConfigs(context.Background(), "aws", &ListCloudSettingNetworkConfigsOptions{Page: 1, Size: 20, Region: network.Region})
	if err != nil || list.Total != 1 || len(list.Data) != 1 || list.Data[0].NetworkConfigID != network.NetworkConfigID || len(list.Data[0].ZoneMappings) != 3 {
		t.Fatalf("ListCloudSettingNetworkConfigs: result=%#v error=%v", list, err)
	}

	if err := client.DeleteCloudSettingNetworkConfig(context.Background(), "aws", network.NetworkConfigID); err != nil {
		t.Fatalf("DeleteCloudSettingNetworkConfig: %v", err)
	}
}
