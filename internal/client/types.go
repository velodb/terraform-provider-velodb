package client

import "time"

// --- Response envelope ---

type APIResponse[T any] struct {
	Success   bool   `json:"success"`
	RequestID string `json:"requestId"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Data      T      `json:"data"`
}

type PageResponse[T any] struct {
	Success   bool   `json:"success"`
	RequestID string `json:"requestId"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Data      []T    `json:"data"`
	Page      int    `json:"page"`
	Size      int    `json:"size"`
	Total     int64  `json:"total"`
}

// --- Warehouse ---

type CreateWarehouseRequest struct {
	Name            string                 `json:"name"`
	DeploymentMode  string                 `json:"deploymentMode"`
	CloudProvider   string                 `json:"cloudProvider"`
	Region          string                 `json:"region"`
	VpcMode         *string                `json:"vpcMode,omitempty"`
	SetupMode       *string                `json:"setupMode,omitempty"`
	CredentialID    *int64                 `json:"credentialId,omitempty"`
	NetworkConfigID *int64                 `json:"networkConfigId,omitempty"`
	AdminPassword   *string                `json:"adminPassword,omitempty"`
	InitialCluster  *InitialClusterRequest `json:"initialCluster,omitempty"`
}

type InitialClusterRequest struct {
	Zone        string           `json:"zone"`
	ComputeVcpu int              `json:"computeVcpu"`
	CacheGb     int              `json:"cacheGb"`
	AutoPause   *AutoPauseConfig `json:"autoPause,omitempty"`
}

type AutoPauseConfig struct {
	Enabled            bool `json:"enabled"`
	IdleTimeoutMinutes *int `json:"idleTimeoutMinutes,omitempty"`
}

type UpdateWarehouseRequest struct {
	Name *string `json:"name,omitempty"`
}

type UpgradeWarehouseRequest struct {
	TargetVersionID int64 `json:"targetVersionId"`
}

