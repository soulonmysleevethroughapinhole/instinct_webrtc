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
	"github.com/pion/webrtc/v2/pkg/media"
	"github.com/pion/webrtc/v2/pkg/media/oggwriter"
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

func saveToDisk(i media.Writer, track *webrtc.Track) {
	defer func() {
		if err := i.Close(); err != nil {
			panic(err)
		}
	}()

	for {
		rtpPacket, err := track.ReadRTP()
		if err != nil {
			panic(err)
		}
		if err := i.WriteRTP(rtpPacket); err != nil {
			panic(err)
		}
	}
}

//not quite sure how this works, maybe change and see what happens
var (
	incomingClients = make(chan *agent.Client, 10)
	sdpChan         = make(chan string)
	answerChan      = make(chan []byte)
	numOfPeers      = 0
	isSetToDelete   = false
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

	isMediaFinished bool
	Description     string
	Image           string
	Artist          string
}

func NewWebInterface(router *chi.Mux, path string) *WebInterface {
	w := WebInterface{Clients: make(map[int]*agent.Client),
		ClientsLock:     new(sync.Mutex),
		Channels:        make(map[int]*agent.Channel),
		ChannelsLock:    new(sync.Mutex),
		isMediaFinished: false,
		Description:     "Description not set",
		Image:           "default.png",
		Artist:          "username",
	}

	w.ChannelsLock.Lock()
	defer w.ChannelsLock.Unlock()

	//go w.handleIncomingClients()
	//go w.handleExpireTransmit()

	regPath := "/api/sdp/" + path
	wsPath := "/api/" + path

	log.Println(regPath)
	log.Println(wsPath)

	//router.HandleFunc(path, w.webSocketHandler)

	//getting the sdp is stateless, people can keep the connection even if they are not in the chit chat room
	router.Post(regPath, func(w http.ResponseWriter, r *http.Request) {
		//auth here
		//have counter for number of users
		//allow only publisher to be first

		//if numOfPeers == 0 {
		//auth(token)
		//}

		body, _ := ioutil.ReadAll(r.Body)
		sdpChan <- string(body)

		answer := <-answerChan
		numOfPeers++
		fmt.Fprint(w, string(answer))
	})
	//this is for the chat only, and managing connections
	router.Get(wsPath, w.webSocketHandler)

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

		DecodeBase64(<-sdpChan, &offer)

		peerConnection, err := api.NewPeerConnection(peerConnectionConfig)
		if err != nil {
			panic(err)
		}

		if _, err := peerConnection.AddTransceiver(webrtc.RTPCodecTypeAudio); err != nil {
			panic(err)
		}

		oggFile, err := oggwriter.New("output.ogg", 48000, 2)
		if err != nil {
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

			//check how many times this bad boy is called

			localTrack, newTrackErr := peerConnection.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "audio", "pion")
			if newTrackErr != nil {
				panic(newTrackErr)
			}
			localTrackChan <- localTrack

			/* duct tape and bandaids solution */
			//if peerconnnum == 0 { you know the drill, else skip this stuff
			//I think I'll need to go routine here, so yeh
			/* TESTING SAVING */

			if numOfPeers == 0 {
				codec := remoteTrack.Codec()
				if codec.Name == webrtc.Opus {
					fmt.Println("Got Opus track, saving to disk as output.opus (48 kHz, 2 channels)")
					saveToDisk(oggFile, remoteTrack)
				} else {
					log.Println("Wrong codec, not recording")
				}
			}

			/* This should only be seen for the sendonly connection */

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
			// alright
			// I think I will close the oggfile recording by checking bool here
			//with every channel

			if isSetToDelete == true {
				closeErr := oggFile.Close()
				if closeErr != nil {
					panic(closeErr)
				}
				fmt.Println("Done writing media files")
				w.isMediaFinished = true
				//os.Exit(0)
			}

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
		//no I don't think so
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
		case agent.MessagePing:
			c.Out <- &agent.Message{T: agent.MessagePong, M: msg.M}
		case agent.MessageChat:
			log.Printf("<%s> %s", c.Name, msg.M)
			//Find a way to authenticate nicknames
			//I think either
			//A: auth at socket creation
			//B: with every request
			// not sure if the latter is necessary
			// or if the first one is enough
			// only one way to tell, ask god

			w.ClientsLock.Lock()
			for _, wc := range w.Clients {
				wc.Out <- &agent.Message{S: c.ID, C: msg.C, N: c.Name, T: agent.MessageChat, M: msg.M}
			}
			w.ClientsLock.Unlock()
		case agent.MessageConnect, agent.MessageDisconnect:
			w.ClientsLock.Lock()

			if msg.T == agent.MessageDisconnect {
				w.quitChannel(c)

				c.Close()
			}

			msg.N = c.Name

			for _, wc := range w.Clients {
				wc.Out <- msg
			}

			w.ClientsLock.Unlock()

			w.updateUserList()
		default:
			log.Printf("Unhandled message %d %s", msg.T, msg.M)
		}
	}
}

func (w *WebInterface) createChannels() {
	w.ChannelsLock.Lock()
	defer w.ChannelsLock.Unlock()

	// TODO Load channels from database
}

func (w *WebInterface) AddChannel(name string, topic string) {
	w.ChannelsLock.Lock()
	defer w.ChannelsLock.Unlock()

	//ch := agent.NewChannel(w.nextChannelID(), t)
	ch := agent.NewChannel(w.nextChannelID())
	ch.Name = name
	ch.Topic = topic

	w.Channels[ch.ID] = ch
}

func (w *WebInterface) nextChannelID() int {
	id := 0
	for cid := range w.Channels {
		if cid > id {
			id = cid
		}
	}

	return id + 1
}

/*yeah no idea anymore on what is happening*/
//func (w *WebInterface) MessageRequestSDP(offerSDP []byte) ([]byte, error) {

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

func (w *WebInterface) updateUserList() {
	w.ClientsLock.Lock()

	msg := &agent.Message{T: agent.MessageUsers}

	var userList agent.UserList
	for _, wc := range w.Clients {
		c := 0
		if wc.Channel != nil {
			c = wc.Channel.ID
		}

		userList = append(userList, &agent.User{ID: wc.ID, N: wc.Name, C: c})
	}

	sort.Sort(userList)

	var err error
	msg.M, err = json.Marshal(userList)
	if err != nil {
		log.Fatal("failed to marshal user list: ", err)
	}

	for _, wc := range w.Clients {
		wc.Out <- msg
	}

	w.ClientsLock.Unlock()
}

func (w *WebInterface) quitChannel(c *agent.Client) {
	if c.Channel == nil {
		return
	}

	ch := c.Channel

	w.ClientsLock.Lock()
	ch.Lock()

	for _, wc := range ch.Clients {
		if len(wc.AudioOut.Tracks) == 0 && wc.ID != c.ID {
			continue
		}

		wc.Out <- &agent.Message{T: agent.MessageQuit, N: c.Name, C: ch.ID}
	}

	delete(ch.Clients, c.ID)
	c.Channel = nil

	ch.Unlock()
	w.ClientsLock.Unlock()

	w.updateUserList()
}

func (w *WebInterface) webSocketHandler(wr http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(wr, r, nil)
	if err != nil {
		return
	}

	c := agent.NewClient(conn)
	incomingClients <- c

	<-c.Terminated

	w.quitChannel(c)

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
