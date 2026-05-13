package client

import (
	"context"
	"fmt"
	"net/url"
)

func clustersBasePath(warehouseID string) string {
	return fmt.Sprintf("/v1/warehouses/%s/clusters", warehouseID)
}

func clusterPath(warehouseID, clusterID string) string {
	return fmt.Sprintf("/v1/warehouses/%s/clusters/%s", warehouseID, clusterID)
}

// CreateCluster creates a new cluster in a warehouse.
func (c *FormationClient) CreateCluster(ctx context.Context, warehouseID string, req *CreateClusterRequest) (*CreateClusterResult, error) {
	resp, err := c.post(ctx, clustersBasePath(warehouseID), req)
	if err != nil {
		return nil, err
	}
	var result APIResponse[CreateClusterResult]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// GetCluster returns a single cluster with billingSummary and billingPools populated.
func (c *FormationClient) GetCluster(ctx context.Context, warehouseID, clusterID string) (*ClusterDetail, error) {
	resp, err := c.get(ctx, clusterPath(warehouseID, clusterID), nil)
	if err != nil {
		return nil, err
	}
	var result APIResponse[ClusterDetail]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// ListClusters returns a paginated list of clusters in a warehouse.
func (c *FormationClient) ListClusters(ctx context.Context, warehouseID string, opts *ListClustersOptions) (*PageResponse[ClusterItem], error) {
	q := url.Values{}
	if opts != nil {
		addPagination(q, opts.Page, opts.Size)
		if opts.Keyword != "" {
			q.Set("keyword", opts.Keyword)
		}
		if opts.Status != "" {
			q.Set("status", opts.Status)
		}
		if opts.ClusterType != "" {
			q.Set("clusterType", opts.ClusterType)
		}
		if opts.BillingModel != "" {
			q.Set("billingModel", opts.BillingModel)
		}
	}
	resp, err := c.get(ctx, clustersBasePath(warehouseID), q)
	if err != nil {
		return nil, err
	}
	var result PageResponse[ClusterItem]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateCluster updates a cluster.
func (c *FormationClient) UpdateCluster(ctx context.Context, warehouseID, clusterID string, req *UpdateClusterRequest) error {
	resp, err := c.patch(ctx, clusterPath(warehouseID, clusterID), req)
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}

// DeleteCluster deletes a cluster.
func (c *FormationClient) DeleteCluster(ctx context.Context, warehouseID, clusterID string) error {
	resp, err := c.delete(ctx, clusterPath(warehouseID, clusterID))
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}

// OperateCluster performs a cluster action (pause, resume, reboot) via POST /actions.
func (c *FormationClient) OperateCluster(ctx context.Context, warehouseID, clusterID, action string) error {
	req := &ClusterActionRequest{Action: action}
	resp, err := c.post(ctx, fmt.Sprintf("%s/actions", clusterPath(warehouseID, clusterID)), req)
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}

// RebootCluster reboots a cluster.
func (c *FormationClient) RebootCluster(ctx context.Context, warehouseID, clusterID string) error {
	return c.OperateCluster(ctx, warehouseID, clusterID, "reboot")
}

