package datasource

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// cloudSettingListPageSize is the page size used when listing cloud-setting
// objects (credentials, network configs, encryption keys).
const cloudSettingListPageSize = 100

// cloudSettingPageFetcher fetches one 1-indexed page of size items, returning
// the items, the server-reported total, and any error.
type cloudSettingPageFetcher[T any] func(page, size int) ([]T, int64, error)

// renderCloudSettingList pages through every result from fetch, converts each
// item with mapItem, and returns the assembled list plus the authoritative
// total. Pagination stops when the server reports it has returned everything
// (total reached), or returns a short/empty page -- so results are never
// silently capped at a single page. label names the objects in error messages.
func renderCloudSettingList[T any](
	ctx context.Context,
	diags *diag.Diagnostics,
	label string,
	attrTypes map[string]attr.Type,
	fetch cloudSettingPageFetcher[T],
	mapItem func(context.Context, *diag.Diagnostics, T) attr.Value,
) (types.List, int64) {
	objType := types.ObjectType{AttrTypes: attrTypes}

	var items []attr.Value
	var total int64
	for page := 1; ; page++ {
		data, pageTotal, err := fetch(page, cloudSettingListPageSize)
		if err != nil {
			diags.AddError("Error listing "+label, err.Error())
			return types.ListNull(objType), 0
		}
		total = pageTotal
		for _, item := range data {
			items = append(items, mapItem(ctx, diags, item))
		}
		if len(data) == 0 || (total > 0 && int64(len(items)) >= total) || len(data) < cloudSettingListPageSize {
			break
		}
	}
	if total == 0 {
		// The server did not report a total; fall back to what we collected
		// (accurate because pagination above fetches every page).
		total = int64(len(items))
	}

	list, listDiags := types.ListValue(objType, items)
	diags.Append(listDiags...)
	return list, total
}

// timeVal formats an optional RFC 3339 timestamp, returning null when absent.
func timeVal(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}
