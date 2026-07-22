package services

import "testing"

type recordingRealtimeClient struct {
	messages []WebSocketMsg
}

func (c *recordingRealtimeClient) Send(msg WebSocketMsg) error {
	c.messages = append(c.messages, msg)
	return nil
}

func newTestWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		playlists: make(map[int]*PlaylistState),
		sockets:   make(map[string]RealtimeClient),
	}
}

func TestWebSocketManagerAddSendBroadcastRemove(t *testing.T) {
	manager := newTestWebSocketManager()
	client := &recordingRealtimeClient{}
	otherClient := &recordingRealtimeClient{}

	id := manager.AddClient(client, 1, "user")
	otherId := manager.AddClient(otherClient, 1, "other-user")

	if len(client.messages) != 1 {
		t.Fatalf("expected handshake message, got %d messages", len(client.messages))
	}
	if client.messages[0].Type != Handshake || client.messages[0].Payload != id {
		t.Fatalf("unexpected handshake message: %#v", client.messages[0])
	}

	manager.SendId(id, WebSocketMsg{Type: Event, Payload: string(PlaylistChanged)})
	if len(client.messages) != 2 {
		t.Fatalf("expected direct send after handshake, got %d messages", len(client.messages))
	}
	if client.messages[1].Type != Event || client.messages[1].Payload != string(PlaylistChanged) {
		t.Fatalf("unexpected direct message: %#v", client.messages[1])
	}

	manager.BroadcastPlaylist(1, WebSocketMsg{Type: Event, Payload: string(ManagersChanged)})
	if len(client.messages) != 3 {
		t.Fatalf("expected broadcast for first client, got %d messages", len(client.messages))
	}
	if len(otherClient.messages) != 2 {
		t.Fatalf("expected handshake and broadcast for second client, got %d messages", len(otherClient.messages))
	}

	manager.Remove(1, "user", id)
	if _, ok := manager.sockets[id]; ok {
		t.Fatal("expected removed client to be deleted from socket map")
	}
	if _, ok := manager.playlists[1]; !ok {
		t.Fatal("expected playlist to remain while another client is connected")
	}

	manager.SendId(id, WebSocketMsg{Type: Event, Payload: "removed-client"})
	if len(client.messages) != 3 {
		t.Fatalf("expected no send after removal, got %d messages", len(client.messages))
	}

	manager.Remove(1, "other-user", otherId)
	if _, ok := manager.playlists[1]; ok {
		t.Fatal("expected playlist state to be removed after last client disconnects")
	}
}
