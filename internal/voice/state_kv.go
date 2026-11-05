package voice

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

//go:embed scripts/*.lua
var scriptFiles embed.FS

const (
	kvOperationTimeout = 2 * time.Second
	instanceTTL        = 30 * time.Second
)

var errUnexpectedScriptReply = errors.New("resposta inesperada do script do key value")

type kvScripts struct {
	presenceJoin  *redis.Script
	presenceLeave *redis.Script
	roomFinished  *redis.Script
	awayEnter     *redis.Script
	awayLeave     *redis.Script
	awayRelease   *redis.Script
	awaySweep     *redis.Script
	hydrateRoom   *redis.Script
}

func loadScript(name string) *redis.Script {
	source, err := scriptFiles.ReadFile("scripts/" + name + ".lua")
	if err != nil {
		panic(fmt.Sprintf("script %s ausente do binário: %v", name, err))
	}
	return redis.NewScript(string(source))
}

func loadScripts() kvScripts {
	return kvScripts{
		presenceJoin:  loadScript("presence_join"),
		presenceLeave: loadScript("presence_leave"),
		roomFinished:  loadScript("room_finished"),
		awayEnter:     loadScript("away_enter"),
		awayLeave:     loadScript("away_leave"),
		awayRelease:   loadScript("away_release"),
		awaySweep:     loadScript("away_sweep"),
		hydrateRoom:   loadScript("hydrate_room"),
	}
}

type kvParticipant struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

type kvAwayEntry struct {
	Username   string `json:"username"`
	Since      int64  `json:"since"`
	ConnID     string `json:"conn_id"`
	InstanceID string `json:"instance_id"`
}

type kvVoiceRoomState struct {
	client  *redis.Client
	prefix  string
	scripts kvScripts
}

var _ VoiceRoomState = (*kvVoiceRoomState)(nil)

func NewKVVoiceRoomState(client *redis.Client, keyPrefix string) VoiceRoomState {
	return &kvVoiceRoomState{client: client, prefix: keyPrefix, scripts: loadScripts()}
}

func (s *kvVoiceRoomState) presenceKey(roomID uuid.UUID) string {
	return s.prefix + ":room:" + roomID.String() + ":presence"
}

func (s *kvVoiceRoomState) awayKey(roomID uuid.UUID) string {
	return s.prefix + ":room:" + roomID.String() + ":away"
}

func (s *kvVoiceRoomState) indexKey() string {
	return s.prefix + ":away:index"
}

func (s *kvVoiceRoomState) instanceKey(instanceID uuid.UUID) string {
	return s.prefix + ":instance:" + instanceID.String()
}

func (s *kvVoiceRoomState) epochKey() string {
	return s.prefix + ":epoch"
}

func (s *kvVoiceRoomState) run(ctx context.Context, script *redis.Script, keys []string, args ...any) ([]any, error) {
	ctx, cancel := context.WithTimeout(ctx, kvOperationTimeout)
	defer cancel()

	return script.Run(ctx, s.client, keys, args...).Slice()
}

func (s *kvVoiceRoomState) Join(ctx context.Context, roomID uuid.UUID, p Participant, at time.Time) (JoinResult, error) {
	value, err := json.Marshal(kvParticipant{UserID: p.UserID, Username: p.Username})
	if err != nil {
		return JoinResult{}, err
	}

	reply, err := s.run(ctx, s.scripts.presenceJoin,
		[]string{s.presenceKey(roomID), s.indexKey()},
		p.SID, string(value), p.UserID, at.UnixMilli(), s.prefix)
	if err != nil {
		return JoinResult{}, err
	}
	if len(reply) != 3 {
		return JoinResult{}, errUnexpectedScriptReply
	}

	participants, err := participantsFromFlat(reply[0])
	if err != nil {
		return JoinResult{}, err
	}
	cleared, err := clearedAwayFrom(reply[1], reply[2], p.UserID)
	if err != nil {
		return JoinResult{}, err
	}
	return JoinResult{Participants: participants, ClearedAway: cleared}, nil
}

func (s *kvVoiceRoomState) Leave(ctx context.Context, roomID uuid.UUID, sid string) (LeaveResult, error) {
	reply, err := s.run(ctx, s.scripts.presenceLeave,
		[]string{s.presenceKey(roomID), s.awayKey(roomID), s.indexKey()},
		sid, roomID.String())
	if err != nil {
		return LeaveResult{}, err
	}
	if len(reply) != 4 {
		return LeaveResult{}, errUnexpectedScriptReply
	}

	removed, _ := reply[0].(int64)
	participants, err := participantsFromFlat(reply[1])
	if err != nil {
		return LeaveResult{}, err
	}
	removedAway, err := awayEntriesInRoomFromFlat(roomID, reply[2])
	if err != nil {
		return LeaveResult{}, err
	}
	active, err := activeCountFrom(reply[3])
	if err != nil {
		return LeaveResult{}, err
	}
	return LeaveResult{Participants: participants, Left: removed == 1, Emptied: active == 0, RemovedAway: removedAway}, nil
}

