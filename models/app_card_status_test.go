package models

import (
	"strings"
	"testing"
	"time"
)

func TestAppCardStatus_Validate(t *testing.T) {
	cases := []struct {
		name    string
		record  AppCardStatus
		wantErr bool
	}{
		{
			name: "valid service record",
			record: AppCardStatus{
				ScopeType:  AppCardStatusScopeService,
				ScopeValue: "catalog",
				Enabled:    true,
				Label:      "Catalog",
			},
			wantErr: false,
		},
		{
			name: "valid service record disabled",
			record: AppCardStatus{
				ScopeType:  AppCardStatusScopeService,
				ScopeValue: "shelf",
				Enabled:    false,
				Label:      "Shelf",
			},
			wantErr: false,
		},
		{
			name: "missing scope_value is rejected",
			record: AppCardStatus{
				ScopeType: AppCardStatusScopeService,
				Enabled:   true,
				Label:     "Catalog",
			},
			wantErr: true,
		},
		{
			name: "missing label is rejected",
			record: AppCardStatus{
				ScopeType:  AppCardStatusScopeService,
				ScopeValue: "catalog",
				Enabled:    true,
			},
			wantErr: true,
		},
		{
			name: "label exceeding 24 runes is rejected",
			record: AppCardStatus{
				ScopeType:  AppCardStatusScopeService,
				ScopeValue: "catalog",
				Enabled:    true,
				Label:      strings.Repeat("a", 25),
			},
			wantErr: true,
		},
		{
			name: "label with exactly 24 runes is valid",
			record: AppCardStatus{
				ScopeType:  AppCardStatusScopeService,
				ScopeValue: "catalog",
				Enabled:    true,
				Label:      strings.Repeat("a", 24),
			},
			wantErr: false,
		},
		{
			name: "label with Unicode runes respects rune count",
			record: AppCardStatus{
				ScopeType:  AppCardStatusScopeService,
				ScopeValue: "catalog",
				Enabled:    true,
				Label:      strings.Repeat("🎮", 25), // 25 emoji runes
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.record.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("Validate() = nil, want error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestAppCardStatus_CreatedAtUpdatedAt(t *testing.T) {
	now := time.Now().UTC()
	record := AppCardStatus{
		ScopeType:  AppCardStatusScopeService,
		ScopeValue: "catalog",
		Enabled:    true,
		Label:      "Catalog",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := record.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	if record.CreatedAt != now {
		t.Errorf("CreatedAt = %v, want %v", record.CreatedAt, now)
	}
	if record.UpdatedAt != now {
		t.Errorf("UpdatedAt = %v, want %v", record.UpdatedAt, now)
	}
}
