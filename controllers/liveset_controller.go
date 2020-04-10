package controllers

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
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v2"
)

var (
	LivesetController livesetControllerInterface = &livesetController{}
)

type livesetControllerInterface interface {
	SDP(w http.ResponseWriter, r *http.Request)
}

type livesetController struct{}

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

const (
	rtcpPLIInterval = time.Second * 3
	compress        = false
)

func (c *livesetController) SDP(w http.ResponseWriter, r *http.Request) {
	log.Println("fuck")
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	sdpChan := make(chan string)
	sdpChan <- string(body)

	offer := webrtc.SessionDescription{}

	log.Println("FUCK")
	fmt.Println("FUCK")

	DecodeBase64(<-sdpChan, &offer)

	log.Println("It stop right before this")

	fmt.Println("")
	fmt.Println("Decode called")
	fmt.Println("")

	mediaEngine := webrtc.MediaEngine{}
	err = mediaEngine.PopulateFromSDP(offer)
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

	log.Println("4")

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

	log.Println("n")

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

	log.Println("FUCK")
	fmt.Println("FUCK")

	log.Println("------------")
	fmt.Println(EncodeBase64(answer))
	log.Println("------------")

	localTrack := <-localTrackChan
	for {
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

		fmt.Println(EncodeBase64(answer))
	}

}
