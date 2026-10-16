package client

import (
	"context"
	"fmt"
	"net/url"
)

type CreateVirtualClusterRequest struct {
	Name             string `json:"name"`
	ActiveClusterID  string `json:"activeClusterId"`
	StandbyClusterID string `json:"standbyClusterId"`
}

type CreateVirtualClusterResult struct {
	VirtualClusterID string `json:"virtualClusterId"`
}

type RenameVirtualClusterRequest struct {
	NewName string `json:"newName"`
}

type UpdateVirtualClusterRequest struct {
	NewActiveClusterID  string `json:"newActiveClusterId"`
	NewStandbyClusterID string `json:"newStandbyClusterId"`
}

type VirtualCluster struct {
	ID               string   `json:"id"`
	WarehouseID      string   `json:"warehouseId"`
	Name             string   `json:"name"`
	Status           string   `json:"status"`
	ActiveClusterID  string   `json:"activeClusterId"`
	StandbyClusterID string   `json:"standbyClusterId"`
	AtRisk           bool     `json:"atRisk"`
	RiskReasons      []string `json:"riskReasons"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
}

func virtualClustersBasePath(warehouseID string) string {
	return "/v1/warehouses/" + url.PathEscape(warehouseID) + "/virtual-clusters"
}

func virtualClusterPath(warehouseID, virtualClusterID string) string {
	return virtualClustersBasePath(warehouseID) + "/" + url.PathEscape(virtualClusterID)
}

func (c *FormationClient) CreateVirtualCluster(ctx context.Context, warehouseID string, req *CreateVirtualClusterRequest) (*CreateVirtualClusterResult, error) {
	resp, err := c.post(ctx, virtualClustersBasePath(warehouseID), req)
	if err != nil {
		return nil, err
	}
	var result APIResponse[CreateVirtualClusterResult]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	if result.Data.VirtualClusterID == "" {
		return nil, withRequestID(fmt.Errorf("creating virtual cluster: response has no virtualClusterId"), resp.Request)
	}
	return &result.Data, nil
}

func (c *FormationClient) ListVirtualClusters(ctx context.Context, warehouseID string) ([]VirtualCluster, error) {
	resp, err := c.get(ctx, virtualClustersBasePath(warehouseID), nil)
	if err != nil {
		return nil, err
	}
	var result APIResponse[[]VirtualCluster]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	if result.Data == nil {
		return nil, fmt.Errorf("listing virtual clusters: response has no data array")
	}
	for _, cluster := range result.Data {
		if cluster.ID == "" || cluster.WarehouseID != warehouseID {
			return nil, fmt.Errorf("listing virtual clusters: response contains an invalid cluster identity")
		}
	}
	return result.Data, nil
}

func (c *FormationClient) GetVirtualCluster(ctx context.Context, warehouseID, virtualClusterID string) (*VirtualCluster, error) {
	resp, err := c.get(ctx, virtualClusterPath(warehouseID, virtualClusterID), nil)
	if err != nil {
		return nil, err
	}
	var result APIResponse[VirtualCluster]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	if result.Data.ID != virtualClusterID || result.Data.WarehouseID != warehouseID {
		return nil, fmt.Errorf("getting virtual cluster: response identity does not match requested cluster")
	}
	return &result.Data, nil
}

func (c *FormationClient) RenameVirtualCluster(ctx context.Context, warehouseID, virtualClusterID string, req *RenameVirtualClusterRequest) error {
	resp, err := c.post(ctx, virtualClusterPath(warehouseID, virtualClusterID)+"/rename", req)
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}

func (c *FormationClient) UpdateVirtualCluster(ctx context.Context, warehouseID, virtualClusterID string, req *UpdateVirtualClusterRequest) error {
	resp, err := c.post(ctx, virtualClusterPath(warehouseID, virtualClusterID)+"/update", req)
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}

func (c *FormationClient) DeleteVirtualCluster(ctx context.Context, warehouseID, virtualClusterID string) error {
	resp, err := c.delete(ctx, virtualClusterPath(warehouseID, virtualClusterID))
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}
