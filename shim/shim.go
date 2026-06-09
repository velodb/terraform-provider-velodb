// Package shim exposes the VeloDB Terraform provider's entry point to external
// consumers that cannot import internal/ packages — most notably the Pulumi
// terraform-bridge, which lives in a separate Go module (pulumi-velodb) and
// shims this provider to generate the Pulumi SDKs.
//
// Keep this package a thin re-export. All resource, data source, and client
// logic stays in internal/; this is the only stable, externally importable
// surface, so the bridge tracks the Terraform provider with a single dependency
// bump rather than reaching into internal/.
package shim

import (
	"github.com/hashicorp/terraform-plugin-framework/provider"

	velodbprovider "github.com/velodb/terraform-provider-velodb/internal/provider"
)

// NewProvider returns the plugin-framework provider constructor for the given
// version string. It mirrors internal/provider.New so the Pulumi bridge can do:
//
//	pf.ShimProvider(shim.NewProvider(version)())
func NewProvider(version string) func() provider.Provider {
	return velodbprovider.New(version)
}
