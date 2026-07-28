package server

import (
	"testing"

	"github.com/sweetrpg/admin-api/models"
)

func TestParseScope(t *testing.T) {
	cases := []struct {
		raw       string
		wantType  models.ScopeType
		wantValue string
	}{
		{"platform", models.ScopePlatform, ""},
		{"service:catalog", models.ScopeService, "catalog"},
		{"page:/catalog/browse", models.ScopePage, "/catalog/browse"},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			gotType, gotValue := parseScope(tc.raw)
			if gotType != tc.wantType {
				t.Errorf("parseScope(%q) type = %q, want %q", tc.raw, gotType, tc.wantType)
			}
			if gotValue != tc.wantValue {
				t.Errorf("parseScope(%q) value = %q, want %q", tc.raw, gotValue, tc.wantValue)
			}
		})
	}
}
