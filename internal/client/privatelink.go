package client

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// --- Types ---

type CreatePrivateLinkEndpointServiceRequest struct {
	CloudProvider       string  `json:"cloudProvider"`
	Region              string  `json:"region"`
	EndpointServiceID   string  `json:"endpointServiceId"`
	EndpointServiceName string  `json:"endpointServiceName"`
	Description         *string `json:"description,omitempty"`
}

type PrivateLinkEndpointService struct {
	CloudProvider       string     `json:"cloudProvider,omitempty"`
	Region              string     `json:"region,omitempty"`
	Zone                string     `json:"zone,omitempty"`
	EndpointServiceID   string     `json:"endpointServiceId,omitempty"`
	EndpointServiceName string     `json:"endpointServiceName,omitempty"`
	ProviderAccountID   string     `json:"providerAccountId,omitempty"`
	Description         string     `json:"description,omitempty"`
	CreatedAt           *time.Time `json:"createdAt,omitempty"`
	Connected           bool       `json:"connected,omitempty"`
}

type ListPrivateLinkEndpointServicesOptions struct {
	CloudProvider string
	Region        string
}

// --- Methods ---

// CreatePrivateLinkEndpointService registers a new PrivateLink endpoint service.
func (c *FormationClient) CreatePrivateLinkEndpointService(ctx context.Context, req *CreatePrivateLinkEndpointServiceRequest) (*PrivateLinkEndpointService, error) {
	resp, err := c.post(ctx, "/v1/private-link/endpoint-services", req)
	if err != nil {
		return nil, err
	}
	if err := parseResponse[any](resp, nil); err != nil {
		return nil, err
	}
	return &PrivateLinkEndpointService{
		CloudProvider:       req.CloudProvider,
		Region:              req.Region,
		EndpointServiceID:   req.EndpointServiceID,
		EndpointServiceName: req.EndpointServiceName,
	}, nil
}

// ListPrivateLinkEndpointServices returns all registered endpoint services.
func (c *FormationClient) ListPrivateLinkEndpointServices(ctx context.Context, opts *ListPrivateLinkEndpointServicesOptions) ([]PrivateLinkEndpointService, error) {
	q := url.Values{}
	if opts != nil {
		if opts.CloudProvider != "" {
			q.Set("cloudProvider", opts.CloudProvider)
		}
		if opts.Region != "" {
			q.Set("region", opts.Region)
		}
	}
	resp, err := c.get(ctx, "/v1/private-link/endpoint-services", q)
	if err != nil {
		return nil, err
	}
	var result APIResponse[[]PrivateLinkEndpointService]
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

// DeletePrivateLinkEndpointService deregisters an endpoint service.
func (c *FormationClient) DeletePrivateLinkEndpointService(ctx context.Context, endpointServiceID string) error {
	resp, err := c.delete(ctx, fmt.Sprintf("/v1/private-link/endpoint-services/%s", endpointServiceID))
	if err != nil {
		return err
	}
	return parseResponse[any](resp, nil)
}
