package resource

import (
	"testing"
)

func TestParseBYOCNetworkImportID(t *testing.T) {
	tests := []struct {
		name             string
		importID         string
		wantErr          bool
		wantNetworkID    int64
		wantCredentialID int64
	}{
		{name: "valid", importID: "aws/456/123", wantNetworkID: 456, wantCredentialID: 123},
		{name: "valid with surrounding whitespace", importID: "  aws/456/123  ", wantNetworkID: 456, wantCredentialID: 123},
		{name: "two-part format rejected", importID: "aws/456", wantErr: true},
		{name: "trailing slash", importID: "aws/456/123/", wantErr: true},
		{name: "missing credential id", importID: "aws/456/", wantErr: true},
		{name: "missing both ids", importID: "aws//", wantErr: true},
		{name: "provider only", importID: "aws", wantErr: true},
		{name: "empty", importID: "", wantErr: true},
		{name: "wrong provider", importID: "gcp/456/123", wantErr: true},
		{name: "non-numeric network id", importID: "aws/abc/123", wantErr: true},
		{name: "non-numeric credential id", importID: "aws/456/abc", wantErr: true},
		{name: "zero network id", importID: "aws/0/123", wantErr: true},
		{name: "zero credential id", importID: "aws/456/0", wantErr: true},
		{name: "negative credential id", importID: "aws/456/-5", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, networkID, credentialID, err := parseBYOCNetworkImportID(tt.importID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got provider=%q networkID=%d credentialID=%d", tt.importID, provider, networkID, credentialID)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.importID, err)
			}
			if provider != "aws" || networkID != tt.wantNetworkID || credentialID != tt.wantCredentialID {
				t.Fatalf("parseBYOCNetworkImportID(%q) = (%q, %d, %d), want (aws, %d, %d)", tt.importID, provider, networkID, credentialID, tt.wantNetworkID, tt.wantCredentialID)
			}
		})
	}
}
