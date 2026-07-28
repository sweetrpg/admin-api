package models

import (
	"testing"
	"time"
)

func TestBannerMessage_Validate(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)

	cases := []struct {
		name    string
		banner  BannerMessage
		wantErr bool
	}{
		{
			name: "valid platform banner",
			banner: BannerMessage{
				ScopeType: ScopePlatform,
				Severity:  SeverityInfo,
				Message:   "hello",
				ExpiresAt: future,
			},
			wantErr: false,
		},
		{
			name: "valid service banner",
			banner: BannerMessage{
				ScopeType:  ScopeService,
				ScopeValue: "catalog",
				Severity:   SeverityWarning,
				Message:    "hello",
				ExpiresAt:  future,
			},
			wantErr: false,
		},
		{
			name: "missing expires_at is rejected",
			banner: BannerMessage{
				ScopeType: ScopePlatform,
				Severity:  SeverityInfo,
				Message:   "hello",
			},
			wantErr: true,
		},
		{
			name: "unknown scope type is rejected",
			banner: BannerMessage{
				ScopeType: "tenant",
				Severity:  SeverityInfo,
				Message:   "hello",
				ExpiresAt: future,
			},
			wantErr: true,
		},
		{
			name: "unknown severity is rejected",
			banner: BannerMessage{
				ScopeType: ScopePlatform,
				Severity:  "urgent",
				Message:   "hello",
				ExpiresAt: future,
			},
			wantErr: true,
		},
		{
			name: "service scope without scope_value is rejected",
			banner: BannerMessage{
				ScopeType: ScopeService,
				Severity:  SeverityInfo,
				Message:   "hello",
				ExpiresAt: future,
			},
			wantErr: true,
		},
		{
			name: "platform scope with a scope_value is rejected",
			banner: BannerMessage{
				ScopeType:  ScopePlatform,
				ScopeValue: "catalog",
				Severity:   SeverityInfo,
				Message:    "hello",
				ExpiresAt:  future,
			},
			wantErr: true,
		},
		{
			name: "empty message is rejected",
			banner: BannerMessage{
				ScopeType: ScopePlatform,
				Severity:  SeverityInfo,
				ExpiresAt: future,
			},
			wantErr: true,
		},
		{
			name: "starts_at after expires_at is rejected",
			banner: BannerMessage{
				ScopeType: ScopePlatform,
				Severity:  SeverityInfo,
				Message:   "hello",
				StartsAt:  &future,
				ExpiresAt: past,
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.banner.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("Validate() = nil, want error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestSeverity_Rank(t *testing.T) {
	if SeverityCritical.Rank() >= SeverityWarning.Rank() {
		t.Errorf("critical should rank before warning")
	}
	if SeverityWarning.Rank() >= SeverityInfo.Rank() {
		t.Errorf("warning should rank before info")
	}
	if Severity("bogus").Rank() <= SeverityInfo.Rank() {
		t.Errorf("unknown severity should rank last")
	}
}
