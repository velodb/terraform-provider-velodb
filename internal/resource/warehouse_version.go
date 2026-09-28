package resource

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/velodb/terraform-provider-velodb/internal/client"
)

// A two-part creation selector remains stable when the API fills in its patch.
// An omitted selector continues to expose the full API version as before.
func coreVersionForState(configured types.String, actual string) types.String {
	if !configured.IsNull() && !configured.IsUnknown() && strings.Count(configured.ValueString(), ".") == 1 && strings.HasPrefix(actual, configured.ValueString()+".") {
		return configured
	}
	return stringOrNull(actual)
}

func coreVersionMatches(target, actual string) bool {
	return target == actual || (strings.Count(target, ".") == 1 && strings.HasPrefix(actual, target+"."))
}

func validateCoreVersionUpgrade(target, current string) error {
	if coreVersionMatches(target, current) {
		return nil
	}
	if strings.Count(target, ".") != 2 {
		return fmt.Errorf("upgrades require an exact major.minor.patch core_version, for example 4.1.9; two-part versions are only resolved by the API at creation")
	}
	targetParts, currentParts := strings.Split(target, "."), strings.Split(current, ".")
	// The API's upgrade eligibility list remains authoritative for nonstandard reported versions.
	if len(currentParts) != 3 {
		return nil
	}
	for i := 0; i < 3; i++ {
		next, err := strconv.ParseUint(targetParts[i], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid target core_version %q", target)
		}
		prior, err := strconv.ParseUint(currentParts[i], 10, 64)
		if err != nil {
			return nil
		}
		if next < prior {
			return fmt.Errorf("core_version downgrade from %s to %s is not supported", current, target)
		}
		if next > prior {
			return nil
		}
	}
	return nil
}

func resolveCoreVersionID(target string, versions []client.WarehouseVersion) (int64, error) {
	var id int64
	for _, candidate := range versions {
		if candidate.Version != target {
			continue
		}
		if candidate.VersionID <= 0 {
			return 0, fmt.Errorf("upgrade target %s has an invalid version ID", target)
		}
		if id != 0 && id != candidate.VersionID {
			return 0, fmt.Errorf("upgrade target %s matches multiple version IDs; cannot safely select a target", target)
		}
		id = candidate.VersionID
	}
	if id == 0 {
		return 0, fmt.Errorf("core_version %s is not an available upgrade target for this warehouse", target)
	}
	return id, nil
}

func (r *WarehouseResource) coreVersionUpgradeID(ctx context.Context, configured types.String, plan, state *WarehouseResourceModel) (int64, error) {
	if !configured.IsNull() && !configured.IsUnknown() {
		current := state.CoreVersion.ValueString()
		target := configured.ValueString()
		if coreVersionMatches(target, current) {
			return 0, nil
		}
		// State may retain a two-part creation selector. Fetch its actual patch
		// before validating a change, without exposing another Terraform field.
		if strings.Count(current, ".") == 1 {
			warehouse, err := r.client.GetWarehouse(ctx, state.ID.ValueString())
			if err != nil {
				return 0, err
			}
			current = warehouse.CoreVersion
			if coreVersionMatches(target, current) {
				return 0, nil
			}
		}
		if err := validateCoreVersionUpgrade(target, current); err != nil {
			return 0, err
		}
		versions, err := r.client.ListWarehouseVersions(ctx, state.ID.ValueString())
		if err != nil {
			return 0, err
		}
		return resolveCoreVersionID(target, versions)
	}
	if !plan.CoreVersionID.IsNull() && !plan.CoreVersionID.IsUnknown() && !plan.CoreVersionID.Equal(state.CoreVersionID) {
		if plan.CoreVersionID.ValueInt64() <= 0 {
			return 0, fmt.Errorf("core_version_id must be a positive core version ID")
		}
		return plan.CoreVersionID.ValueInt64(), nil
	}
	return 0, nil
}
