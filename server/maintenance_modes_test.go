package server

import (
	"testing"
	"time"

	"github.com/sweetrpg/admin-api/models"
	"go.mongodb.org/mongo-driver/bson"
)

func TestParseMaintenanceScope(t *testing.T) {
	cases := []struct {
		raw       string
		wantType  models.MaintenanceScopeType
		wantValue string
	}{
		{"platform", models.MaintenanceScopePlatform, ""},
		{"service:catalog", models.MaintenanceScopeService, "catalog"},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			gotType, gotValue := parseMaintenanceScope(tc.raw)
			if gotType != tc.wantType {
				t.Errorf("parseMaintenanceScope(%q) type = %q, want %q", tc.raw, gotType, tc.wantType)
			}
			if gotValue != tc.wantValue {
				t.Errorf("parseMaintenanceScope(%q) value = %q, want %q", tc.raw, gotValue, tc.wantValue)
			}
		})
	}
}

func TestExtractServiceScopeValues(t *testing.T) {
	cases := []struct {
		name   string
		scopes []string
		want   []string
	}{
		{"platform only", []string{"platform"}, nil},
		{"single service", []string{"service:catalog"}, []string{"catalog"}},
		{"platform and service", []string{"platform", "service:catalog"}, []string{"catalog"}},
		{"multiple services", []string{"service:catalog", "service:shelf"}, []string{"catalog", "shelf"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractServiceScopeValues(tc.scopes)
			if len(got) != len(tc.want) {
				t.Fatalf("extractServiceScopeValues(%v) = %v, want %v", tc.scopes, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("extractServiceScopeValues(%v)[%d] = %q, want %q", tc.scopes, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestBuildActiveMaintenanceModeFilter_AlwaysRequiresEnabled(t *testing.T) {
	filter := buildActiveMaintenanceModeFilter([]string{"platform"})

	found := false
	for _, elem := range filter {
		if elem.Key == "enabled" && elem.Value == true {
			found = true
		}
	}
	if !found {
		t.Errorf("buildActiveMaintenanceModeFilter() filter = %v, want an enabled=true clause", filter)
	}
}

func TestBuildActiveMaintenanceModeFilter_PlatformAlwaysIncluded(t *testing.T) {
	// Even when only a service scope is requested, platform-scoped records must
	// still match, since a platform-wide maintenance record applies to every scope.
	filter := buildActiveMaintenanceModeFilter([]string{"service:catalog"})

	scopeOr := scopeOrClause(t, filter)
	if !scopeOrContainsPlatform(scopeOr) {
		t.Errorf("buildActiveMaintenanceModeFilter() $or = %v, want a platform clause", scopeOr)
	}
}

func TestBuildActiveMaintenanceModeFilter_ServiceScopeOmittedWhenNotRequested(t *testing.T) {
	filter := buildActiveMaintenanceModeFilter([]string{"platform"})

	scopeOr := scopeOrClause(t, filter)
	if len(scopeOr) != 1 {
		t.Errorf("buildActiveMaintenanceModeFilter() $or = %v, want exactly the platform clause", scopeOr)
	}
}

// TestBuildActiveMaintenanceModeFilter_ExcludesPastEndDate reproduces the bug where a
// record with enabled=true but an ends_at in the past still matched "active" - enabled
// never auto-clears on expiry (see models.MaintenanceMode.isStale), so the filter itself
// must exclude expired windows.
func TestBuildActiveMaintenanceModeFilter_ExcludesPastEndDate(t *testing.T) {
	filter := buildActiveMaintenanceModeFilter([]string{"platform"})

	for _, elem := range filter {
		if elem.Key != "$and" {
			continue
		}
		andClauses, ok := elem.Value.(bson.A)
		if !ok || len(andClauses) == 0 {
			t.Fatalf("$and clause has unexpected shape %v", elem.Value)
		}
		endsDoc, ok := andClauses[0].(bson.D)
		if !ok {
			t.Fatalf("$and[0] has unexpected type %T", andClauses[0])
		}
		for _, e := range endsDoc {
			if e.Key != "$or" {
				continue
			}
			endsOr, ok := e.Value.(bson.A)
			if !ok {
				t.Fatalf("ends_at $or has unexpected type %T", e.Value)
			}
			foundNull, foundGt := false, false
			for _, clause := range endsOr {
				doc, ok := clause.(bson.D)
				if !ok {
					continue
				}
				for _, d := range doc {
					if d.Key != "ends_at" {
						continue
					}
					if d.Value == nil {
						foundNull = true
					}
					if sub, ok := d.Value.(bson.D); ok {
						for _, s := range sub {
							if s.Key == "$gt" {
								if _, ok := s.Value.(time.Time); ok {
									foundGt = true
								}
							}
						}
					}
				}
			}
			if !foundNull || !foundGt {
				t.Errorf("ends_at $or = %v, want a null clause and a $gt-now clause", endsOr)
			}
			return
		}
		t.Fatalf("$and clause missing nested $or: %v", andClauses)
	}
	t.Fatalf("buildActiveMaintenanceModeFilter() filter = %v, want an $and clause excluding expired records", filter)
}

// scopeOrClause extracts the $or clause from a filter built by
// buildActiveMaintenanceModeFilter, failing the test if it's missing or the wrong
// type.
func scopeOrClause(t *testing.T, filter bson.D) bson.A {
	t.Helper()
	for _, elem := range filter {
		if elem.Key != "$or" {
			continue
		}
		scopeOr, ok := elem.Value.(bson.A)
		if !ok {
			t.Fatalf("$or clause has unexpected type %T", elem.Value)
		}
		return scopeOr
	}
	t.Fatalf("buildActiveMaintenanceModeFilter() filter = %v, want a $or clause", filter)
	return nil
}

// scopeOrContainsPlatform reports whether any clause in the $or array matches on
// scope_type=platform.
func scopeOrContainsPlatform(scopeOr bson.A) bool {
	for _, clause := range scopeOr {
		doc, ok := clause.(bson.D)
		if !ok {
			continue
		}
		for _, elem := range doc {
			if elem.Key == "scope_type" && elem.Value == models.MaintenanceScopePlatform {
				return true
			}
		}
	}
	return false
}
