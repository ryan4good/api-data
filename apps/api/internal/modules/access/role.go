package access

type Role string

const (
	RoleOwner      Role = "owner"
	RoleMaintainer Role = "maintainer"
	RoleReviewer   Role = "reviewer"
	RoleRunner     Role = "runner"
	RoleViewer     Role = "viewer"
)

func (r Role) Valid() bool {
	switch r {
	case RoleOwner, RoleMaintainer, RoleReviewer, RoleRunner, RoleViewer:
		return true
	default:
		return false
	}
}
