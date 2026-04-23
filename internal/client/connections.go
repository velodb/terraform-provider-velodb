package client

import (
	"context"
	"fmt"
)

// --- Types ---

type WarehouseAllowlistRule struct {
	CIDR        string `json:"cidr"`
	Description string `json:"description,omitempty"`
}

type WarehousePublicAccessPolicyRequest struct {
	PublicAccessPolicy string                   `json:"publicAccessPolicy"`
	Rules              []WarehouseAllowlistRule `json:"rules,omitempty"`
}

type WarehousePublicConnection struct {
	PublicAccessPolicy string                   `json:"publicAccessPolicy,omitempty"`
	Rules              []WarehouseAllowlistRule `json:"rules,omitempty"`
	SQLEndpoint        string                   `json:"sqlEndpoint,omitempty"`
	HTTPEndpoint       string                   `json:"httpEndpoint,omitempty"`
	ArrowFlightPort    int                      `json:"arrowFlightPort,omitempty"`
	StreamLoadEndpoint string                   `json:"streamLoadEndpoint,omitempty"`
}

// --- Methods ---

// GetWarehousePublicConnection returns public connection info including access policy.
func (c *FormationClient) GetWarehousePublicConnection(ctx context.Context, warehouseID string) (*WarehousePublicConnection, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/v1/warehouses/%s/connections/public", warehouseID), nil)
	if err != nil {
		return nil, err
	}
	var result APIResponse[WarehousePublicConnection]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// UpdateWarehousePublicAccessPolicy sets the public access policy (DENY_ALL, ALLOW_ALL, or ALLOWLIST_ONLY with rules).
func (c *FormationClient) UpdateWarehousePublicAccessPolicy(ctx context.Context, warehouseID string, req *WarehousePublicAccessPolicyRequest) error {
	resp, err := c.patch(ctx, fmt.Sprintf("/v1/warehouses/%s/connections/public/access-policy", warehouseID), req)
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}
