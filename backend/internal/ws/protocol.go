// Package ws implements D1: the WebSocket protocol and connection handling.
package ws

import "encoding/json"

// ClientMessageType enumerates what a client may send. These names are the
// protocol specified in D1.
type ClientMessageType string

const (
	MsgAuthenticate  ClientMessageType = "AUTHENTICATE"
	MsgJoinGame      ClientMessageType = "JOIN_GAME"
	MsgLeaveGame     ClientMessageType = "LEAVE_GAME"
	MsgPlayMove      ClientMessageType = "PLAY_MOVE"
	MsgPass          ClientMessageType = "PASS"
	MsgResign        ClientMessageType = "RESIGN"
	MsgMarkDeadGroup ClientMessageType = "MARK_DEAD_GROUP"
	MsgConfirmScore  ClientMessageType = "CONFIRM_SCORE"
	MsgDisputeScore  ClientMessageType = "DISPUTE_SCORE"
	MsgChatMessage   ClientMessageType = "CHAT_MESSAGE"
	MsgHeartbeat     ClientMessageType = "HEARTBEAT"
)

// ServerMessageType enumerates what the server emits.
type ServerMessageType string

const (
	MsgAuthenticated  ServerMessageType = "AUTHENTICATED"
	MsgGameStarted    ServerMessageType = "GAME_STARTED"
	MsgGameSnapshot   ServerMessageType = "GAME_SNAPSHOT"
	MsgMoveAccepted   ServerMessageType = "MOVE_ACCEPTED"
	MsgMoveRejected   ServerMessageType = "MOVE_REJECTED"
	MsgMovePlayed     ServerMessageType = "MOVE_PLAYED"
	MsgClockUpdate    ServerMessageType = "CLOCK_UPDATE"
	MsgScoringStarted ServerMessageType = "SCORING_STARTED"
	MsgScoreUpdated   ServerMessageType = "SCORE_UPDATED"
	MsgGameEnded      ServerMessageType = "GAME_ENDED"
	MsgChatReceived   ServerMessageType = "CHAT_RECEIVED"
	MsgPresence       ServerMessageType = "PRESENCE"
	MsgError          ServerMessageType = "ERROR"
	MsgPong           ServerMessageType = "PONG"
)

// Envelope is the wire frame. `Seq` echoes a client's request id so a client
// can correlate MOVE_ACCEPTED / MOVE_REJECTED with the move it sent.
type Envelope struct {
	Type    string          `json:"type"`
	Seq     int64           `json:"seq,omitempty"`
	GameID  string          `json:"gameId,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// AuthenticatePayload carries the access token when it is not in the URL.
type AuthenticatePayload struct {
	Token string `json:"token"`
}

// JoinGamePayload asks to join or spectate a game.
type JoinGamePayload struct {
	GameID string `json:"gameId"`
}

// PlayMovePayload is a stone placement.
type PlayMovePayload struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// MarkDeadGroupPayload toggles the group containing a point during scoring.
type MarkDeadGroupPayload struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// ChatPayload is an in-game chat line.
type ChatPayload struct {
	Text string `json:"text"`
}

// ErrorPayload is a protocol-level error.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
