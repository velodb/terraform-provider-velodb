package client

import (
	"context"
	"fmt"
)

// --- Public connection (WarehousePublicConnection in API) ---

type WarehousePublicConnection struct {
	Host               string                   `json:"host,omitempty"`
	JdbcURL            string                   `json:"jdbcUrl,omitempty"`
	HTTPURL            string                   `json:"httpUrl,omitempty"`
	JdbcPort           *int                     `json:"jdbcPort,omitempty"`
	HTTPPort           *int                     `json:"httpPort,omitempty"`
	StreamLoadPort     *int                     `json:"streamLoadPort,omitempty"`
	AdbcPort           *int                     `json:"adbcPort,omitempty"`
	StudioPort         *int                     `json:"studioPort,omitempty"`
	PublicAccessPolicy string                   `json:"publicAccessPolicy,omitempty"`
	Allowlist          []WarehouseAllowlistRule `json:"allowlist,omitempty"`
	ObserverGroups     []ObserverGroup          `json:"observerGroups,omitempty"`
}

type ObserverGroup struct {
	ClusterID string `json:"clusterId,omitempty"`
	Name      string `json:"name,omitempty"`
	JdbcPort  *int   `json:"jdbcPort,omitempty"`
}

// --- Public access policy ---

type WarehouseAllowlistRule struct {
	CIDR        string `json:"cidr"`
	Description string `json:"description,omitempty"`
}

type WarehousePublicAccessPolicyRequest struct {
	PublicAccessPolicy string                   `json:"publicAccessPolicy"`
	Rules              []WarehouseAllowlistRule `json:"rules,omitempty"`
}

type WarehousePublicAccessPolicyResponse struct {
	PublicAccessPolicy string                   `json:"publicAccessPolicy,omitempty"`
	Allowlist          []WarehouseAllowlistRule `json:"allowlist,omitempty"`
	Rules              []WarehouseAllowlistRule `json:"rules,omitempty"`
}

// --- Combined connections (GET /connections) ---

type WarehouseConnections struct {
	PublicEndpoints  []ConnectionEndpoint        `json:"publicEndpoints,omitempty"`
	PrivateEndpoints []PrivateConnectionEndpoint `json:"privateEndpoints,omitempty"`
	ComputeClusters  []ConnectionCluster         `json:"computeClusters,omitempty"`
	ObserverGroups   []ObserverGroup             `json:"observerGroups,omitempty"`
}

type ConnectionEndpoint struct {
	Protocol string `json:"protocol,omitempty"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	URL      string `json:"url,omitempty"`
}

type PrivateConnectionEndpoint struct {
	ConnectionEndpoint
	EndpointID string `json:"endpointId,omitempty"`
}

type ConnectionCluster struct {
	ClusterID   string `json:"clusterId,omitempty"`
	ClusterName string `json:"clusterName,omitempty"`
	HTTPPort    int    `json:"httpPort,omitempty"`
}

// --- Methods ---

// GetWarehouseConnections returns the combined public + private connection view.
func (c *FormationClient) GetWarehouseConnections(ctx context.Context, warehouseID string) (*WarehouseConnections, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/v1/warehouses/%s/connections", warehouseID), nil)
	if err != nil {
		return nil, err
	}
	var result APIResponse[WarehouseConnections]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

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

// GetWarehousePublicAccessPolicy returns the public access policy.
func (c *FormationClient) GetWarehousePublicAccessPolicy(ctx context.Context, warehouseID string) (*WarehousePublicAccessPolicyResponse, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/v1/warehouses/%s/connections/public/access-policy", warehouseID), nil)
	if err != nil {
		return nil, err
	}
	var result APIResponse[WarehousePublicAccessPolicyResponse]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// UpdateWarehousePublicAccessPolicy sets the public access policy.
func (c *FormationClient) UpdateWarehousePublicAccessPolicy(ctx context.Context, warehouseID string, req *WarehousePublicAccessPolicyRequest) error {
	resp, err := c.patch(ctx, fmt.Sprintf("/v1/warehouses/%s/connections/public/access-policy", warehouseID), req)
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}

// RegisterWarehousePrivateEndpoint registers an existing cloud private endpoint for a warehouse.
func (c *FormationClient) RegisterWarehousePrivateEndpoint(ctx context.Context, warehouseID string, req *RegisterWarehousePrivateEndpointRequest) error {
	resp, err := c.post(ctx, fmt.Sprintf("/v1/private-link/warehouses/%s/endpoints", warehouseID), req)
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}
