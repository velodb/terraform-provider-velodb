package datasource

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// listPageSize is the page size used when a data source lists every matching
// object from a paginated endpoint.
const listPageSize = 100

// pageFetcher fetches one 1-indexed page of size items, returning the items,
// the server-reported total, and any error.
type pageFetcher[T any] func(page, size int) ([]T, int64, error)

// fetchAllPages pages through every result from fetch and returns the items
// plus the authoritative total. Pagination stops when the server reports it has
// returned everything (total reached), or returns a short/empty page -- so
// results are never silently capped at a single page.
func fetchAllPages[T any](fetch pageFetcher[T]) ([]T, int64, error) {
	var items []T
	var total int64
	for page := 1; ; page++ {
		data, pageTotal, err := fetch(page, listPageSize)
		if err != nil {
			return nil, 0, err
		}
		total = pageTotal
		items = append(items, data...)
		if len(data) == 0 || (total > 0 && int64(len(items)) >= total) || len(data) < listPageSize {
			break
		}
	}
	if total == 0 {
		// The server did not report a total; fall back to what we collected
		// (accurate because pagination above fetches every page).
		total = int64(len(items))
	}
	return items, total, nil
}

// renderCloudSettingList fetches every page from fetch, converts each item
// with mapItem, and returns the assembled list plus the authoritative total.
// label names the objects in error messages.
func renderCloudSettingList[T any](
	ctx context.Context,
	diags *diag.Diagnostics,
	label string,
	attrTypes map[string]attr.Type,
	fetch pageFetcher[T],
	mapItem func(context.Context, *diag.Diagnostics, T) attr.Value,
) (types.List, int64) {
	objType := types.ObjectType{AttrTypes: attrTypes}

	data, total, err := fetchAllPages(fetch)
	if err != nil {
		diags.AddError("Error listing "+label, err.Error())
		return types.ListNull(objType), 0
	}
	items := make([]attr.Value, 0, len(data))
	for _, item := range data {
		items = append(items, mapItem(ctx, diags, item))
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
