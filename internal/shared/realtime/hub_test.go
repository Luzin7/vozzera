package realtime

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHub_Register(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run()

	client := NewClient(hub, nil, uuid.New(), "usuário-teste", uuid.New(), nil)
	client.Topics = make(map[Topic]bool)

	hub.Register(client)
	hub.Sync(ctx)

	if !hub.clients[client] {
		t.Error("cliente não registrado")
	}
}

func TestHub_Unregister(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run()

	client := NewClient(hub, nil, uuid.New(), "usuário-teste", uuid.New(), nil)
	client.Topics = make(map[Topic]bool)

	hub.Register(client)
	hub.Sync(ctx)

	hub.Unregister(client)

	// espera o hub processar o unregister, que fecha o canal send
	_, ok := <-client.send
	if ok {
		t.Error("canal send deveria estar fechado após unregister")
	}
}

func TestHub_SubscribeEPublish(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run()

	client := NewClient(hub, nil, uuid.New(), "usuário-teste", uuid.New(), nil)
	client.Topics = make(map[Topic]bool)
	topic := Topic("room:" + uuid.New().String())

	hub.Register(client)
	hub.Sync(ctx)

	hub.Subscribe(client, topic)
	hub.Sync(ctx)

	if !hub.topics[topic][client] {
		t.Error("cliente não inscrito no tópico")
	}
	if !client.Topics[topic] {
		t.Error("tópico não registrado nos topics do cliente")
	}

	data := []byte(`{"type":"test"}`)
	pubErr := hub.Publish(ctx, topic, Envelope{V: 1, Type: "test", Topic: topic, TS: time.Now(), Data: data})
	if pubErr != nil {
		t.Fatalf("Publish erro: %v", pubErr)
	}

	select {
	case msg := <-client.send:
		if len(msg) == 0 {
			t.Error("mensagem publish vazia")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout aguardando mensagem no send do cliente")
	}
}

func TestHub_RevokeRemoveClienteDaSessionID(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run()

	sessionID := uuid.New()
	client1 := NewClient(hub, nil, uuid.New(), "alice", sessionID, nil)
	client1.Topics = make(map[Topic]bool)
	client2 := NewClient(hub, nil, uuid.New(), "bob", uuid.New(), nil)
	client2.Topics = make(map[Topic]bool)

	hub.Register(client1)
	hub.Register(client2)
	hub.Sync(ctx)

	hub.Revoke(ctx, sessionID)
	hub.Sync(ctx)

	if hub.clients[client1] {
		t.Error("client1 não deveria estar registrado após revoke")
	}
	if !hub.clients[client2] {
		t.Error("client2 deveria permanecer registrado")
	}
}
