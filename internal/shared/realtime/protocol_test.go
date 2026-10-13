package realtime

import "testing"

func TestRoomListTopicFor(t *testing.T) {
	if got := RoomListTopicFor(true); got != StaffRoomListTopic {
		t.Errorf("RoomListTopicFor(true) = %q, want %q", got, StaffRoomListTopic)
	}
	if got := RoomListTopicFor(false); got != MemberRoomListTopic {
		t.Errorf("RoomListTopicFor(false) = %q, want %q", got, MemberRoomListTopic)
	}
}
