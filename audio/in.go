package audio

import (
	"log"
	"sync"
	"time"
)

const (
	TransmitRepeat = 5 * time.Second
	TransmitExpire = 100 * time.Millisecond
)

type In struct {
	In           chan []int16
	Active       time.Time
	Notify       time.Time
	Transmitting bool
	*sync.Mutex
}

func NewIn() *In {
	in := In{
		In:    make(chan []int16, 10),
		Mutex: new(sync.Mutex),
	}

	return &in
}

func (in *In) StartTransmit() bool {
	in.Lock()
	defer in.Unlock()

	in.Active = time.Now()
	if in.Transmitting && time.Since(in.Notify) < TransmitRepeat {
		log.Println("shit's bad")
		return false
	}

	in.Transmitting = true
	in.Notify = time.Now()
	return true
}

func (in *In) ExpireTransmit() bool {
	in.Lock()
	defer in.Unlock()

	if !in.Transmitting || time.Since(in.Active) < TransmitExpire {
		return false
	}

	in.Transmitting = false
	in.Active = time.Time{}
	in.Notify = time.Time{}
	return true

}
