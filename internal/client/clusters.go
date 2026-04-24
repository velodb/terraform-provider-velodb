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

// PauseCluster pauses a cluster.
func (c *FormationClient) PauseCluster(ctx context.Context, warehouseID, clusterID string) error {
	resp, err := c.post(ctx, fmt.Sprintf("%s/pause", clusterPath(warehouseID, clusterID)), nil)
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}

// ResumeCluster resumes a paused cluster.
func (c *FormationClient) ResumeCluster(ctx context.Context, warehouseID, clusterID string) error {
	resp, err := c.post(ctx, fmt.Sprintf("%s/resume", clusterPath(warehouseID, clusterID)), nil)
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}

// RebootCluster reboots a cluster.
func (c *FormationClient) RebootCluster(ctx context.Context, warehouseID, clusterID string) error {
	resp, err := c.post(ctx, fmt.Sprintf("%s/reboot", clusterPath(warehouseID, clusterID)), nil)
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}

// OperateCluster performs a cluster action by name (kept for backward compatibility).
// Dispatches to PauseCluster, ResumeCluster, or RebootCluster.
func (c *FormationClient) OperateCluster(ctx context.Context, warehouseID, clusterID, action string) error {
	switch action {
	case "pause":
		return c.PauseCluster(ctx, warehouseID, clusterID)
	case "resume":
		return c.ResumeCluster(ctx, warehouseID, clusterID)
	case "reboot":
		return c.RebootCluster(ctx, warehouseID, clusterID)
	default:
		return fmt.Errorf("unknown cluster action: %s (valid: pause, resume, reboot)", action)
	}
}

// RenewCluster renews a subscription cluster.
func (c *FormationClient) RenewCluster(ctx context.Context, warehouseID, clusterID string, req *RenewClusterRequest) error {
	resp, err := c.post(ctx, fmt.Sprintf("%s/renew", clusterPath(warehouseID, clusterID)), req)
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}

// ConvertClusterToSubscription converts a cluster to subscription billing.
func (c *FormationClient) ConvertClusterToSubscription(ctx context.Context, warehouseID, clusterID string, req *ConvertToSubscriptionRequest) error {
	resp, err := c.post(ctx, fmt.Sprintf("%s/convert-to-subscription", clusterPath(warehouseID, clusterID)), req)
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}
