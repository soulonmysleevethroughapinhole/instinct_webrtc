package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v2"
)

const (
	rtcpPLIInterval = time.Second * 3
	compress        = false
)

type SdpStruct struct {
	User string `json:"user"`
	SDP  string `json:"sdp"`
}

func Save(answer string) {
	d1 := []byte(answer)

	err := ioutil.WriteFile("answerfile", d1, 0700)
	if err != nil {
		panic(err)
	}
}

func HTTPSDPServer() chan string {
	port := flag.Int("port", 8004, "hypertexto transfero 🅱️rotocol server 🅱️ort")
	flag.Parse()
	sdpChan := make(chan string)
	//userChan := make(chan string)
	http.HandleFunc("/api/sdp", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {

			//body, _ := ioutil.ReadAll(r.Body)
			requestBody, err := ioutil.ReadAll(r.Body)
			if err != nil {
				log.Fatal("fuck")
			}
			defer r.Body.Close()

			var itemRequest SdpStruct
			if err := json.Unmarshal(requestBody, &itemRequest); err != nil {
				log.Fatal("foock")
			}

			//userChan = itemRequest.User

			sdpChan <- string(itemRequest.SDP)

			fmt.Fprint(w, "OK")
		}
	})
	http.HandleFunc("/api/answer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			answer, err := ioutil.ReadFile("answerfile")
			if err != nil {
				log.Fatal("fuck")
			}
			fmt.Fprint(w, string(answer))

			os.Remove("answerfile")
		}
	})

	go func() {
		log.Println("LISTENING & SERVING ON PORT::" + strconv.Itoa(*port))
		err := http.ListenAndServe(":"+strconv.Itoa(*port), nil)
		if err != nil {
			panic(err)
		}
	}()

	return sdpChan //, userChan
}

// func ReallyFuckedUpShit(nigga string) {
// 	http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		json.NewEncoder(w).Encode(nigga)
// 	}
// }

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

func main() {
	// r := chi.NewRouter()

	// sdpChan := make(chan string)
	// userChan := make(chan string)

	// r.Post("/api/sdp", func(w http.ResponseWriter, r *http.Request) {
	// 	log.Println("foo")
	// 	requestBody, err := ioutil.ReadAll(r.Body)
	// 	if err != nil {
	// 		log.Fatal("fuck")
	// 	}
	// 	defer r.Body.Close()

	// 	var itemRequest SdpStruct
	// 	if err := json.Unmarshal(requestBody, &itemRequest); err != nil {
	// 		log.Fatal("fuck")
	// 	}

	// 	userChan <- string(itemRequest.User)
	// 	sdpChan <- string(itemRequest.SDP)

	// 	log.Println("this won't be seen :(")

	// })

	// go func() {
	// 	log.Println("LISTENING & SERVING ON PORT::8006")
	// 	if err := http.ListenAndServe(":8006", r); err != nil {
	// 		panic(err)
	// 	}
	// }()

	sdpChan := HTTPSDPServer()

	offer := webrtc.SessionDescription{}

	DecodeBase64(<-sdpChan, &offer)
	fmt.Println("line")

	mediaEngine := webrtc.MediaEngine{}
	err := mediaEngine.PopulateFromSDP(offer)
	if err != nil {
		panic(err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine((mediaEngine)))

	peerConnectionConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	peerConnection, err := api.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		panic(err)
	}

	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
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

	//fmt.Println(EncodeBase64(answer))

	//Broadcast sender

	//encodedans := EncodeBase64(answer)

	Save(EncodeBase64(answer))

	localTrack := <-localTrackChan
	for {
		fmt.Println("line")
		fmt.Println("Curl an base64 SDP to start sendonly peer connection")

		recvOnlyOffer := webrtc.SessionDescription{}
		DecodeBase64(<-sdpChan, &recvOnlyOffer)

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

		Save(EncodeBase64(answer))
	}
}
