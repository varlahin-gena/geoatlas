package auth

const (
	RoleAdministrator = "administrator"
	RoleOperator      = "operator"
	RoleDashboard     = "dashboard"
)

func IsAdmin(role string) bool {
	return role == RoleAdministrator
}

// HasPersistentSession — роли с бессрочной cookie-сессией (видеостена / дэшборд).
func HasPersistentSession(role string) bool {
	return role == RoleDashboard
}

func ValidRole(role string) bool {
	return role == RoleAdministrator || role == RoleOperator || role == RoleDashboard
}

// AllRoles — канонический список UI-ролей (SoT для OpenAPI enum и тестов).
func AllRoles() []string {
	return []string{RoleAdministrator, RoleOperator, RoleDashboard}
}
