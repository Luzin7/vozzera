package voice

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const sweepMaxEntries = 500

type Participant struct {
	SID      string `json:"sid"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

type AwayOwner struct {
	InstanceID uuid.UUID
	ConnID     uuid.UUID
}

type AwayEntry struct {
	RoomID   uuid.UUID
	UserID   uuid.UUID
	Username string
	Since    time.Time
	Owner    AwayOwner
}

type RoomSnapshot struct {
	Participants []Participant
	Away         []AwayEntry
}

type JoinResult struct {
	Participants []Participant
	ClearedAway  *AwayEntry
}

type LeaveResult struct {
	Participants []Participant
	Left         bool
	Emptied      bool
	RemovedAway  []AwayEntry
}

type EnterAwayResult struct {
	Accepted  bool
	MovedFrom *uuid.UUID
}

type BeatResult struct {
	Wiped bool
}

type VoiceRoomState interface {
	Join(ctx context.Context, roomID uuid.UUID, p Participant, at time.Time) (JoinResult, error)
	Leave(ctx context.Context, roomID uuid.UUID, sid string) (LeaveResult, error)
	Finish(ctx context.Context, roomID uuid.UUID) ([]AwayEntry, error)
	Snapshot(ctx context.Context, roomID uuid.UUID) (RoomSnapshot, error)
	EnterAway(ctx context.Context, entry AwayEntry) (EnterAwayResult, error)
	LeaveAway(ctx context.Context, userID uuid.UUID) (*AwayEntry, error)
	ReleaseAway(ctx context.Context, userID uuid.UUID, owner AwayOwner) (*AwayEntry, error)
	SweepDeadOwners(ctx context.Context) ([]AwayEntry, error)
	HydrateRoom(ctx context.Context, roomID uuid.UUID, participants []Participant) (bool, error)
	Beat(ctx context.Context, instanceID uuid.UUID) (BeatResult, error)
}
