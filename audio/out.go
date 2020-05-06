package audio

import (
	"log"
	"sync"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/pkg/media"
)

type Out struct {
	Out    []chan *rtp.Packet
	Tracks []*webrtc.Track
	Client []int
	Active []time.Time
	*sync.Mutex
}

func NewOut() *Out {
	out := Out{
		Mutex: new(sync.Mutex),
	}

	return &out
}

func (o *Out) AddTrack(track *webrtc.Track) {
	o.Lock()
	defer o.Unlock()

	o.Tracks = append(o.Tracks, track)
	o.Out = append(o.Out, make(chan *rtp.Packet, 10))
	o.Client = append(o.Client, 0)
	o.Active = append(o.Active, time.Time{})

	go o.handleTrack(len(o.Tracks) - 1)
}

func (o *Out) handleTrack(i int) {
	track := o.Tracks[i]
	var err error
	for p := range o.Out[i] {
		if p == nil {
			return
		}

		err = track.WriteSample(media.Sample{Data: p.Payload, Samples: Samples})
		if err != nil {
			panic(err)
		}
	}
}

func (o *Out) Write(p *rtp.Packet, source int) {
	o.Lock()
	defer o.Unlock()

	for i := range o.Out {
		if o.Client[i] == 0 || o.Client[i] == source || time.Since(o.Active[i]) >= 50*time.Millisecond {
			select {
			case o.Out[i] <- p:
			default:
				log.Printf("warning: filled voice out buffer when writing from %d", source)
				continue
			}

			o.Active[i] = time.Now()
			o.Client[i] = source
			return
		}
	}
}

func (o *Out) Reset() {
	o.Lock()
	defer o.Unlock()

	for i := range o.Out {
		o.Out[i] <- nil
	}

	o.Tracks = nil
	o.Out = nil
	o.Client = nil
	o.Active = nil
}
