package voice

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type contractHarness struct {
	state          VoiceRoomState
	expireInstance func(instanceID uuid.UUID)
}

type newContractHarness func(t *testing.T) contractHarness

var contractEpoch = time.UnixMilli(1_700_000_000_000)

func at(offset time.Duration) time.Time {
	return contractEpoch.Add(offset)
}

func newParticipant(overrides ...func(*Participant)) Participant {
	p := Participant{SID: "sid-" + uuid.NewString(), UserID: uuid.NewString(), Username: "alice"}
	for _, override := range overrides {
		override(&p)
	}
	return p
}

func withUser(userID uuid.UUID) func(*Participant) {
	return func(p *Participant) { p.UserID = userID.String() }
}

func newAwayEntry(roomID uuid.UUID, overrides ...func(*AwayEntry)) AwayEntry {
	e := AwayEntry{
		RoomID:   roomID,
		UserID:   uuid.New(),
		Username: "alice",
		Since:    at(0),
		Owner:    AwayOwner{InstanceID: uuid.New(), ConnID: uuid.New()},
	}
	for _, override := range overrides {
		override(&e)
	}
	return e
}

func forUser(userID uuid.UUID) func(*AwayEntry) {
	return func(e *AwayEntry) { e.UserID = userID }
}

func since(t time.Time) func(*AwayEntry) {
	return func(e *AwayEntry) { e.Since = t }
}

func ownedBy(owner AwayOwner) func(*AwayEntry) {
	return func(e *AwayEntry) { e.Owner = owner }
}

func mustJoin(t *testing.T, s VoiceRoomState, roomID uuid.UUID, p Participant, when time.Time) JoinResult {
	t.Helper()
	result, err := s.Join(context.Background(), roomID, p, when)
	if err != nil {
		t.Fatalf("Join erro: %v", err)
	}
	return result
}

func mustEnterAway(t *testing.T, s VoiceRoomState, entry AwayEntry) EnterAwayResult {
	t.Helper()
	result, err := s.EnterAway(context.Background(), entry)
	if err != nil {
		t.Fatalf("EnterAway erro: %v", err)
	}
	return result
}

func mustSnapshot(t *testing.T, s VoiceRoomState, roomID uuid.UUID) RoomSnapshot {
	t.Helper()
	snapshot, err := s.Snapshot(context.Background(), roomID)
	if err != nil {
		t.Fatalf("Snapshot erro: %v", err)
	}
	return snapshot
}

func awayUserIDs(snapshot RoomSnapshot) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(snapshot.Away))
	for _, entry := range snapshot.Away {
		ids = append(ids, entry.UserID)
	}
	return ids
}

func assertAwayUsers(t *testing.T, snapshot RoomSnapshot, want ...uuid.UUID) {
	t.Helper()
	got := awayUserIDs(snapshot)
	if len(got) != len(want) {
		t.Fatalf("away = %v, want %v", got, want)
	}
	for _, id := range want {
		if !containsUUID(got, id) {
			t.Fatalf("away = %v, want %v", got, want)
		}
	}
}

func containsUUID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func participantOf(userID uuid.UUID, sid string) Participant {
	return Participant{SID: sid, UserID: userID.String(), Username: "alice"}
}

