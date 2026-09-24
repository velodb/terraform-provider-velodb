package resource

import (
	"strings"
	"testing"
)

func TestParseBYOCNetworkImportID(t *testing.T) {
	tests := []struct {
		name        string
		importID    string
		wantErr     bool
		wantID      int64
		errContains string
	}{
		{name: "valid", importID: "aws/456", wantID: 456},
		{name: "valid with surrounding whitespace", importID: "  aws/456  ", wantID: 456},
		{name: "old three-part format rejected", importID: "aws/456/123", wantErr: true, errContains: "credential_id is now read automatically"},
		{name: "trailing slash", importID: "aws/456/", wantErr: true},
		{name: "missing id", importID: "aws/", wantErr: true},
		{name: "provider only", importID: "aws", wantErr: true},
		{name: "empty", importID: "", wantErr: true},
		{name: "wrong provider", importID: "gcp/456", wantErr: true},
		{name: "non-numeric id", importID: "aws/abc", wantErr: true},
		{name: "zero id", importID: "aws/0", wantErr: true},
		{name: "negative id", importID: "aws/-5", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, id, err := parseBYOCNetworkImportID(tt.importID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got provider=%q id=%d", tt.importID, provider, id)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.importID, err)
			}
			if provider != "aws" || id != tt.wantID {
				t.Fatalf("parseBYOCNetworkImportID(%q) = (%q, %d), want (aws, %d)", tt.importID, provider, id, tt.wantID)
			}
		})
	}
}
