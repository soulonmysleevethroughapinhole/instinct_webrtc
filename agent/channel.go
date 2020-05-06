package agent

import (
	"strings"
	"sync"
)

type ChannelType int

const (
	ChannelUnknown ChannelType = 0
	ChannelText    ChannelType = 1
	ChannelVoice   ChannelType = 2
)

type Channel struct {
	ID      int
	Type    ChannelType
	Name    string
	Topic   string
	Clients map[int]*Client

	*sync.Mutex
}

func NewChannel(id int, t ChannelType) *Channel {
	c := Channel{ID: id, Type: t, Clients: make(map[int]*Client), Mutex: new(sync.Mutex)}

	return &c
}

type ChannelListing struct {
	ID    int
	Type  ChannelType
	Name  string
	Topic string
}

type ChannelList []*ChannelListing

func (c ChannelList) Len() int      { return len(c) }
func (c ChannelList) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c ChannelList) Less(i, j int) bool {
	if c[i] == nil || c[j] == nil {
		return c[j] != nil
	}

	return c[i].Type < c[j].Type || (c[i].Type == c[j].Type && strings.ToLower(c[i].Name) < strings.ToLower(c[j].Name))
}
