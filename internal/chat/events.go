package chat

// O que o cliente PODE PEDIR (Commands/Actions)
const (
	CmdSubscribe   = "room.subscribe"
	CmdUnsubscribe = "room.unsubscribe"
	CmdMessage     = "message"
	CmdTypingStart = "typing.start"
	CmdTypingStop  = "typing.stop"
)

// Eventos emitidos pelo servidor
const (
	EventRoomCreated = "room.created"
	EventRoomUpdated = "room.updated"
	EventRoomDeleted = "room.deleted"

	EventMessageCreated = "message.created"
	EventMessageUpdated = "message.updated"
	EventMessageDeleted = "message.deleted"

	EventVoicePresenceSnapshot = "voice.presence.snapshot"

	EventSystemError = "system.error"
)
