package authz

const (
	ResourceLog = "log"

	ActionLogReadAll = "read_all"
)

var (
	LogReadAll = Permission{Resource: ResourceLog, Action: ActionLogReadAll}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceLog,
		LabelKey: "Log Management",
		Actions: []ActionDefinition{
			{
				Action:         ActionLogReadAll,
				LabelKey:       "View all user logs",
				DescriptionKey: "View usage logs for all users (without this, only own logs are visible).",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
		},
	})
}