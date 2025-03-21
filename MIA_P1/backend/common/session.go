package common

// Variables para IDs de usuario actual
var (
	ActiveUserID  int32 = 0
	ActiveGroupID int32 = 0
)

// SetActiveUser establece el usuario y grupo activos
func SetActiveUser(userID, groupID int32) {
	ActiveUserID = userID
	ActiveGroupID = groupID
}
