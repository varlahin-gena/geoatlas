package auth

import "testing"

func TestValidRole(t *testing.T) {
	for _, role := range AllRoles() {
		if !ValidRole(role) {
			t.Fatalf("ValidRole(%q) = false", role)
		}
	}
	for _, bad := range []string{"", "admin", "viewer", "ADMINISTRATOR"} {
		if ValidRole(bad) {
			t.Fatalf("ValidRole(%q) = true", bad)
		}
	}
}

func TestIsAdmin(t *testing.T) {
	if !IsAdmin(RoleAdministrator) {
		t.Fatal("administrator must be admin")
	}
	for _, role := range []string{RoleOperator, RoleDashboard} {
		if IsAdmin(role) {
			t.Fatalf("%q must not be admin", role)
		}
	}
}

func TestHasPersistentSession(t *testing.T) {
	if !HasPersistentSession(RoleDashboard) {
		t.Fatal("dashboard must have persistent session")
	}
	for _, role := range []string{RoleAdministrator, RoleOperator} {
		if HasPersistentSession(role) {
			t.Fatalf("%q must not have persistent session", role)
		}
	}
}

func TestAllRolesMatchesConstants(t *testing.T) {
	want := map[string]struct{}{
		RoleAdministrator: {},
		RoleOperator:      {},
		RoleDashboard:     {},
	}
	if len(AllRoles()) != len(want) {
		t.Fatalf("len(AllRoles()) = %d, want %d", len(AllRoles()), len(want))
	}
	for _, role := range AllRoles() {
		if _, ok := want[role]; !ok {
			t.Fatalf("unexpected role %q in AllRoles()", role)
		}
	}
}
