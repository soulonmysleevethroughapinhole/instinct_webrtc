package agent

import "fmt"

type MessageType int

const (
	MessageBinary        MessageType = 2
	MessagePing          MessageType = 101
	MessagePong          MessageType = 102
	MessageCall          MessageType = 103
	MessageAnswer        MessageType = 104
	MessageConnect       MessageType = 110
	MessageJoin          MessageType = 111
	MessageQuit          MessageType = 112
	MessageNick          MessageType = 113
	MessageTopic         MessageType = 114
	MessageAction        MessageType = 115
	MessageDisconnect    MessageType = 119
	MessageChat          MessageType = 120
	MessageTypingStart   MessageType = 121
	MessageTypingStop    MessageType = 122
	MessageTransmitStart MessageType = 123
	MessageTransmitStop  MessageType = 124
	MessageServers       MessageType = 130
	MessageChannels      MessageType = 131
	MessageUsers         MessageType = 132
)

func (t MessageType) String() string {
	switch t {
	case MessageBinary:
		return "Binary"
	case MessagePing:
		return "Ping"
	case MessagePong:
		return "Pong"
	case MessageCall:
		return "Call"
	case MessageAnswer:
		return "Answer"
	case MessageConnect:
		return "Connect"
	case MessageJoin:
		return "Join"
	case MessageQuit:
		return "Quit"
	case MessageNick:
		return "Nick"
	case MessageTopic:
		return "Topic"
	case MessageAction:
		return "Action"
	case MessageDisconnect:
		return "Disconnect"
	case MessageChat:
		return "Chat"
	default:
		return fmt.Sprintf("%d?", t)
	}
}

type Message struct {
	S  int         // Source
	N  string      // Source nickname
	PC int         // PeerConn
	C  int         // Channel
	T  MessageType // Type
	M  []byte      // Message
}
