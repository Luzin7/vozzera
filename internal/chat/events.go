package chat

// O que o cliente PODE PEDIR (Commands/Actions)
const (
	CmdSubscribe   = "room.subscribe"
	CmdUnsubscribe = "room.unsubscribe"
	CmdMessage     = "message"
	CmdTypingStart = "typing.start"
	CmdTypingStop  = "typing.stop"
)

// O que o servidor AVISA QUE ACONTECEU (Events)
const (
	EventRoomCreated = "room.created"
	EventRoomUpdated = "room.updated"
	EventRoomDeleted = "room.deleted"

	EventMessageCreated = "message.created"
	EventMessageUpdated = "message.updated"
	EventMessageDeleted = "message.deleted"

	EventSystemError = "system.error"
)