type WarehouseVersion struct {
	VersionID   int64  `json:"versionId"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	IsDefault   bool   `json:"isDefault,omitempty"`
}

type ChangePasswordRequest struct {
	NewPassword string `json:"newPassword"`
}

type WarehouseItem struct {
	WarehouseID         string               `json:"warehouseId"`
	Name                string               `json:"name"`
	Status              string               `json:"status,omitempty"`
	CloudProvider       string               `json:"cloudProvider"`
	Region              string               `json:"region"`
	Zone                string               `json:"zone,omitempty"`
	DeploymentMode      string               `json:"deploymentMode,omitempty"`
	CoreVersion         string               `json:"coreVersion,omitempty"`
	PayType             string               `json:"payType,omitempty"`
	EndpointServiceID   string               `json:"endpointServiceId,omitempty"`
	EndpointServiceName string               `json:"endpointServiceName,omitempty"`
	SetupGuide          *WarehouseSetupGuide `json:"setupGuide,omitempty"`
	CreatedAt           *time.Time           `json:"createdAt,omitempty"`
	ExpireTime          *time.Time           `json:"expireTime,omitempty"`
	Tags                map[string]string    `json:"tags,omitempty"`
}

type CreateWarehouseResult struct {
	WarehouseID string               `json:"warehouseId"`
	SetupGuide  *WarehouseSetupGuide `json:"setupGuide,omitempty"`
}

type WarehouseSetupGuide struct {
	SetupURL     string `json:"setupUrl,omitempty"`
	GuideURL     string `json:"guideUrl,omitempty"`
	ShellCommand string `json:"shellCommand,omitempty"`
}

type RegisterWarehousePrivateEndpointRequest struct {
	EndpointID  string  `json:"endpointId"`
	DNSName     *string `json:"dnsName,omitempty"`
	Description *string `json:"description,omitempty"`
}

// --- Cluster ---

type CreateClusterRequest struct {
	Name        string           `json:"name"`
	ClusterType string           `json:"clusterType"`
	Zone        *string          `json:"zone,omitempty"`
	ComputeVcpu int              `json:"computeVcpu"`
	CacheGb     int              `json:"cacheGb"`
	AutoPause   *AutoPauseConfig `json:"autoPause,omitempty"`
}

type UpdateClusterRequest struct {
	Name        *string          `json:"name,omitempty"`
	ComputeVcpu *int             `json:"computeVcpu,omitempty"`
	CacheGb     *int             `json:"cacheGb,omitempty"`
	AutoPause   *AutoPauseConfig `json:"autoPause,omitempty"`
}

type ClusterConnectionInfo struct {
	PublicEndpoint  string `json:"publicEndpoint,omitempty"`
	PrivateEndpoint string `json:"privateEndpoint,omitempty"`
	ListenerPort    int    `json:"listenerPort,omitempty"`
}

type ClusterItem struct {
	ClusterID             string                 `json:"clusterId"`
	WarehouseID           string                 `json:"warehouseId"`
	Name                  string                 `json:"name"`
	Status                string                 `json:"status"`
	ClusterType           string                 `json:"clusterType,omitempty"`
	CloudProvider         string                 `json:"cloudProvider,omitempty"`
	Region                string                 `json:"region,omitempty"`
	Zone                  string                 `json:"zone,omitempty"`
	DiskSumSize           int                    `json:"diskSumSize,omitempty"`
	BillingModel          string                 `json:"billingModel,omitempty"`
	Period                int                    `json:"period,omitempty"`
	PeriodUnit            string                 `json:"periodUnit,omitempty"`
	NodeCount             int                    `json:"nodeCount,omitempty"`
	OnDemandNodeCount     int                    `json:"onDemandNodeCount,omitempty"`
	SubscriptionNodeCount int                    `json:"subscriptionNodeCount,omitempty"`
	CreatedAt             *time.Time             `json:"createdAt,omitempty"`
	StartedAt             *time.Time             `json:"startedAt,omitempty"`
	ExpireTime            *time.Time             `json:"expireTime,omitempty"`
	AutoPause             *AutoPauseConfig       `json:"autoPause,omitempty"`
	ConnectionInfo        *ClusterConnectionInfo `json:"connectionInfo,omitempty"`
}

// ClusterDetail — GET /clusters/{id} returns richer mixed-billing info
type ClusterDetail struct {
	ClusterItem
	BillingSummary *ClusterBillingSummary `json:"billingSummary,omitempty"`
	BillingPools   *ClusterBillingPools   `json:"billingPools,omitempty"`
}

type ClusterBillingSummary struct {
	IsMixedBilling        bool `json:"isMixedBilling"`
	NodeCount             int  `json:"nodeCount,omitempty"`
	OnDemandNodeCount     int  `json:"onDemandNodeCount,omitempty"`
	SubscriptionNodeCount int  `json:"subscriptionNodeCount,omitempty"`
	TotalCpu              int  `json:"totalCpu,omitempty"`
	TotalDiskSizeGb       int  `json:"totalDiskSizeGb,omitempty"`
}

type ClusterBillingPools struct {
	OnDemand     *ClusterBillingPool `json:"onDemand,omitempty"`
	Subscription *ClusterBillingPool `json:"subscription,omitempty"`
}

type ClusterBillingPool struct {
	NodeCount  int        `json:"nodeCount,omitempty"`
	Cpu        int        `json:"cpu,omitempty"`
	DiskSizeGb int        `json:"diskSizeGb,omitempty"`
	Period     int        `json:"period,omitempty"`
	PeriodUnit string     `json:"periodUnit,omitempty"`
	ExpireTime *time.Time `json:"expireTime,omitempty"`
}

type CreateClusterResult struct {
	ClusterID string `json:"clusterId"`
}

// --- List options ---

type ListWarehousesOptions struct {
	Page           int
	Size           int
	WarehouseID    string
	Name           string
	Keyword        string // Deprecated: use WarehouseID or Name.
	CloudProvider  string
	Region         string
	DeploymentMode string
}

type ListClustersOptions struct {
	Page        int
	Size        int
	ClusterID   string
	ClusterName string
	Keyword     string // Deprecated: use ClusterID or ClusterName.
	Status      string
	ClusterType string
}
