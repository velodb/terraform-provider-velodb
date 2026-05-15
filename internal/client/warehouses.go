package client

import (
	"context"
	"fmt"
	"net/url"
)

const warehousesBasePath = "/v1/warehouses"

// CreateWarehouse creates a new warehouse.
func (c *FormationClient) CreateWarehouse(ctx context.Context, req *CreateWarehouseRequest) (*CreateWarehouseResult, error) {
	resp, err := c.post(ctx, warehousesBasePath, req)
	if err != nil {
		return nil, err
	}
	var result APIResponse[CreateWarehouseResult]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// GetWarehouse returns a single warehouse by ID.
func (c *FormationClient) GetWarehouse(ctx context.Context, warehouseID string) (*WarehouseItem, error) {
	resp, err := c.get(ctx, fmt.Sprintf("%s/%s", warehousesBasePath, warehouseID), nil)
	if err != nil {
		return nil, err
	}
	var result APIResponse[WarehouseItem]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// ListWarehouses returns a paginated list of warehouses.
func (c *FormationClient) ListWarehouses(ctx context.Context, opts *ListWarehousesOptions) (*PageResponse[WarehouseItem], error) {
	q := url.Values{}
	if opts != nil {
		addPagination(q, opts.Page, opts.Size)
		if opts.WarehouseID != "" {
			q.Set("warehouseId", opts.WarehouseID)
		}
		if opts.Name != "" {
			q.Set("name", opts.Name)
		}
		if opts.CloudProvider != "" {
			q.Set("cloudProvider", opts.CloudProvider)
		}
		if opts.Region != "" {
			q.Set("region", opts.Region)
		}
		if opts.DeploymentMode != "" {
			q.Set("deploymentMode", opts.DeploymentMode)
		}
	}
	resp, err := c.get(ctx, warehousesBasePath, q)
	if err != nil {
		return nil, err
	}
	var result PageResponse[WarehouseItem]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateWarehouse updates a warehouse's basic info (name, maintenance window).
func (c *FormationClient) UpdateWarehouse(ctx context.Context, warehouseID string, req *UpdateWarehouseRequest) error {
	resp, err := c.patch(ctx, fmt.Sprintf("%s/%s", warehousesBasePath, warehouseID), req)
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}

// DeleteWarehouse deletes a warehouse.
func (c *FormationClient) DeleteWarehouse(ctx context.Context, warehouseID string) error {
	resp, err := c.delete(ctx, fmt.Sprintf("%s/%s", warehousesBasePath, warehouseID))
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}

// UpgradeWarehouse triggers a warehouse version upgrade.
func (c *FormationClient) UpgradeWarehouse(ctx context.Context, warehouseID string, targetVersionID int64) error {
	resp, err := c.post(ctx, fmt.Sprintf("%s/%s/settings/upgrade", warehousesBasePath, warehouseID), &UpgradeWarehouseRequest{
		TargetVersionID: targetVersionID,
	})
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}

// ListWarehouseVersions returns valid upgrade target versions for a warehouse.
func (c *FormationClient) ListWarehouseVersions(ctx context.Context, warehouseID string) ([]WarehouseVersion, error) {
	resp, err := c.get(ctx, fmt.Sprintf("%s/%s/versions", warehousesBasePath, warehouseID), nil)
	if err != nil {
		return nil, err
	}
	var result APIResponse[[]WarehouseVersion]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

// ChangeWarehousePassword changes the warehouse admin password.
func (c *FormationClient) ChangeWarehousePassword(ctx context.Context, warehouseID string, newPassword string) error {
	resp, err := c.post(ctx, fmt.Sprintf("%s/%s/settings/password", warehousesBasePath, warehouseID), &ChangePasswordRequest{
		NewPassword: newPassword,
	})
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}

// GetWarehouseConnections is defined in connections.go.
