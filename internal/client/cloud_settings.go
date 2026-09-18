package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
)

const (
	cloudSettingDeleteRetryInterval = 2 * time.Second
	cloudSettingDeleteRetryTimeout  = 5 * time.Minute
)

type CreateCloudSettingCredentialRequest struct {
	Name                    string `json:"name"`
	Region                  string `json:"region"`
	BucketName              string `json:"bucketName"`
	DataCredentialARN       string `json:"dataCredentialArn"`
	DeploymentCredentialARN string `json:"deploymentCredentialArn"`
}

type CreateCloudSettingCredentialResult struct {
	CredentialID int64 `json:"credentialId"`
}

type CloudSettingCredential struct {
	CredentialID            int64      `json:"credentialId"`
	Name                    string     `json:"name"`
	CloudProvider           string     `json:"cloudProvider"`
	Region                  string     `json:"region"`
	BucketName              string     `json:"bucketName,omitempty"`
	DataCredentialARN       string     `json:"dataCredentialArn,omitempty"`
	DeploymentCredentialARN string     `json:"deploymentCredentialArn,omitempty"`
	ExternalID              string     `json:"externalId,omitempty"`
	WarehouseCount          int        `json:"warehouseCount"`
	WarehouseIDs            []string   `json:"warehouseIds,omitempty"`
	CreatedAt               *time.Time `json:"createdAt,omitempty"`
	UpdatedAt               *time.Time `json:"updatedAt,omitempty"`
}

type ListCloudSettingCredentialsOptions struct {
	Page   int
	Size   int
	Region string
}

type CloudSettingZoneMapping struct {
	ZoneID   string `json:"zoneId"`
	SubnetID string `json:"subnetId"`
}

type CreateCloudSettingNetworkConfigRequest struct {
	Name            string                    `json:"name"`
	CredentialID    int64                     `json:"credentialId"`
	ZoneMappings    []CloudSettingZoneMapping `json:"zoneMappings"`
	SecurityGroupID string                    `json:"securityGroupId"`
	EndpointID      *string                   `json:"endpointId,omitempty"`
}

type CreateCloudSettingNetworkConfigResult struct {
	NetworkConfigID int64 `json:"networkConfigId"`
}

type CloudSettingNetworkConfig struct {
	NetworkConfigID int64                     `json:"networkConfigId"`
	Name            string                    `json:"name"`
	CloudProvider   string                    `json:"cloudProvider"`
	Region          string                    `json:"region"`
	VPCID           string                    `json:"vpcId"`
	ZoneMappings    []CloudSettingZoneMapping `json:"zoneMappings"`
	SecurityGroupID string                    `json:"securityGroupId"`
	EndpointID      string                    `json:"endpointId,omitempty"`
	WarehouseCount  int                       `json:"warehouseCount"`
	WarehouseIDs    []string                  `json:"warehouseIds,omitempty"`
	CreatedAt       *time.Time                `json:"createdAt,omitempty"`
	UpdatedAt       *time.Time                `json:"updatedAt,omitempty"`
}

type ListCloudSettingNetworkConfigsOptions struct {
	Page   int
	Size   int
	Region string
}

func cloudSettingsPath(cloudProvider, kind string) string {
	return fmt.Sprintf("/v1/cloud-settings/%s/%s", url.PathEscape(cloudProvider), kind)
}