func runVoiceRoomStateContract(t *testing.T, newHarness newContractHarness) {
	ctx := context.Background()

	t.Run("EnterAway rejeitado quando a sala está vazia", func(t *testing.T) {
		s := newHarness(t).state
		entry := newAwayEntry(uuid.New())

		result := mustEnterAway(t, s, entry)

		if result.Accepted {
			t.Fatal("enter deveria ser rejeitado")
		}
		assertAwayUsers(t, mustSnapshot(t, s, entry.RoomID))
	})

	t.Run("EnterAway rejeitado quando só o próprio usuário está na sala", func(t *testing.T) {
		s := newHarness(t).state
		roomID, userID := uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(userID, "sid-a"), at(0))

		result := mustEnterAway(t, s, newAwayEntry(roomID, forUser(userID)))

		if result.Accepted {
			t.Fatal("enter deveria ser rejeitado")
		}
	})

	t.Run("EnterAway rejeitado quando os outros estão ausentes", func(t *testing.T) {
		s := newHarness(t).state
		roomID, caller, other, third := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(caller, "sid-a"), at(0))
		mustJoin(t, s, roomID, participantOf(other, "sid-b"), at(0))
		mustJoin(t, s, roomID, participantOf(third, "sid-c"), at(0))
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(other)))
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(third)))

		result := mustEnterAway(t, s, newAwayEntry(roomID, forUser(caller)))

		if result.Accepted {
			t.Fatal("enter deveria ser rejeitado: só há ausentes além do chamador")
		}
		assertAwayUsers(t, mustSnapshot(t, s, roomID), other, third)
	})

	t.Run("EnterAway aceito com outro participante ativo", func(t *testing.T) {
		s := newHarness(t).state
		roomID, caller, other := uuid.New(), uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(caller, "sid-a"), at(0))
		mustJoin(t, s, roomID, participantOf(other, "sid-b"), at(0))
		entry := newAwayEntry(roomID, forUser(caller), since(at(5*time.Second)))

		result := mustEnterAway(t, s, entry)

		if !result.Accepted || result.MovedFrom != nil {
			t.Fatalf("result = %+v, want aceito sem MovedFrom", result)
		}
		snapshot := mustSnapshot(t, s, roomID)
		assertAwayUsers(t, snapshot, caller)
		if got := snapshot.Away[0]; got.Owner != entry.Owner || got.Username != entry.Username || !got.Since.Equal(entry.Since) || got.RoomID != roomID {
			t.Errorf("entrada = %+v, want %+v", got, entry)
		}
	})

	t.Run("EnterAway em outra sala move a entrada", func(t *testing.T) {
		s := newHarness(t).state
		first, second, caller, other := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		for _, roomID := range []uuid.UUID{first, second} {
			mustJoin(t, s, roomID, participantOf(other, "sid-"+roomID.String()), at(0))
		}
		mustEnterAway(t, s, newAwayEntry(first, forUser(caller)))

		result := mustEnterAway(t, s, newAwayEntry(second, forUser(caller)))

		if !result.Accepted || result.MovedFrom == nil || *result.MovedFrom != first {
			t.Fatalf("result = %+v, want MovedFrom = %s", result, first)
		}
		assertAwayUsers(t, mustSnapshot(t, s, first))
		assertAwayUsers(t, mustSnapshot(t, s, second), caller)
	})

	t.Run("EnterAway reenviado mantém since", func(t *testing.T) {
		s := newHarness(t).state
		roomID, caller, other := uuid.New(), uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(other, "sid-b"), at(0))
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(caller), since(at(time.Second))))

		result := mustEnterAway(t, s, newAwayEntry(roomID, forUser(caller), since(at(time.Minute))))

		if !result.Accepted || result.MovedFrom != nil {
			t.Fatalf("result = %+v, want aceito sem MovedFrom", result)
		}
		snapshot := mustSnapshot(t, s, roomID)
		assertAwayUsers(t, snapshot, caller)
		if !snapshot.Away[0].Since.Equal(at(time.Second)) {
			t.Errorf("since = %v, want %v", snapshot.Away[0].Since, at(time.Second))
		}
	})

	t.Run("EnterAway reenviado passa a pertencer à nova conexão", func(t *testing.T) {
		s := newHarness(t).state
		roomID, caller, other := uuid.New(), uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(other, "sid-b"), at(0))
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(caller)))
		newOwner := AwayOwner{InstanceID: uuid.New(), ConnID: uuid.New()}

		mustEnterAway(t, s, newAwayEntry(roomID, forUser(caller), ownedBy(newOwner)))

		if got := mustSnapshot(t, s, roomID).Away[0].Owner; got != newOwner {
			t.Errorf("owner = %+v, want %+v", got, newOwner)
		}
	})

	t.Run("Leave remove todos os ausentes quando não sobra ativo", func(t *testing.T) {
		s := newHarness(t).state
		roomID, active, awayA, awayB := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(active, "sid-active"), at(0))
		mustJoin(t, s, roomID, participantOf(awayA, "sid-a"), at(0))
		mustJoin(t, s, roomID, participantOf(awayB, "sid-b"), at(0))
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(awayA)))
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(awayB)))

		result, err := s.Leave(ctx, roomID, "sid-active")

		if err != nil {
			t.Fatalf("Leave erro: %v", err)
		}
		if !result.Left || len(result.RemovedAway) != 2 {
			t.Fatalf("result = %+v, want Left e 2 ausentes removidos", result)
		}
		assertAwayUsers(t, mustSnapshot(t, s, roomID))
		for _, userID := range []uuid.UUID{awayA, awayB} {
			if stale, _ := s.LeaveAway(ctx, userID); stale != nil {
				t.Errorf("índice deveria estar limpo, restou %+v", stale)
			}
		}
	})

	t.Run("Leave mantém os ausentes quando sobra um ativo", func(t *testing.T) {
		s := newHarness(t).state
		roomID, leaver, stays, away := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(leaver, "sid-leaver"), at(0))
		mustJoin(t, s, roomID, participantOf(stays, "sid-stays"), at(0))
		mustJoin(t, s, roomID, participantOf(away, "sid-away"), at(0))
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(away)))

		result, err := s.Leave(ctx, roomID, "sid-leaver")

		if err != nil {
			t.Fatalf("Leave erro: %v", err)
		}
		if !result.Left || len(result.RemovedAway) != 0 || len(result.Participants) != 2 {
			t.Fatalf("result = %+v, want Left, 2 participantes e nenhum ausente removido", result)
		}
		assertAwayUsers(t, mustSnapshot(t, s, roomID), away)
	})

	t.Run("Leave repetido continua reportando sala sem ativos", func(t *testing.T) {
		s := newHarness(t).state
		roomID, active, away := uuid.New(), uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(active, "sid-active"), at(0))
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(away)))

		first, firstErr := s.Leave(ctx, roomID, "sid-active")
		second, secondErr := s.Leave(ctx, roomID, "sid-active")

		if firstErr != nil || secondErr != nil {
			t.Fatalf("erros = %v, %v", firstErr, secondErr)
		}
		if !first.Emptied || len(first.RemovedAway) != 1 {
			t.Errorf("primeiro = %+v, want Emptied e 1 ausente removido", first)
		}
		if !second.Emptied || second.Left || len(second.RemovedAway) != 0 {
			t.Errorf("segundo = %+v, want Emptied sem Left nem ausentes removidos", second)
		}
	})

	t.Run("Leave com ativo restante não reporta sala sem ativos", func(t *testing.T) {
		s := newHarness(t).state
		roomID := uuid.New()
		mustJoin(t, s, roomID, participantOf(uuid.New(), "sid-a"), at(0))
		mustJoin(t, s, roomID, participantOf(uuid.New(), "sid-b"), at(0))

		result, err := s.Leave(ctx, roomID, "sid-a")

		if err != nil || result.Emptied {
			t.Fatalf("result = %+v, err = %v, want não Emptied", result, err)
		}
	})

	t.Run("Leave de sid desconhecido não remove nada", func(t *testing.T) {
		s := newHarness(t).state
		roomID, other, away := uuid.New(), uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(other, "sid-b"), at(0))
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(away)))

		result, err := s.Leave(ctx, roomID, "sid-inexistente")

		if err != nil {
			t.Fatalf("Leave erro: %v", err)
		}
		if result.Left || len(result.RemovedAway) != 0 {
			t.Fatalf("result = %+v, want nada removido", result)
		}
		assertAwayUsers(t, mustSnapshot(t, s, roomID), away)
	})

	t.Run("Finish limpa presença e ausentes", func(t *testing.T) {
		s := newHarness(t).state
		roomID, other, away := uuid.New(), uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(other, "sid-b"), at(0))
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(away)))

		removed, err := s.Finish(ctx, roomID)

		if err != nil {
			t.Fatalf("Finish erro: %v", err)
		}
		if len(removed) != 1 || removed[0].UserID != away {
			t.Fatalf("removed = %+v, want o ausente", removed)
		}
		snapshot := mustSnapshot(t, s, roomID)
		if len(snapshot.Participants) != 0 || len(snapshot.Away) != 0 {
			t.Errorf("snapshot = %+v, want vazio", snapshot)
		}
		if stale, _ := s.LeaveAway(ctx, away); stale != nil {
			t.Errorf("índice deveria estar limpo, restou %+v", stale)
		}
	})

	t.Run("Join limpa o próprio ausente quando at >= since", func(t *testing.T) {
		s := newHarness(t).state
		roomID, caller, other := uuid.New(), uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(other, "sid-b"), at(0))
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(caller), since(at(10*time.Second))))

		result := mustJoin(t, s, roomID, participantOf(caller, "sid-new"), at(10*time.Second))

		if result.ClearedAway == nil || result.ClearedAway.UserID != caller || result.ClearedAway.RoomID != roomID {
			t.Fatalf("ClearedAway = %+v, want a entrada do chamador", result.ClearedAway)
		}
		assertAwayUsers(t, mustSnapshot(t, s, roomID))
	})

	t.Run("Join anterior ao since não limpa o ausente", func(t *testing.T) {
		s := newHarness(t).state
		roomID, caller, other := uuid.New(), uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(other, "sid-b"), at(0))
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(caller), since(at(10*time.Second))))

		result := mustJoin(t, s, roomID, participantOf(caller, "sid-late"), at(10*time.Second-time.Millisecond))

		if result.ClearedAway != nil {
			t.Fatalf("ClearedAway = %+v, want nil", result.ClearedAway)
		}
		assertAwayUsers(t, mustSnapshot(t, s, roomID), caller)
	})

	t.Run("Join em outra sala limpa o ausente de qualquer sala", func(t *testing.T) {
		s := newHarness(t).state
		away, other, newRoom, oldRoom := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		mustJoin(t, s, oldRoom, participantOf(other, "sid-b"), at(0))
		mustEnterAway(t, s, newAwayEntry(oldRoom, forUser(away), since(at(time.Second))))

		result := mustJoin(t, s, newRoom, participantOf(away, "sid-new"), at(2*time.Second))

		if result.ClearedAway == nil || result.ClearedAway.RoomID != oldRoom {
			t.Fatalf("ClearedAway = %+v, want entrada da sala antiga", result.ClearedAway)
		}
		assertAwayUsers(t, mustSnapshot(t, s, oldRoom))
	})

	t.Run("LeaveAway remove a entrada onde estiver", func(t *testing.T) {
		s := newHarness(t).state
		roomID, caller, other := uuid.New(), uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(other, "sid-b"), at(0))
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(caller)))

		removed, err := s.LeaveAway(ctx, caller)

		if err != nil {
			t.Fatalf("LeaveAway erro: %v", err)
		}
		if removed == nil || removed.RoomID != roomID || removed.UserID != caller {
			t.Fatalf("removed = %+v, want a entrada do chamador", removed)
		}
		assertAwayUsers(t, mustSnapshot(t, s, roomID))
	})

	t.Run("LeaveAway sem entrada devolve nil", func(t *testing.T) {
		s := newHarness(t).state

		removed, err := s.LeaveAway(ctx, uuid.New())

		if err != nil || removed != nil {
			t.Fatalf("removed = %+v, err = %v, want nil, nil", removed, err)
		}
	})

	t.Run("ReleaseAway exige conexão e instância iguais", func(t *testing.T) {
		s := newHarness(t).state
		roomID, caller, other := uuid.New(), uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(other, "sid-b"), at(0))
		owner := AwayOwner{InstanceID: uuid.New(), ConnID: uuid.New()}
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(caller), ownedBy(owner)))

		for name, wrong := range map[string]AwayOwner{
			"outra conexão":   {InstanceID: owner.InstanceID, ConnID: uuid.New()},
			"outra instância": {InstanceID: uuid.New(), ConnID: owner.ConnID},
		} {
			removed, err := s.ReleaseAway(ctx, caller, wrong)
			if err != nil || removed != nil {
				t.Fatalf("%s: removed = %+v, err = %v, want nil, nil", name, removed, err)
			}
		}
		assertAwayUsers(t, mustSnapshot(t, s, roomID), caller)

		removed, err := s.ReleaseAway(ctx, caller, owner)

		if err != nil || removed == nil || removed.UserID != caller {
			t.Fatalf("removed = %+v, err = %v, want a entrada", removed, err)
		}
		assertAwayUsers(t, mustSnapshot(t, s, roomID))
	})

	t.Run("SweepDeadOwners remove entradas de instâncias sem heartbeat", func(t *testing.T) {
		h := newHarness(t)
		s := h.state
		roomID, deadUser, liveUser, other := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		deadInstance, liveInstance := uuid.New(), uuid.New()
		mustJoin(t, s, roomID, participantOf(other, "sid-b"), at(0))
		for _, instance := range []uuid.UUID{deadInstance, liveInstance} {
			if _, err := s.Beat(ctx, instance); err != nil {
				t.Fatalf("Beat erro: %v", err)
			}
		}
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(deadUser), ownedBy(AwayOwner{InstanceID: deadInstance, ConnID: uuid.New()})))
		mustEnterAway(t, s, newAwayEntry(roomID, forUser(liveUser), ownedBy(AwayOwner{InstanceID: liveInstance, ConnID: uuid.New()})))
		h.expireInstance(deadInstance)

		removed, err := s.SweepDeadOwners(ctx)

		if err != nil {
			t.Fatalf("SweepDeadOwners erro: %v", err)
		}
		if len(removed) != 1 || removed[0].UserID != deadUser || removed[0].RoomID != roomID {
			t.Fatalf("removed = %+v, want só a entrada da instância morta", removed)
		}
		assertAwayUsers(t, mustSnapshot(t, s, roomID), liveUser)
	})

	t.Run("HydrateRoom não sobrescreve sala existente", func(t *testing.T) {
		s := newHarness(t).state
		roomID := uuid.New()
		existing := newParticipant()
		mustJoin(t, s, roomID, existing, at(0))

		written, err := s.HydrateRoom(ctx, roomID, []Participant{newParticipant()})

		if err != nil || written {
			t.Fatalf("written = %v, err = %v, want false, nil", written, err)
		}
		participants := mustSnapshot(t, s, roomID).Participants
		if len(participants) != 1 || participants[0].SID != existing.SID {
			t.Errorf("participants = %+v, want só o existente", participants)
		}
	})

	t.Run("HydrateRoom preenche sala vazia", func(t *testing.T) {
		s := newHarness(t).state
		roomID := uuid.New()
		hydrated := []Participant{newParticipant(), newParticipant()}

		written, err := s.HydrateRoom(ctx, roomID, hydrated)

		if err != nil || !written {
			t.Fatalf("written = %v, err = %v, want true, nil", written, err)
		}
		if got := len(mustSnapshot(t, s, roomID).Participants); got != 2 {
			t.Errorf("participantes = %d, want 2", got)
		}
	})

	t.Run("HydrateRoom sem participantes não escreve", func(t *testing.T) {
		s := newHarness(t).state

		written, err := s.HydrateRoom(ctx, uuid.New(), nil)

		if err != nil || written {
			t.Fatalf("written = %v, err = %v, want false, nil", written, err)
		}
	})

	t.Run("Beat reporta Wiped só na primeira chamada", func(t *testing.T) {
		s := newHarness(t).state
		first, second := uuid.New(), uuid.New()

		firstResult, err := s.Beat(ctx, first)
		if err != nil || !firstResult.Wiped {
			t.Fatalf("primeira chamada = %+v, err = %v, want Wiped", firstResult, err)
		}
		repeated, err := s.Beat(ctx, first)
		if err != nil || repeated.Wiped {
			t.Fatalf("segunda chamada = %+v, err = %v, want não Wiped", repeated, err)
		}
		otherInstance, err := s.Beat(ctx, second)
		if err != nil || otherInstance.Wiped {
			t.Fatalf("outra instância = %+v, err = %v, want não Wiped", otherInstance, err)
		}
	})

	t.Run("Snapshot de sala desconhecida é vazio", func(t *testing.T) {
		s := newHarness(t).state

		snapshot := mustSnapshot(t, s, uuid.New())

		if len(snapshot.Participants) != 0 || len(snapshot.Away) != 0 {
			t.Errorf("snapshot = %+v, want vazio", snapshot)
		}
	})

	t.Run("Join devolve todos os participantes da sala", func(t *testing.T) {
		s := newHarness(t).state
		roomID := uuid.New()
		mustJoin(t, s, roomID, newParticipant(), at(0))

		result := mustJoin(t, s, roomID, newParticipant(), at(0))

		if len(result.Participants) != 2 {
			t.Errorf("participantes = %d, want 2", len(result.Participants))
		}
	})

	t.Run("operações repetidas deixam o mesmo estado", func(t *testing.T) {
		type operation func(s VoiceRoomState, roomID, caller, other uuid.UUID)
		entryFor := func(roomID, caller uuid.UUID) AwayEntry {
			return newAwayEntry(roomID, forUser(caller), since(at(time.Second)),
				ownedBy(AwayOwner{InstanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), ConnID: uuid.MustParse("00000000-0000-0000-0000-000000000002")}))
		}
		operations := map[string]operation{
			"Join": func(s VoiceRoomState, roomID, caller, _ uuid.UUID) {
				s.Join(ctx, roomID, participantOf(caller, "sid-repeat"), at(time.Minute))
			},
			"Leave":  func(s VoiceRoomState, roomID, _, _ uuid.UUID) { s.Leave(ctx, roomID, "sid-other") },
			"Finish": func(s VoiceRoomState, roomID, _, _ uuid.UUID) { s.Finish(ctx, roomID) },
			"EnterAway": func(s VoiceRoomState, roomID, caller, _ uuid.UUID) {
				s.EnterAway(ctx, entryFor(roomID, caller))
			},
			"LeaveAway": func(s VoiceRoomState, _, caller, _ uuid.UUID) { s.LeaveAway(ctx, caller) },
			"ReleaseAway": func(s VoiceRoomState, roomID, caller, _ uuid.UUID) {
				s.ReleaseAway(ctx, caller, entryFor(roomID, caller).Owner)
			},
			"SweepDeadOwners": func(s VoiceRoomState, _, _, _ uuid.UUID) { s.SweepDeadOwners(ctx) },
			"HydrateRoom": func(s VoiceRoomState, roomID, _, _ uuid.UUID) {
				s.HydrateRoom(ctx, roomID, []Participant{participantOf(uuid.MustParse("00000000-0000-0000-0000-000000000009"), "sid-hydrated")})
			},
			"Beat": func(s VoiceRoomState, _, _, _ uuid.UUID) {
				s.Beat(ctx, uuid.MustParse("00000000-0000-0000-0000-000000000001"))
			},
		}

		for name, run := range operations {
			t.Run(name, func(t *testing.T) {
				s := newHarness(t).state
				roomID, caller, other := uuid.New(), uuid.New(), uuid.New()
				mustJoin(t, s, roomID, participantOf(other, "sid-other"), at(0))
				mustJoin(t, s, roomID, participantOf(caller, "sid-caller"), at(0))
				mustEnterAway(t, s, entryFor(roomID, caller))

				run(s, roomID, caller, other)
				once := mustSnapshot(t, s, roomID)
				run(s, roomID, caller, other)
				twice := mustSnapshot(t, s, roomID)

				assertSameSnapshot(t, once, twice)
			})
		}
	})

	t.Run("EnterAway concorrente com Leave não deixa ausente em sala vazia", func(t *testing.T) {
		s := newHarness(t).state

		for range 50 {
			roomID, caller, other := uuid.New(), uuid.New(), uuid.New()
			mustJoin(t, s, roomID, participantOf(caller, "sid-caller"), at(0))
			mustJoin(t, s, roomID, participantOf(other, "sid-other"), at(0))

			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				s.EnterAway(ctx, newAwayEntry(roomID, forUser(caller)))
			}()
			go func() {
				defer wg.Done()
				s.Leave(ctx, roomID, "sid-other")
			}()
			wg.Wait()

			assertAwayUsers(t, mustSnapshot(t, s, roomID))
		}
	})
}

func assertSameSnapshot(t *testing.T, want, got RoomSnapshot) {
	t.Helper()
	if len(want.Participants) != len(got.Participants) || len(want.Away) != len(got.Away) {
		t.Fatalf("snapshot mudou: %+v -> %+v", want, got)
	}
	for _, entry := range want.Away {
		if !containsAwayEntry(got.Away, entry) {
			t.Fatalf("snapshot mudou: %+v -> %+v", want, got)
		}
	}
	for _, p := range want.Participants {
		if !containsParticipant(got.Participants, p) {
			t.Fatalf("snapshot mudou: %+v -> %+v", want, got)
		}
	}
}

func containsAwayEntry(entries []AwayEntry, want AwayEntry) bool {
	for _, entry := range entries {
		if entry.UserID == want.UserID && entry.RoomID == want.RoomID && entry.Owner == want.Owner && entry.Since.Equal(want.Since) {
			return true
		}
	}
	return false
}

func containsParticipant(participants []Participant, want Participant) bool {
	for _, p := range participants {
		if p == want {
			return true
		}
	}
	return false
}