func (s *kvVoiceRoomState) Finish(ctx context.Context, roomID uuid.UUID) ([]AwayEntry, error) {
	reply, err := s.run(ctx, s.scripts.roomFinished,
		[]string{s.presenceKey(roomID), s.awayKey(roomID), s.indexKey()},
		roomID.String())
	if err != nil {
		return nil, err
	}
	return awayEntriesInRoomFromFlat(roomID, any(reply))
}

func (s *kvVoiceRoomState) Snapshot(ctx context.Context, roomID uuid.UUID) (RoomSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, kvOperationTimeout)
	defer cancel()

	pipe := s.client.TxPipeline()
	presence := pipe.HGetAll(ctx, s.presenceKey(roomID))
	away := pipe.HGetAll(ctx, s.awayKey(roomID))
	if _, err := pipe.Exec(ctx); err != nil {
		return RoomSnapshot{}, err
	}

	participants, err := participantsFromMap(presence.Val())
	if err != nil {
		return RoomSnapshot{}, err
	}
	entries, err := awayEntriesFromMap(roomID, away.Val())
	if err != nil {
		return RoomSnapshot{}, err
	}
	return RoomSnapshot{Participants: participants, Away: entries}, nil
}

func (s *kvVoiceRoomState) EnterAway(ctx context.Context, entry AwayEntry) (EnterAwayResult, error) {
	value, err := json.Marshal(kvAwayEntry{
		Username:   entry.Username,
		Since:      entry.Since.UnixMilli(),
		ConnID:     entry.Owner.ConnID.String(),
		InstanceID: entry.Owner.InstanceID.String(),
	})
	if err != nil {
		return EnterAwayResult{}, err
	}

	reply, err := s.run(ctx, s.scripts.awayEnter,
		[]string{s.presenceKey(entry.RoomID), s.awayKey(entry.RoomID), s.indexKey()},
		entry.UserID.String(), string(value), entry.RoomID.String(), s.prefix)
	if err != nil {
		return EnterAwayResult{}, err
	}
	if len(reply) != 2 {
		return EnterAwayResult{}, errUnexpectedScriptReply
	}

	status, _ := reply[0].(string)
	if status != "accepted" {
		return EnterAwayResult{}, nil
	}
	return EnterAwayResult{Accepted: true, MovedFrom: optionalUUID(reply[1])}, nil
}

func (s *kvVoiceRoomState) LeaveAway(ctx context.Context, userID uuid.UUID) (*AwayEntry, error) {
	reply, err := s.run(ctx, s.scripts.awayLeave, []string{s.indexKey()}, userID.String(), s.prefix)
	if err != nil {
		return nil, err
	}
	return removedAwayFrom(reply, userID)
}

func (s *kvVoiceRoomState) ReleaseAway(ctx context.Context, userID uuid.UUID, owner AwayOwner) (*AwayEntry, error) {
	reply, err := s.run(ctx, s.scripts.awayRelease, []string{s.indexKey()},
		userID.String(), owner.ConnID.String(), owner.InstanceID.String(), s.prefix)
	if err != nil {
		return nil, err
	}
	return removedAwayFrom(reply, userID)
}

