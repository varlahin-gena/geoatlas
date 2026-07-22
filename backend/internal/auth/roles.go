package auth

const (
	RoleAdministrator = "administrator"
	RoleOperator      = "operator"
)

func IsAdmin(role string) bool {
	return role == RoleAdministrator
}

func ValidRole(role string) bool {
	return role == RoleAdministrator || role == RoleOperator
}
