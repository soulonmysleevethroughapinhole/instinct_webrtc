package web

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/go-chi/chi"
	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v2"
	"github.com/soulonmysleevethroughapinhole/instinct_webrtc/agent"
)

//
// Testing
//

const (
	rtcpPLIInterval = time.Second * 3
	compress        = false
)

var peerConnectionConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	},
}

//not quite sure how this works, maybe change and see what happens
var (
	incomingClients = make(chan *agent.Client, 10)
	sdpChan         = make(chan string)
	answerChan      = make(chan []byte)
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	EnableCompression: true,
}

type WebInterface struct {
	Clients     map[int]*agent.Client
	ClientsLock *sync.Mutex

	Channels     map[int]*agent.Channel
	ChannelsLock *sync.Mutex
}

func NewWebInterface(router *chi.Mux, path string) *WebInterface {
	w := WebInterface{Clients: make(map[int]*agent.Client), ClientsLock: new(sync.Mutex), Channels: make(map[int]*agent.Channel), ChannelsLock: new(sync.Mutex)}

	w.ChannelsLock.Lock()
	defer w.ChannelsLock.Unlock()

	//go w.handleIncomingClients()
	//go w.handleExpireTransmit()

	regPath := "/api/sdp/" + path

	log.Println(regPath)

	//router.HandleFunc(path, w.webSocketHandler)
	router.Post(regPath, func(w http.ResponseWriter, r *http.Request) {
		log.Println("ENDPOINT HIT")

		body, _ := ioutil.ReadAll(r.Body)
		sdpChan <- string(body)

		answer := <-answerChan
		fmt.Fprint(w, string(answer))

	})

	log.Println(path)

	go func() {
		/* cracks knuckles */
		/* nothin personnel kid */

		m := webrtc.MediaEngine{}

		m.RegisterCodec(webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000))
		api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

		offer := webrtc.SessionDescription{}
		//DecodeBase64(<-sdpChan, &offer) // I think this just puts the chan into the offer
		//Decode?
		//going to do something retarded and skip the above

		log.Println("new init")

		DecodeBase64(<-sdpChan, &offer)

		log.Println("blocks right there")

		peerConnection, err := api.NewPeerConnection(peerConnectionConfig)
		if err != nil {
			panic(err)
		}

		if _, err := peerConnection.AddTransceiver(webrtc.RTPCodecTypeAudio); err != nil {
			panic(err)
		}

		localTrackChan := make(chan *webrtc.Track)
		peerConnection.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
			go func() {
				ticker := time.NewTicker(rtcpPLIInterval)
				for range ticker.C {
					if rtcpSendErr := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: remoteTrack.SSRC()}}); rtcpSendErr != nil {
						fmt.Println(rtcpSendErr)
					}
				}
			}()

			localTrack, newTrackErr := peerConnection.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "audio", "pion")
			if newTrackErr != nil {
				panic(newTrackErr)
			}
			localTrackChan <- localTrack

			rtpBuf := make([]byte, 1400)
			for {
				i, readErr := remoteTrack.Read(rtpBuf)
				if readErr != nil {
					panic(readErr)
				}

				// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
				if _, err = localTrack.Write(rtpBuf[:i]); err != nil && err != io.ErrClosedPipe {
					panic(err)
				}
			}
		})

		err = peerConnection.SetRemoteDescription(offer)
		if err != nil {
			panic(err)
		}

		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			panic(err)
		}

		err = peerConnection.SetLocalDescription(answer)
		if err != nil {
			panic(err)
		}

		//alright
		//so the plan is to send the response to a channel which allows it to
		//to be sent through websockets

		//EncodeBase64(answer) ?

		answerChan <- []byte(EncodeBase64(answer))

		//answerchan should receive the new the answer
		//until then answerchan should block

		localTrack := <-localTrackChan
		for {

			recvOnlyOffer := webrtc.SessionDescription{}
			DecodeBase64(<-sdpChan, &recvOnlyOffer)

			log.Println("big think")

			peerConnection, err := api.NewPeerConnection(peerConnectionConfig)
			if err != nil {
				panic(err)
			}

			_, err = peerConnection.AddTrack(localTrack)
			if err != nil {
				panic(err)
			}

			err = peerConnection.SetRemoteDescription(recvOnlyOffer)
			if err != nil {
				panic(err)
			}

			answer, err := peerConnection.CreateAnswer(nil)
			if err != nil {
				panic(err)
			}

			err = peerConnection.SetLocalDescription(answer)
			if err != nil {
				panic(err)
			}

			log.Println("This should be seen after first only")
			answerChan <- []byte(EncodeBase64(answer))
		}
	}()

	return &w
}

func (w *WebInterface) handleIncomingClients() {
	for c := range incomingClients {
		c := c

		w.ClientsLock.Lock()
		id := w.nextClientID()
		c.ID = id
		w.Clients[id] = c

		go func(c *agent.Client) {
			time.Sleep(500 * time.Millisecond)

			c.Connected = true

			//w.updateUserList()

			w.ClientsLock.Lock()
			for _, wc := range w.Clients {
				wc.Out <- &agent.Message{T: agent.MessageConnect, N: c.Name, M: []byte(c.Name)}
			}
			w.ClientsLock.Unlock()
		}(c)

		w.ClientsLock.Unlock()

		//yeah I most likely need this
		w.sendChannelList(c)

		go w.handleRead(c)
	}
}

