package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"sync"

	"github.com/go-chi/chi"
	"github.com/go-chi/cors"

	"github.com/soulonmysleevethroughapinhole/instinct_webrtc/web"
)

type InstanceWrapper struct {
	Servers     map[string]*web.WebInterface //changed int to string, key is
	ServersLock *sync.Mutex
}

type ServerConfig struct {
	Webpath  string `json:"webpath"`
	Channels []ServerChConfig
}

type ServerChConfig struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Topic string `json:"topic"`
}

type ServerInformation struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Image       string `json:"image"`
	NumOfPeers  int    `json:peers`
}

func main() {
	port := ":8005"
	router := chi.NewRouter()

	router.Use(cors.Handler(cors.Options{
		// AllowedOrigins: []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowedOrigins: []string{"*"},
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	iw := InstanceWrapper{Servers: make(map[string]*web.WebInterface), ServersLock: new(sync.Mutex)}

	iw.ServersLock.Lock()
	iw.ServersLock.Unlock()

	//returns all livesets
	router.Get("/api/livesets", func(w http.ResponseWriter, r *http.Request) {
		keys := reflect.ValueOf(iw.Servers).MapKeys()
		res := make([]ServerInformation, len(keys))

		//strkeys := make([]string, len(keys))
		for i := 0; i < len(keys); i++ {
			res[i].Name = keys[i].String()
			res[i].Description = iw.Servers[keys[i].String()].Description
			//res[i].Image = iw.Servers[keys[i].String()].Image
		}

		RespondJson(w, http.StatusOK, res)
	})
	//returns one liveset
	router.Get("/api/livesets/{username}", func(w http.ResponseWriter, r *http.Request) {
		serverName := chi.URLParam(r, "username")

		var res ServerInformation

		if _, ok := iw.Servers[serverName]; ok {
			res.Name = serverName
			res.Description = iw.Servers[serverName].Description
			res.NumOfPeers = iw.Servers[serverName].NumOfPeers
			//res.Image = iw.Servers[serverName].Image
		} else {
			w.WriteHeader(404)
			return
		}

		RespondJson(w, http.StatusOK, res)
	})
	//creates a liveset
	router.Post("/api/livesets/{username}", func(w http.ResponseWriter, r *http.Request) {
		newServerName := chi.URLParam(r, "username")
		body, _ := ioutil.ReadAll(r.Body)
		newServerDescription := string(body)

		var res ServerInformation

		// requestBody, err := ioutil.ReadAll(r.Body)
		// if err != nil {
		// 	w.WriteHeader(400)
		// 	return
		// }
		// defer r.Body.Close()

		// var serverConfigRequest ServerConfig
		// if err := json.Unmarshal(requestBody, &serverConfigRequest); err != nil {
		// 	w.WriteHeader(400)
		// 	return
		// }

		/* serverConfigRequest := ServerConfig{
			Webpath: newServerName,
			Channels: []ServerChConfig{
				ServerChConfig{
					Name:  "name of audio channel",
					Type:  "voice", //make voice & text auto generate
					Topic: "audio channel"},
				ServerChConfig{
					Name:  "name of text channel",
					Type:  "text", //make voice & text auto generate,
					Topic: "text channel"},
			},
		} */

		okChan := make(chan bool)

		if _, ok := iw.Servers[newServerName]; !ok {
			go func() {
				w := web.NewWebInterface(router, newServerName) // maybe configuration as an argument
				iw.ServersLock.Lock()
				defer iw.ServersLock.Unlock()

				w.AddChannel(newServerName, newServerDescription)
				w.Description = newServerDescription

				iw.Servers[newServerName] = w //TODO: ability to remove from map

				// t := agent.ChannelVoice
				// w.AddChannel(t, "audio channel", "Voice-1")
				// t = agent.ChannelText

				log.Printf("Server of the name %s is running . . .\n", newServerName)

				res.Name = newServerName
				res.Description = iw.Servers[newServerName].Description

				okChan <- true

				_ = w

				// I don't know, maybe select is needed try to figure it out
				//select {}

				// for ch := range serverConfigRequest.Channels {
				// 	t := agent.ChannelText
				// 	if ch.Type != "" && strings.ToLower(ch.Type)[0] == 'v' {
				// 		t = agent.ChannelVoice
				// 	}

				// 	w.AddChannel(t, ch.Name, ch.Topic)
				// }
			}()
		} else {
			log.Printf("Server of the name %s already exists.", newServerName)
			w.WriteHeader(409)
			return
		}
		<-okChan
		RespondJson(w, http.StatusCreated, res)
	})
	router.Delete("/api/livesets/{username}", func(w http.ResponseWriter, r *http.Request) {
		serverName := chi.URLParam(r, "username")
		//auth here

		//I will set a boolean value in iw[serverName]
		//then fiddle with the chans a bit
		//making sure recording has ended
		//then delete

		iw.Servers[serverName].SetToDelete = true

		if iw.Servers[serverName].MediaWriting == false {
			delete(iw.Servers, serverName)
		}

		w.WriteHeader(http.StatusOK)
	})

	log.Println("LISTENING AND SERVING", port)
	log.Fatal(http.ListenAndServe(port, router))

}

func RespondJson(w http.ResponseWriter, statusCode int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(body)
}
