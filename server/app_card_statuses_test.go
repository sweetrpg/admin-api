package server

import (
	"testing"
)

func TestParseAppCardScope(t *testing.T) {
	cases := []struct {
		raw       string
		wantType  string
		wantValue string
	}{
		{"service:catalog", "service", "catalog"},
		{"service:shelf", "service", "shelf"},
		{"unknown", "unknown", ""},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			gotType, gotValue := parseAppCardScope(tc.raw)
			if gotType != tc.wantType {
				t.Errorf("parseAppCardScope(%q) type = %q, want %q", tc.raw, gotType, tc.wantType)
			}
			if gotValue != tc.wantValue {
				t.Errorf("parseAppCardScope(%q) value = %q, want %q", tc.raw, gotValue, tc.wantValue)
			}
		})
	}
}

func TestExtractAppCardServiceScopeValues(t *testing.T) {
	cases := []struct {
		name   string
		scopes []string
		want   []string
	}{
		{"single service", []string{"service:catalog"}, []string{"catalog"}},
		{"multiple services", []string{"service:catalog", "service:shelf"}, []string{"catalog", "shelf"}},
		{"unknown scope type", []string{"unknown:value"}, nil},
		{"empty result", []string{"invalid"}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractAppCardServiceScopeValues(tc.scopes)
			if len(got) != len(tc.want) {
				t.Fatalf("extractAppCardServiceScopeValues(%v) = %v, want %v", tc.scopes, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("extractAppCardServiceScopeValues(%v)[%d] = %q, want %q", tc.scopes, i, got[i], tc.want[i])
				}
			}
		})
	}
}
