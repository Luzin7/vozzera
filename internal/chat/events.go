package chat

// O que o cliente PODE PEDIR (Commands/Actions)
const (
	CmdSubscribe   = "room.subscribe"
	CmdUnsubscribe = "room.unsubscribe"
	CmdMessage     = "message"
	CmdTypingStart = "typing.start"
	CmdTypingStop  = "typing.stop"
	CmdPing        = "ping"
)

// Eventos emitidos pelo servidor
const (
	EventMessageCreated  = "message.created"
	EventMessageUpdated  = "message.updated"
	EventMessageDeleted  = "message.deleted"
	EventReactionAdded   = "reaction_added"
	EventReactionRemoved = "reaction_removed"

	EventSystemError = "system.error"
	EventPong        = "pong"
)
