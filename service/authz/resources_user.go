package authz

const (
	ResourceUser = "user"

	ActionUserRead = "read"
)

var (
	UserRead = Permission{Resource: ResourceUser, Action: ActionUserRead}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceUser,
		LabelKey: "User Management",
		Actions: []ActionDefinition{
			{
				Action:         ActionUserRead,
				LabelKey:       "View users",
				DescriptionKey: "View user list and details (without sensitive information).",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
		},
	})
}