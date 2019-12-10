package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"math/rand"
	"time"
)

const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))
var responses = make(map[string]*http.ResponseWriter)
var clients = make(map[string]*websocket.Conn)
var broadcast = make(chan Response)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Response struct {
	Token string `json:"token"`
	Res interface{} `json:"res"`
}

type Request struct {

	Token string `json:"token"`
	Query string `json:"query"`
}

func StringWithCharset(length int, charset string) string {
	str := make([]byte, length)
	for i := range str {
		str[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(str)
}

func String(length int) string {
	return StringWithCharset(length, charset)
}

func handleConnections(w http.ResponseWriter, r *http.Request) {

	token := String(4)
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	// ensure connection close when function returns
	defer ws.Close()
	clients[token] = ws

	_ = ws.WriteJSON(&Request{Token: token})

	for {
		var msg Response
		// Read in a new message as JSON and map it to a Message object
		err := ws.ReadJSON(&msg)
		fmt.Println("message; ", msg)
		if err != nil {
			log.Printf("error: %v", err)
			delete(clients, token)
			break
		}
		// send the new message to the broadcast channel
		broadcast <- msg
	}
}

func handleMessages() {
	for {
		// grab next message from the broadcast channel
		msg := <-broadcast

		token := msg.Token
		a := responses[token]
		res := *a
		res.WriteHeader(http.StatusOK)
		fmt.Println("msg", msg)
		_ = json.NewEncoder(res).Encode(msg.Res)
		res.Header().Set("d", "t")
		delete(responses, token)

	}
}

func main() {

	go handleMessages()

	webSocketServer := http.NewServeMux()
	webSocketServer.HandleFunc("/ws", handleConnections)

	httpServer := http.NewServeMux()
	httpServer.HandleFunc("/", Default)

	log.Println("http server started on :9000")
	go func() {
		if err := http.ListenAndServe(":9000", httpServer); err != nil {
			log.Fatal("Error while starting http server: ", err)
		}
	}()

	log.Println("websocket server started on :8000")
	if err := http.ListenAndServe(":8000", webSocketServer); err != nil {
		log.Fatal("Error while starting http server: ", err)
	}

}

func Default(res http.ResponseWriter, req *http.Request) {

	token := req.Header.Get("token")
	query := req.URL.Path + "?"+ req.URL.RawQuery
	fmt.Println("query: ", query, token)

	client := clients[token]

	if client == nil {
		res.WriteHeader(http.StatusNotFound)
		return
	}

	err := client.WriteJSON(&Request{Query:query})
	if err != nil {
		log.Printf("error: %v", err)
		_ = client.Close()
		delete(clients, token)
	}

	responses[token] = &res

	for {
		if res.Header().Get("d") == "t" {
			res.Header().Del("d")
			break
		}
	}

	fmt.Println("res", res.Header().Get("d"))

}