package agent

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v2"
	"github.com/soulonmysleevethroughapinhole/instinct_webrtc/audio"
)

type Client struct {
	ID        int
	Name      string
	Status    int
	Conn      *websocket.Conn
	Connected bool

	PeerConns    map[int]*webrtc.PeerConnection
	PeerConnLock *sync.Mutex

	In  chan *Message
	Out chan *Message

	AudioIn  *audio.In
	AudioOut *audio.Out

	Channel *Channel

	Terminated chan bool
}

func NewClient(conn *websocket.Conn) *Client {
	c := Client{
		Conn:         conn,
		Name:         "Anonymous",
		PeerConns:    make(map[int]*webrtc.PeerConnection),
		PeerConnLock: new(sync.Mutex),
		In:           make(chan *Message, 10),
		Out:          make(chan *Message, 10),
		AudioIn:      audio.NewIn(),
		AudioOut:     audio.NewOut(),
		Terminated:   make(chan bool)}

	go c.handleRead()
	go c.handleWrite()

	return &c
}

func (c *Client) handleRead() {
	var (
		messageTypeInt int
		messageType    MessageType
		message        []byte
		err            error
	)
	for {
		c.Conn.SetReadDeadline(time.Now().Add(1 * time.Minute))
		messageTypeInt, message, err = c.Conn.ReadMessage()
		if err != nil || c.Status == -1 {
			c.Close()
			return
		}
		messageType = MessageType(messageTypeInt)

		in := Message{}
		if messageType == 2 {
			in.T = messageType
			in.M = message
		} else {
			err = json.Unmarshal(message, &in)
			if err != nil {
				log.Println(string(message))
				log.Println()
				log.Println(err)

				c.Close()
				return
			}
		}

		in.S = c.ID
		c.In <- &in
	}
}

func (c *Client) handleWrite() {
	var (
		out []byte
		err error
	)
	for msg := range c.Out {
		if msg == nil {
			return
		}

		out, err = json.Marshal(msg)
		if err != nil {
			c.Close()
			return
		}

		c.Conn.WriteMessage(1, out)
	}
}

func (c *Client) Close() {
	if c.Status == -1 {
		return
	}
	c.Status = -1

	c.CloseAudio()

	if c.Conn != nil {
		c.Conn.Close()
	}

	c.In <- nil
	c.Out <- nil

	go func() {
		c.Terminated <- true
	}()
}

func (c *Client) CloseAudio() {
	c.ClosePeerConns()

	c.PeerConns = make(map[int]*webrtc.PeerConnection)

	c.AudioOut.Reset()
}

func (c *Client) ClosePeerConns() {
	for id := range c.PeerConns {
		c.ClosePeerConn(id)
	}
}

func (c *Client) ClosePeerConn(id int) {
	pc := c.PeerConns[id]
	if pc == nil {
		return
	}

	pc.Close()
	pc = nil
}