func (w *WebInterface) handleRead(c *agent.Client) {
	for msg := range c.In {
		if msg == nil {
			return
		}

		log.Printf("%d -> %s %d", msg.S, msg.T, len(msg.M))

		switch msg.T {
		case 101:
			c.Out <- &agent.Message{T: agent.MessagePong, M: msg.M}
		case 201:
			//answer, err := w.MessageRequestSDP(c, msg.PC, msg.M)
			// answer, err := w.MessageRequestSDP(msg.M)
			// if err != nil {
			// 	log.Println("Failed to answer call")
			// 	log.Fatal(err)
			// }

			//sdpChan <- msg.M
			/*sdpChan <- string(msg.M)

			answer := <-answerChan

			c.Out <- &agent.Message{T: agent.MessageAnswer, PC: msg.PC, M: answer}*/
		}
		//case 202:
		//MessageRequestSDP
	}
}

/*yeah no idea anymore on what is happening*/
//func (w *WebInterface) MessageRequestSDP(offerSDP []byte) ([]byte, error) {
func (w *WebInterface) MessageRequestSDP() {
	// /* cracks knuckles */
	// /* nothin personnel kid */

	// m := webrtc.MediaEngine{}

	// m.RegisterCodec(webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000))
	// api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	// offer := webrtc.SessionDescription{}
	// //DecodeBase64(<-sdpChan, &offer) // I think this just puts the chan into the offer
	// //Decode?
	// //going to do something retarded and skip the above

	// err := json.Unmarhal(<-sdpChan, &offer)
	// if err != nil {
	// 	panic(err)
	// }

	// peerConnection, err := api.NewPeerConnection(peerConnectionConfig)
	// if err != nil {
	// 	panic(err)
	// }

	// if _, err := peerConnection.AddTransceiver(webrtc.RTPCodecTypeAudio); err != nil {
	// 	panic(err)
	// }

	// localTrackChan := make(chan *webrtc.Track)
	// peerConnection.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
	// 	go func() {
	// 		ticker := time.NewTicker(rtcpPLIInterval)
	// 		for range ticker.C {
	// 			if rtcpSendErr := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: remoteTrack.SSRC()}}); rtcpSendErr != nil {
	// 				fmt.Println(rtcpSendErr)
	// 			}
	// 		}
	// 	}()

	// 	localTrack, newTrackErr := peerConnection.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "audio", "pion")
	// 	if newTrackErr != nil {
	// 		panic(newTrackErr)
	// 	}
	// 	localTrackChan <- localTrack

	// 	rtpBuf := make([]byte, 1400)
	// 	for {
	// 		i, readErr := remoteTrack.Read(rtpBuf)
	// 		if readErr != nil {
	// 			panic(readErr)
	// 		}

	// 		// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
	// 		if _, err = localTrack.Write(rtpBuf[:i]); err != nil && err != io.ErrClosedPipe {
	// 			panic(err)
	// 		}
	// 	}
	// })

	// err = peerConnection.SetRemoteDescription(offer)
	// if err != nil {
	// 	panic(err)
	// }

	// answer, err := peerConnection.CreateAnswer(nil)
	// if err != {
	// 	panic(err)
	// }

	// err = peerConnection.SetLocalDescription(answer)
	// if err != nil {
	// 	panic(err)
	// }

	// //alright
	// //so the plan is to send the response to a channel which allows it to
	// //to be sent through websockets

	// //EncodeBase64(answer) ?

	// answerChan <-answer

	// //answerchan should receive the new the answer
	// //until then answerchan should block

	// localTrack := <- localTrackChan
	// for {

	// }
}

// func (w *WebInterface) MessageRequestSDPSubscriber(offerSPD []byte) ([]byte, error) {
// 	//nobody knows what localtrackchan does or what it is,
// 	//you are going to have to take a chance
// 	//there may be scoping issues in here
// }

func (w *WebInterface) nextClientID() int {
	id := 1
	for {
		if _, ok := w.Clients[id]; !ok {
			break
		}

		id++
	}
	return id
}

func (w *WebInterface) sendChannelList(c *agent.Client) {
	var channelList agent.ChannelList

	for _, ch := range w.Channels {
		channelList = append(channelList, &agent.ChannelListing{ID: ch.ID, Type: ch.Type, Name: ch.Name, Topic: ch.Topic})
	}

	//probably useless
	sort.Sort(channelList)

	msg := agent.Message{T: agent.MessageChannels}

	var err error
	msg.M, err = json.Marshal(channelList)
	if err != nil {
		log.Fatal("failed to marshal ch list : ", err)
	}

	c.Out <- &msg
}

func (w *WebInterface) webSocketHandler(wr http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(wr, r, nil)
	if err != nil {
		return
	}

	c := agent.NewClient(conn)
	incomingClients <- c

	<-c.Terminated

	//w.quitChannel(c)

	w.ClientsLock.Lock()
	for id := range w.Clients {
		if w.Clients[id].Status != -1 {
			continue
		}

		name := w.Clients[id].Name
		delete(w.Clients, id)

		for _, wc := range w.Clients {
			wc.Out <- &agent.Message{T: agent.MessageDisconnect, N: name, M: []byte(name)}
		}
	}
	w.ClientsLock.Unlock()
}

func DecodeBase64(in string, obj interface{}) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}
	if compress {
		b = unzip(b)
	}
	err = json.Unmarshal(b, obj)
	if err != nil {
		panic(err)
	}
}

func unzip(in []byte) []byte {
	var b bytes.Buffer
	_, err := b.Write(in)
	if err != nil {
		panic(err)
	}
	r, err := gzip.NewReader(&b)
	if err != nil {
		panic(err)
	}
	res, err := ioutil.ReadAll(r)
	if err != nil {
		panic(err)
	}
	return res
}

func EncodeBase64(obj interface{}) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}