func (s *kvVoiceRoomState) SweepDeadOwners(ctx context.Context) ([]AwayEntry, error) {
	reply, err := s.run(ctx, s.scripts.awaySweep, []string{s.indexKey()}, s.prefix, sweepMaxEntries)
	if err != nil {
		return nil, err
	}

	entries := make([]AwayEntry, 0, len(reply)/3)
	for i := 0; i+2 < len(reply); i += 3 {
		roomID, roomErr := uuid.Parse(asString(reply[i]))
		userID, userErr := uuid.Parse(asString(reply[i+1]))
		if roomErr != nil || userErr != nil {
			return nil, errUnexpectedScriptReply
		}
		entry, err := awayEntryFromJSON(roomID, userID, asString(reply[i+2]))
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *kvVoiceRoomState) HydrateRoom(ctx context.Context, roomID uuid.UUID, participants []Participant) (bool, error) {
	if len(participants) == 0 {
		return false, nil
	}

	args := make([]any, 0, len(participants)*2)
	for _, p := range participants {
		value, err := json.Marshal(kvParticipant{UserID: p.UserID, Username: p.Username})
		if err != nil {
			return false, err
		}
		args = append(args, p.SID, string(value))
	}

	ctx, cancel := context.WithTimeout(ctx, kvOperationTimeout)
	defer cancel()

	written, err := s.scripts.hydrateRoom.Run(ctx, s.client, []string{s.presenceKey(roomID)}, args...).Int()
	if err != nil {
		return false, err
	}
	return written == 1, nil
}

func (s *kvVoiceRoomState) Beat(ctx context.Context, instanceID uuid.UUID) (BeatResult, error) {
	ctx, cancel := context.WithTimeout(ctx, kvOperationTimeout)
	defer cancel()

	pipe := s.client.Pipeline()
	pipe.Set(ctx, s.instanceKey(instanceID), "1", instanceTTL)
	epoch := pipe.SetNX(ctx, s.epochKey(), instanceID.String(), 0)
	if _, err := pipe.Exec(ctx); err != nil {
		return BeatResult{}, err
	}
	return BeatResult{Wiped: epoch.Val()}, nil
}

func activeCountFrom(value any) (int64, error) {
	count, ok := value.(int64)
	if !ok {
		return 0, errUnexpectedScriptReply
	}
	return count, nil
}

func asString(value any) string {
	text, _ := value.(string)
	return text
}

func optionalUUID(value any) *uuid.UUID {
	id, err := uuid.Parse(asString(value))
	if err != nil {
		return nil
	}
	return &id
}

func flatPairs(value any) ([]any, error) {
	flat, ok := value.([]any)
	if !ok || len(flat)%2 != 0 {
		return nil, errUnexpectedScriptReply
	}
	return flat, nil
}

func participantsFromFlat(value any) ([]Participant, error) {
	flat, err := flatPairs(value)
	if err != nil {
		return nil, err
	}

	fields := make(map[string]string, len(flat)/2)
	for i := 0; i < len(flat); i += 2 {
		fields[asString(flat[i])] = asString(flat[i+1])
	}
	return participantsFromMap(fields)
}

func participantsFromMap(fields map[string]string) ([]Participant, error) {
	participants := make([]Participant, 0, len(fields))
	for sid, raw := range fields {
		var stored kvParticipant
		if err := json.Unmarshal([]byte(raw), &stored); err != nil {
			return nil, err
		}
		participants = append(participants, Participant{SID: sid, UserID: stored.UserID, Username: stored.Username})
	}
	sort.Slice(participants, func(i, j int) bool { return participants[i].SID < participants[j].SID })
	return participants, nil
}

func awayEntryFromJSON(roomID, userID uuid.UUID, raw string) (AwayEntry, error) {
	var stored kvAwayEntry
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return AwayEntry{}, err
	}

	connID, connErr := uuid.Parse(stored.ConnID)
	instanceID, instanceErr := uuid.Parse(stored.InstanceID)
	if err := errors.Join(connErr, instanceErr); err != nil {
		return AwayEntry{}, err
	}
	return AwayEntry{
		RoomID:   roomID,
		UserID:   userID,
		Username: stored.Username,
		Since:    time.UnixMilli(stored.Since).UTC(),
		Owner:    AwayOwner{InstanceID: instanceID, ConnID: connID},
	}, nil
}

func awayEntriesFromMap(roomID uuid.UUID, fields map[string]string) ([]AwayEntry, error) {
	entries := make([]AwayEntry, 0, len(fields))
	for rawUserID, raw := range fields {
		userID, err := uuid.Parse(rawUserID)
		if err != nil {
			return nil, err
		}
		entry, err := awayEntryFromJSON(roomID, userID, raw)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].Since.Equal(entries[j].Since) {
			return entries[i].Since.Before(entries[j].Since)
		}
		return entries[i].UserID.String() < entries[j].UserID.String()
	})
	return entries, nil
}

func awayEntriesInRoomFromFlat(roomID uuid.UUID, value any) ([]AwayEntry, error) {
	flat, err := flatPairs(value)
	if err != nil {
		return nil, err
	}

	fields := make(map[string]string, len(flat)/2)
	for i := 0; i < len(flat); i += 2 {
		fields[asString(flat[i])] = asString(flat[i+1])
	}
	return awayEntriesFromMap(roomID, fields)
}

func clearedAwayFrom(roomValue, entryValue any, rawUserID string) (*AwayEntry, error) {
	roomID, err := uuid.Parse(asString(roomValue))
	if err != nil {
		return nil, nil
	}
	userID, err := uuid.Parse(rawUserID)
	if err != nil {
		return nil, err
	}
	entry, err := awayEntryFromJSON(roomID, userID, asString(entryValue))
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func removedAwayFrom(reply []any, userID uuid.UUID) (*AwayEntry, error) {
	if len(reply) == 0 {
		return nil, nil
	}
	if len(reply) != 2 {
		return nil, errUnexpectedScriptReply
	}

	roomID, err := uuid.Parse(asString(reply[0]))
	if err != nil {
		return nil, errUnexpectedScriptReply
	}
	entry, err := awayEntryFromJSON(roomID, userID, asString(reply[1]))
	if err != nil {
		return nil, err
	}
	return &entry, nil
}
