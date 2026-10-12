package ws

import (
	"slices"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
)

func TestConnectTopics_Membro(t *testing.T) {
	got := connectTopics(httpx.UserClaims{Role: httpx.RoleUser})

	want := []realtime.Topic{realtime.GlobalPresenceTopic, realtime.MemberRoomListTopic}
	if !slices.Equal(got, want) {
		t.Errorf("connectTopics = %v, want %v", got, want)
	}
}

func TestConnectTopics_Staff(t *testing.T) {
	got := connectTopics(httpx.UserClaims{Role: httpx.RoleMod})

	want := []realtime.Topic{realtime.GlobalPresenceTopic, realtime.StaffRoomListTopic}
	if !slices.Equal(got, want) {
		t.Errorf("connectTopics = %v, want %v", got, want)
	}
}
