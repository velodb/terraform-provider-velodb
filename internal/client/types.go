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
	Name                    string                 `json:"name"`
	DeploymentMode          string                 `json:"deploymentMode"`
	CloudProvider           string                 `json:"cloudProvider"`
	Region                  string                 `json:"region"`
	VpcMode                 *string                `json:"vpcMode,omitempty"`
	CreateMode              *string                `json:"createMode,omitempty"`
	VpcID                   *string                `json:"vpcId,omitempty"`
	CredentialID            *int64                 `json:"credentialId,omitempty"`
	NetworkConfigID         *int64                 `json:"networkConfigId,omitempty"`
	BucketName              *string                `json:"bucketName,omitempty"`
	DataCredentialArn       *string                `json:"dataCredentialArn,omitempty"`
	DeploymentCredentialArn *string                `json:"deploymentCredentialArn,omitempty"`
	SubnetID                *string                `json:"subnetId,omitempty"`
	SecurityGroupID         *string                `json:"securityGroupId,omitempty"`
	EndpointID              *string                `json:"endpointId,omitempty"`
	CoreVersion             *string                `json:"coreVersion,omitempty"`
	AdminPassword           *string                `json:"adminPassword,omitempty"`
	InitialCluster          *InitialClusterRequest `json:"initialCluster,omitempty"`
	AdvancedSettings        map[string]any         `json:"advancedSettings,omitempty"`
	Tags                    map[string]string      `json:"tags,omitempty"`
}

type InitialClusterRequest struct {
	Name           string          `json:"name"`
	Zone           *string         `json:"zone,omitempty"`
	ComputeVcpu    int             `json:"computeVcpu"`
	CacheGb        int             `json:"cacheGb"`
	BillingModel   *string         `json:"billingModel,omitempty"`
	Period         *int            `json:"period,omitempty"`
	PeriodUnit     *string         `json:"periodUnit,omitempty"`
	AutoPause      *AutoPauseConfig `json:"autoPause,omitempty"`
}

type AutoPauseConfig struct {
	Enabled            bool `json:"enabled"`
	IdleTimeoutMinutes *int `json:"idleTimeoutMinutes,omitempty"`
}

type UpdateWarehouseRequest struct {
	Name                     *string `json:"name,omitempty"`
	MaintainabilityStartTime *string `json:"maintainabilityStartTime,omitempty"`
	MaintainabilityEndTime   *string `json:"maintainabilityEndTime,omitempty"`
}

type UpdateWarehouseSettingsRequest struct {
	MaintainabilityStartTime *string        `json:"maintainabilityStartTime,omitempty"`
	MaintainabilityEndTime   *string        `json:"maintainabilityEndTime,omitempty"`
	AdvancedSettings         map[string]any `json:"advancedSettings,omitempty"`
}

type UpgradeWarehouseRequest struct {
	TargetVersion string `json:"targetVersion"`
}

type ChangePasswordRequest struct {
	NewPassword string `json:"newPassword"`
}

