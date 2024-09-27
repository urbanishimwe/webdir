package node

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

func updateTimeNow(code Code, by, content string) UpdateTime {
	return UpdateTime{
		Code:    code,
		At:      time.Now(),
		By:      by,
		Content: content,
	}
}

func messageBodyFormat(code Code, status ResponseStatus, content string) *MessageBody {
	return &MessageBody{
		Code:    code,
		Status:  status,
		Content: content,
	}
}

func responseFormat(nd *NodeConfig, mssg *Message, status ResponseStatus, oauth bool, content string) *Message {
	v := &Message{
		Header: MessageHeader{
			Destination: mssg.Header.Node.Oauth.UserName,
			Node:        nd.Node,
		},
		Body: *messageBodyFormat(CodeResponse, status, content),
	}
	if !oauth {
		v.Header.Node.Oauth = Oauth{}
	}
	return v
}

// generate random text
func randomText() string {
	// for now we don't need this method to fail
	// fallback is used if we can't generate 8 random characters
	const fallback = "12345678"

	rd := make([]byte, 8)
	if _, err := rand.Read(rd); err != nil {
		rd = []byte(fallback)
	}
	return hex.EncodeToString(rd)
}
