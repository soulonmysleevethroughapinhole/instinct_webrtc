package main

import (
	"log"
	"net/http"
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

	router.Post("/api/liveset/{username}", func(w http.ResponseWriter, r *http.Request) {
		newServerName := chi.URLParam(r, "username")

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

		if _, ok := iw.Servers[newServerName]; !ok {
			go func() {
				w := web.NewWebInterface(router, newServerName) // maybe configuration as an argument
				iw.ServersLock.Lock()
				defer iw.ServersLock.Unlock()

				iw.Servers[newServerName] = w //TODO: ability to remove from map

				// t := agent.ChannelVoice
				// w.AddChannel(t, "audio channel", "Voice-1")
				// t = agent.ChannelText
				// w.AddChannel(t, "text channel", "Text-1")

				log.Printf("Server of the name %s is running . . .\n", newServerName)

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
		w.WriteHeader(201)
	})

	log.Println("LISTENING AND SERVING", port)
	log.Fatal(http.ListenAndServe(port, router))

}
