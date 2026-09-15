package presence

import "github.com/Luzin7/vozzera-backend/internal/shared/realtime"

type PresenceSnapshotPayload struct {
	Online       int64                   `json:"online"`
	Offline      int64                   `json:"offline"`
	Total        int64                   `json:"total"`
	OnlineUsers  []realtime.UserPresence `json:"online_users,omitempty"`
	OfflineUsers []realtime.UserPresence `json:"offline_users,omitempty"`
}