func (c *FormationClient) CreateCloudSettingCredential(ctx context.Context, cloudProvider string, req *CreateCloudSettingCredentialRequest) (*CreateCloudSettingCredentialResult, error) {
	resp, err := c.post(ctx, cloudSettingsPath(cloudProvider, "credentials"), req)
	if err != nil {
		return nil, err
	}
	var result APIResponse[CreateCloudSettingCredentialResult]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (c *FormationClient) GetCloudSettingCredential(ctx context.Context, cloudProvider string, credentialID int64) (*CloudSettingCredential, error) {
	resp, err := c.get(ctx, fmt.Sprintf("%s/%d", cloudSettingsPath(cloudProvider, "credentials"), credentialID), nil)
	if err != nil {
		return nil, err
	}
	var result APIResponse[CloudSettingCredential]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (c *FormationClient) ListCloudSettingCredentials(ctx context.Context, cloudProvider string, opts *ListCloudSettingCredentialsOptions) (*PageResponse[CloudSettingCredential], error) {
	query := url.Values{}
	if opts != nil {
		addPagination(query, opts.Page, opts.Size)
		if opts.Region != "" {
			query.Set("region", opts.Region)
		}
	}
	resp, err := c.get(ctx, cloudSettingsPath(cloudProvider, "credentials"), query)
	if err != nil {
		return nil, err
	}
	var result PageResponse[CloudSettingCredential]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *FormationClient) DeleteCloudSettingCredential(ctx context.Context, cloudProvider string, credentialID int64) error {
	return c.deleteCloudSetting(ctx, fmt.Sprintf("%s/%d", cloudSettingsPath(cloudProvider, "credentials"), credentialID), "CredentialInUse")
}

func (c *FormationClient) CreateCloudSettingNetworkConfig(ctx context.Context, cloudProvider string, req *CreateCloudSettingNetworkConfigRequest) (*CreateCloudSettingNetworkConfigResult, error) {
	resp, err := c.post(ctx, cloudSettingsPath(cloudProvider, "network-configs"), req)
	if err != nil {
		return nil, err
	}
	var result APIResponse[CreateCloudSettingNetworkConfigResult]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (c *FormationClient) GetCloudSettingNetworkConfig(ctx context.Context, cloudProvider string, networkConfigID int64) (*CloudSettingNetworkConfig, error) {
	resp, err := c.get(ctx, fmt.Sprintf("%s/%d", cloudSettingsPath(cloudProvider, "network-configs"), networkConfigID), nil)
	if err != nil {
		return nil, err
	}
	var result APIResponse[CloudSettingNetworkConfig]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (c *FormationClient) ListCloudSettingNetworkConfigs(ctx context.Context, cloudProvider string, opts *ListCloudSettingNetworkConfigsOptions) (*PageResponse[CloudSettingNetworkConfig], error) {
	query := url.Values{}
	if opts != nil {
		addPagination(query, opts.Page, opts.Size)
		if opts.Region != "" {
			query.Set("region", opts.Region)
		}
	}
	resp, err := c.get(ctx, cloudSettingsPath(cloudProvider, "network-configs"), query)
	if err != nil {
		return nil, err
	}
	var result PageResponse[CloudSettingNetworkConfig]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *FormationClient) DeleteCloudSettingNetworkConfig(ctx context.Context, cloudProvider string, networkConfigID int64) error {
	return c.deleteCloudSetting(ctx, fmt.Sprintf("%s/%d", cloudSettingsPath(cloudProvider, "network-configs"), networkConfigID), "NetworkConfigInUse")
}

func (c *FormationClient) deleteCloudSetting(ctx context.Context, path, inUseCode string) error {
	ctx, cancel := context.WithTimeout(ctx, cloudSettingDeleteRetryTimeout)
	defer cancel()

	return retryCloudSettingDelete(ctx, inUseCode, cloudSettingDeleteRetryInterval, func(ctx context.Context) error {
		resp, err := c.delete(ctx, path)
		if err != nil {
			return err
		}
		return parseResponse[any](resp, nil)
	})
}

func retryCloudSettingDelete(ctx context.Context, inUseCode string, interval time.Duration, deleteFn func(context.Context) error) error {
	for {
		err := deleteFn(ctx)
		var apiErr *APIError
		if err == nil || !errors.As(err, &apiErr) || apiErr.Code != inUseCode {
			return err
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("timed out waiting for cloud setting to become unused: %w", err)
		case <-timer.C:
		}
	}
}
