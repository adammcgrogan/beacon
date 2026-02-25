package handlers

import (
	"context"
	"encoding/json"

	"github.com/gorilla/websocket"
)

type playerPermissionsResponse struct {
	RequestID   string   `json:"request_id"`
	PlayerUUID  string   `json:"player_uuid"`
	Online      bool     `json:"online"`
	Permissions []string `json:"permissions"`
}

func (m *WebSocketManager) RequestPlayerPermissions(ctx context.Context, playerUUID string) ([]string, bool, error) {
	if !m.isMinecraftConnected() {
		return nil, false, ErrPluginOffline
	}

	requestID, err := randomHex(16)
	if err != nil {
		return nil, false, err
	}

	responseChan := make(chan playerPermissionsResponse, 1)
	m.permReqLock.Lock()
	if m.pendingPermReqByID == nil {
		m.pendingPermReqByID = make(map[string]chan playerPermissionsResponse)
	}
	m.pendingPermReqByID[requestID] = responseChan
	m.permReqLock.Unlock()

	defer func() {
		m.permReqLock.Lock()
		delete(m.pendingPermReqByID, requestID)
		m.permReqLock.Unlock()
	}()

	message, err := json.Marshal(map[string]any{
		"event": "player_permissions_request",
		"payload": map[string]string{
			"request_id":  requestID,
			"player_uuid": playerUUID,
		},
	})
	if err != nil {
		return nil, false, err
	}

	m.mcConnLock.RLock()
	mcConn := m.mcConn
	m.mcConnLock.RUnlock()
	if mcConn == nil {
		return nil, false, ErrPluginOffline
	}
	m.mcWriteLock.Lock()
	err = mcConn.WriteMessage(websocket.TextMessage, message)
	m.mcWriteLock.Unlock()
	if err != nil {
		return nil, false, ErrPluginOffline
	}

	select {
	case response := <-responseChan:
		return response.Permissions, response.Online, nil
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
}

func (m *WebSocketManager) resolvePendingPermissionsRequest(response playerPermissionsResponse) {
	m.permReqLock.Lock()
	defer m.permReqLock.Unlock()

	ch, ok := m.pendingPermReqByID[response.RequestID]
	if !ok {
		return
	}

	select {
	case ch <- response:
	default:
	}
}

func (m *WebSocketManager) failAllPendingPermissionRequests() {
	m.permReqLock.Lock()
	defer m.permReqLock.Unlock()
	for id, ch := range m.pendingPermReqByID {
		select {
		case ch <- playerPermissionsResponse{RequestID: id, Online: false, Permissions: nil}:
		default:
		}
	}
}
