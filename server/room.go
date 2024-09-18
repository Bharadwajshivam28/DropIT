package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type Room struct {
	clients   map[*websocket.Conn]bool
	files     []string
	fileData  map[string][]byte
	broadcast chan []byte
	mux       sync.Mutex
}

var rooms = make(map[string]*Room)
var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func generateRoomKey() (string, error) {
	b := make([]byte, 6)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%X", b), nil
}

func (r *Room) handleMessages() {
	for msg := range r.broadcast {
		r.mux.Lock()
		for client := range r.clients {
			err := client.WriteMessage(websocket.BinaryMessage, msg)
			if err != nil {
				log.Printf("Error: %v", err)
				client.Close()
				delete(r.clients, client)
			}
		}
		r.mux.Unlock()
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	roomKey := r.URL.Query().Get("key")
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}

	var room *Room
	if roomKey == "" {
		roomKey, err = generateRoomKey()
		if err != nil {
			http.Error(w, "Failed to generate room key", http.StatusInternalServerError)
			return
		}
		room = &Room{
			clients:   make(map[*websocket.Conn]bool),
			files:     make([]string, 0),
			fileData:  make(map[string][]byte),
			broadcast: make(chan []byte),
		}
		rooms[roomKey] = room
		go room.handleMessages()
		ws.WriteMessage(websocket.TextMessage, []byte(roomKey))
	} else {
		room = rooms[roomKey]
		if room == nil {
			ws.Close()
			return
		}
		ws.WriteMessage(websocket.TextMessage, []byte("FILES:"+strings.Join(room.files, ",")))
	}

	room.clients[ws] = true

	defer func() {
		room.mux.Lock()
		delete(room.clients, ws)
		room.mux.Unlock()
		ws.Close()
	}()

	var currentFile string
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			return
		}

		message := string(msg)
		if strings.HasPrefix(message, "FILE:") {
			currentFile = strings.TrimPrefix(message, "FILE:")
			room.files = append(room.files, currentFile)
			room.fileData[currentFile] = make([]byte, 0)
		} else if message == "EOF" {
			currentFile = ""
		} else if message == "CLOSE" {
			return
		} else if strings.HasPrefix(message, "GET:") {
			fileNum, _ := strconv.Atoi(strings.TrimPrefix(message, "GET:"))
			if fileNum > 0 && fileNum <= len(room.files) {
				fileName := room.files[fileNum-1]
				ws.WriteMessage(websocket.TextMessage, []byte(fileName))
				ws.WriteMessage(websocket.BinaryMessage, room.fileData[fileName])
				ws.WriteMessage(websocket.TextMessage, []byte("EOF"))
			}
		} else if currentFile != "" {
			room.fileData[currentFile] = append(room.fileData[currentFile], msg...)
		}
	}
}

func main() {
	http.HandleFunc("/ws", handleConnections)
	log.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
