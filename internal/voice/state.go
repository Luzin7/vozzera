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
	Participants     []Participant
	Away             []AwayEntry
	SessionStartedAt *time.Time
}

type JoinInput struct {
	RoomID        uuid.UUID
	Participant   Participant
	At            time.Time
	RoomCreatedAt time.Time
}

type LeaveInput struct {
	RoomID        uuid.UUID
	SID           string
	At            time.Time
	RoomCreatedAt time.Time
}

type RoomHydration struct {
	RoomID       uuid.UUID
	Participants []Participant
	CreatedAt    time.Time
}

type HydrateResult struct {
	Written          bool
	SessionStartedAt *time.Time
}

type JoinResult struct {
	Participants     []Participant
	ClearedAway      *AwayEntry
	SessionStartedAt *time.Time
}

type LeaveResult struct {
	Participants     []Participant
	Left             bool
	Emptied          bool
	RemovedAway      []AwayEntry
	SessionStartedAt *time.Time
}

type EnterAwayResult struct {
	Accepted  bool
	MovedFrom *uuid.UUID
}

type BeatResult struct {
	Wiped bool
}

type VoiceRoomState interface {
	Join(ctx context.Context, in JoinInput) (JoinResult, error)
	Leave(ctx context.Context, in LeaveInput) (LeaveResult, error)
	Finish(ctx context.Context, roomID uuid.UUID) ([]AwayEntry, error)
	Snapshot(ctx context.Context, roomID uuid.UUID) (RoomSnapshot, error)
	EnterAway(ctx context.Context, entry AwayEntry) (EnterAwayResult, error)
	LeaveAway(ctx context.Context, userID uuid.UUID) (*AwayEntry, error)
	ReleaseAway(ctx context.Context, userID uuid.UUID, owner AwayOwner) (*AwayEntry, error)
	SweepDeadOwners(ctx context.Context) ([]AwayEntry, error)
	HydrateRoom(ctx context.Context, in RoomHydration) (HydrateResult, error)
	Beat(ctx context.Context, instanceID uuid.UUID) (BeatResult, error)
}
