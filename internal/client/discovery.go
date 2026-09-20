package client

import (
	"context"
	"fmt"
	"net/url"
)

type OrganizationInfo struct {
	OrganizationID   string `json:"organizationId"`
	OrganizationName string `json:"organizationName,omitempty"`
	Description      string `json:"description,omitempty"`
	AWSExternalID    string `json:"awsExternalId"`
}

type CloudProviderZone struct {
	Zone        string `json:"zone"`
	DisplayName string `json:"displayName,omitempty"`
}

type CloudProviderRegion struct {
	Region                   string              `json:"region"`
	DisplayName              string              `json:"displayName,omitempty"`
	EndpointServiceID        string              `json:"endpointServiceId"`
	EndpointServiceName      string              `json:"endpointServiceName"`
	MultiAZSupported         bool                `json:"multiAzSupported"`
	SupportedDeploymentModes []string            `json:"supportedDeploymentModes,omitempty"`
	Zones                    []CloudProviderZone `json:"zones,omitempty"`
}

func (c *FormationClient) GetOrganizationProfile(ctx context.Context) (*OrganizationInfo, error) {
	resp, err := c.get(ctx, "/v1/organization", nil)
	if err != nil {
		return nil, err
	}
	var result APIResponse[OrganizationInfo]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (c *FormationClient) ListCloudProviderRegions(ctx context.Context, cloudProvider, deploymentMode string) ([]CloudProviderRegion, error) {
	query := url.Values{}
	if deploymentMode != "" {
		query.Set("deploymentMode", deploymentMode)
	}
	resp, err := c.get(ctx, fmt.Sprintf("/v1/cloud-providers/%s/regions", url.PathEscape(cloudProvider)), query)
	if err != nil {
		return nil, err
	}
	var result APIResponse[[]CloudProviderRegion]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}