type WarehouseItem struct {
	WarehouseID    string            `json:"warehouseId"`
	Name           string            `json:"name"`
	Status         string            `json:"status,omitempty"`
	CloudProvider  string            `json:"cloudProvider"`
	Region         string            `json:"region"`
	Zone           string            `json:"zone,omitempty"`
	DeploymentMode string            `json:"deploymentMode,omitempty"`
	CoreVersion    string            `json:"coreVersion,omitempty"`
	PayType        string            `json:"payType,omitempty"`
	CreatedAt      *time.Time        `json:"createdAt,omitempty"`
	ExpireTime     *time.Time        `json:"expireTime,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
}

type CreateWarehouseResult struct {
	WarehouseID string              `json:"warehouseId"`
	ByocSetup   *WarehouseByocSetup `json:"byocSetup,omitempty"`
}

type WarehouseByocSetup struct {
	Token                  string `json:"token,omitempty"`
	ShellCommand           string `json:"shellCommand,omitempty"`
	ShellCommandForNewVpc  string `json:"shellCommandForNewVpc,omitempty"`
	URL                    string `json:"url,omitempty"`
	DocURL                 string `json:"docUrl,omitempty"`
	URLForNewVpc           string `json:"urlForNewVpc,omitempty"`
	DocURLForNewVpc        string `json:"docUrlForNewVpc,omitempty"`
}

type WarehouseSettings struct {
	WarehouseID       string         `json:"warehouseId"`
	StorageBucket     string         `json:"storageBucket"`
	Region            string         `json:"region"`
	CloudProvider     string         `json:"cloudProvider"`
	VpcID             string         `json:"vpcId,omitempty"`
	MaintenanceWindow map[string]any `json:"maintenanceWindow,omitempty"`
	Config            map[string]any `json:"config,omitempty"`
}

type WarehouseConnections struct {
	WarehouseID string                    `json:"warehouseId"`
	Clusters    []WarehouseConnectionItem `json:"clusters"`
}

type WarehouseConnectionItem struct {
	ClusterID         string `json:"clusterId"`
	Type              string `json:"type"`
	JdbcPort          int    `json:"jdbcPort"`
	HttpPort          int    `json:"httpPort"`
	StreamLoadPort    int    `json:"streamLoadPort"`
	PublicEndpoint    string `json:"publicEndpoint"`
	PrivateEndpoint   string `json:"privateEndpoint"`
	ListenerPort      int    `json:"listenerPort"`
	EndpointServiceID string `json:"endpointServiceId,omitempty"`
}

// --- Cluster ---

type CreateClusterRequest struct {
	Name           string           `json:"name"`
	ClusterType    string           `json:"clusterType"`
	Zone           *string          `json:"zone,omitempty"`
	ComputeVcpu    int              `json:"computeVcpu"`
	CacheGb        int              `json:"cacheGb"`
	BillingModel     *string          `json:"billingModel,omitempty"`
	Period           *int             `json:"period,omitempty"`
	PeriodUnit       *string          `json:"periodUnit,omitempty"`
	AutoRenewEnabled *int             `json:"autoRenewEnabled,omitempty"`
	AutoPause        *AutoPauseConfig `json:"autoPause,omitempty"`
}

type UpdateClusterRequest struct {
	Name             *string          `json:"name,omitempty"`
	ComputeVcpu      *int             `json:"computeVcpu,omitempty"`
	CacheGb          *int             `json:"cacheGb,omitempty"`
	BillingModel     *string          `json:"billingModel,omitempty"`
	Period           *int             `json:"period,omitempty"`
	PeriodUnit       *string          `json:"periodUnit,omitempty"`
	AutoRenewEnabled *int             `json:"autoRenewEnabled,omitempty"`
	AutoPause        *AutoPauseConfig `json:"autoPause,omitempty"`
}

type RenewClusterRequest struct {
	Period           int    `json:"period"`
	PeriodUnit       string `json:"periodUnit"`
	AutoRenewEnabled *int   `json:"autoRenewEnabled,omitempty"`
}

type ConvertToSubscriptionRequest struct {
	Period            int    `json:"period"`
	PeriodUnit        string `json:"periodUnit"`
	AutoRenewEnabled  *int   `json:"autoRenewEnabled,omitempty"`
	OnDemandNodeCount *int   `json:"onDemandNodeCount,omitempty"`
}

type ClusterConnectionInfo struct {
	PublicEndpoint  string `json:"publicEndpoint,omitempty"`
	PrivateEndpoint string `json:"privateEndpoint,omitempty"`
	ListenerPort    int    `json:"listenerPort,omitempty"`
}

type ClusterItem struct {
	ClusterID      string                 `json:"clusterId"`
	WarehouseID    string                 `json:"warehouseId"`
	Name           string                 `json:"name"`
	Status         string                 `json:"status"`
	ClusterType    string                 `json:"clusterType,omitempty"`
	CloudProvider  string                 `json:"cloudProvider,omitempty"`
	Region         string                 `json:"region,omitempty"`
	Zone           string                 `json:"zone,omitempty"`
	DiskSumSize    int                    `json:"diskSumSize,omitempty"`
	PayType        string                 `json:"payType,omitempty"`
	Period         int                    `json:"period,omitempty"`
	PeriodUnit     string                 `json:"periodUnit,omitempty"`
	CreatedAt      *time.Time             `json:"createdAt,omitempty"`
	StartedAt      *time.Time             `json:"startedAt,omitempty"`
	ExpireTime     *time.Time             `json:"expireTime,omitempty"`
	ConnectionInfo *ClusterConnectionInfo `json:"connectionInfo,omitempty"`
}

type CreateClusterResult struct {
	ClusterID string `json:"clusterId"`
}

// --- List options ---

type ListWarehousesOptions struct {
	Page           int
	Size           int
	Keyword        string
	CloudProvider  string
	Region         string
	DeploymentMode string
}

type ListClustersOptions struct {
	Page        int
	Size        int
	Keyword     string
	Status      string
	ClusterType string
	PayType     string
}
