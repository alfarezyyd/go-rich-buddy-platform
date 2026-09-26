package websocket

import (
	"sync"
)

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	syncMutex  sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client, 10),
		unregister: make(chan *Client, 10),
		broadcast:  make(chan []byte, 100),
	}
}
func (websocketHub *Hub) Run() {
	for {
		select {
		case client := <-websocketHub.register:
			websocketHub.clients[client] = true

		case client := <-websocketHub.unregister:
			if _, ok := websocketHub.clients[client]; ok {
				delete(websocketHub.clients, client)
				close(client.send)
			}

		case message := <-websocketHub.broadcast:
			for client := range websocketHub.clients {
				select {
				case client.send <- message:
				default:
					delete(websocketHub.clients, client)
					close(client.send)
				}
			}
		}
	}
}

func (websocketHub *Hub) Broadcast(message []byte) {
	websocketHub.broadcast <- message
}
