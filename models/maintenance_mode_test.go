package models

import (
	"testing"
	"time"
)

func TestMaintenanceMode_Validate(t *testing.T) {
	starts := time.Now().Add(-time.Hour)
	ends := time.Now().Add(time.Hour)
	beforeStarts := time.Now().Add(-2 * time.Hour)

	cases := []struct {
		name    string
		record  MaintenanceMode
		wantErr bool
	}{
		{
			name: "valid platform record, no ends_at",
			record: MaintenanceMode{
				ScopeType:   MaintenanceScopePlatform,
				Enabled:     true,
				StartsAt:    starts,
				Label:       "Down for maintenance",
				Description: "The whole platform is offline for a scheduled upgrade.",
			},
			wantErr: false,
		},
		{
			name: "valid service record with ends_at",
			record: MaintenanceMode{
				ScopeType:   MaintenanceScopeService,
				ScopeValue:  "catalog",
				Enabled:     true,
				StartsAt:    starts,
				EndsAt:      &ends,
				Label:       "Catalog maintenance",
				Description: "Catalog is offline for a data migration.",
			},
			wantErr: false,
		},
		{
			name: "disabled record is still valid",
			record: MaintenanceMode{
				ScopeType:   MaintenanceScopePlatform,
				Enabled:     false,
				StartsAt:    starts,
				Label:       "Down for maintenance",
				Description: "Staged but not active.",
			},
			wantErr: false,
		},
		{
			name: "missing starts_at is rejected",
			record: MaintenanceMode{
				ScopeType:   MaintenanceScopePlatform,
				Label:       "Down for maintenance",
				Description: "The whole platform is offline.",
			},
			wantErr: true,
		},
		{
			name: "unknown scope type is rejected",
			record: MaintenanceMode{
				ScopeType:   "tenant",
				StartsAt:    starts,
				Label:       "Down for maintenance",
				Description: "The whole platform is offline.",
			},
			wantErr: true,
		},
		{
			name: "service scope without scope_value is rejected",
			record: MaintenanceMode{
				ScopeType:   MaintenanceScopeService,
				StartsAt:    starts,
				Label:       "Down for maintenance",
				Description: "Catalog is offline.",
			},
			wantErr: true,
		},
		{
			name: "platform scope with a scope_value is rejected",
			record: MaintenanceMode{
				ScopeType:   MaintenanceScopePlatform,
				ScopeValue:  "catalog",
				StartsAt:    starts,
				Label:       "Down for maintenance",
				Description: "The whole platform is offline.",
			},
			wantErr: true,
		},
		{
			name: "missing label is rejected",
			record: MaintenanceMode{
				ScopeType:   MaintenanceScopePlatform,
				StartsAt:    starts,
				Description: "The whole platform is offline.",
			},
			wantErr: true,
		},
		{
			name: "missing description is rejected",
			record: MaintenanceMode{
				ScopeType: MaintenanceScopePlatform,
				StartsAt:  starts,
				Label:     "Down for maintenance",
			},
			wantErr: true,
		},
		{
			name: "ends_at before starts_at is rejected",
			record: MaintenanceMode{
				ScopeType:   MaintenanceScopePlatform,
				StartsAt:    starts,
				EndsAt:      &beforeStarts,
				Label:       "Down for maintenance",
				Description: "The whole platform is offline.",
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
