package handlers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gorilla/websocket"
)

type permissionAdminResponse struct {
	RequestID   string          `json:"request_id"`
	Action      string          `json:"action"`
	PlayerUUID  string          `json:"player_uuid"`
	OK          bool            `json:"ok"`
	Error       string          `json:"error"`
	Permissions map[string]bool `json:"permissions"`
}

func (m *WebSocketManager) RequestPermissionSnapshot(ctx context.Context, playerUUID, playerName string, permissionNodes []string) (map[string]bool, error) {
	response, err := m.requestPermissionAdmin(ctx, map[string]any{
		"action":           "snapshot",
		"player_uuid":      playerUUID,
		"player_name":      playerName,
		"permission_nodes": permissionNodes,
	})
	if err != nil {
		return nil, err
	}
	if !response.OK {
		if response.Error == "" {
			return nil, errors.New("permission snapshot failed")
		}
		return nil, errors.New(response.Error)
	}
	return response.Permissions, nil
}

func (m *WebSocketManager) RequestPermissionSet(ctx context.Context, playerUUID, playerName, permissionNode string, enabled bool) error {
	response, err := m.requestPermissionAdmin(ctx, map[string]any{
		"action":          "set",
		"player_uuid":     playerUUID,
		"player_name":     playerName,
		"permission_node": permissionNode,
		"enabled":         enabled,
	})
	if err != nil {
		return err
	}
	if !response.OK {
		if response.Error == "" {
			return errors.New("permission update failed")
		}
		return errors.New(response.Error)
	}
	return nil
}

func (m *WebSocketManager) requestPermissionAdmin(ctx context.Context, payload map[string]any) (permissionAdminResponse, error) {
	if !m.isMinecraftConnected() {
		return permissionAdminResponse{}, ErrPluginOffline
	}

	requestID, err := randomHex(16)
	if err != nil {
		return permissionAdminResponse{}, err
	}
	payload["request_id"] = requestID

	responseChan := make(chan permissionAdminResponse, 1)
	m.permissionAdminReqLock.Lock()
	if m.pendingPermissionAdminReqByID == nil {
		m.pendingPermissionAdminReqByID = make(map[string]chan permissionAdminResponse)
	}
	m.pendingPermissionAdminReqByID[requestID] = responseChan
	m.permissionAdminReqLock.Unlock()

	defer func() {
		m.permissionAdminReqLock.Lock()
		delete(m.pendingPermissionAdminReqByID, requestID)
		m.permissionAdminReqLock.Unlock()
	}()

	message, err := json.Marshal(map[string]any{
		"event":   "permission_admin_request",
		"payload": payload,
	})
	if err != nil {
		return permissionAdminResponse{}, err
	}

	m.mcConnLock.RLock()
	mcConn := m.mcConn
	m.mcConnLock.RUnlock()
	if mcConn == nil {
		return permissionAdminResponse{}, ErrPluginOffline
	}

	m.mcWriteLock.Lock()
	err = mcConn.WriteMessage(websocket.TextMessage, message)
	m.mcWriteLock.Unlock()
	if err != nil {
		return permissionAdminResponse{}, ErrPluginOffline
	}

	select {
	case response := <-responseChan:
		return response, nil
	case <-ctx.Done():
		return permissionAdminResponse{}, ctx.Err()
	}
}

func (m *WebSocketManager) resolvePendingPermissionAdminRequest(response permissionAdminResponse) {
	m.permissionAdminReqLock.Lock()
	defer m.permissionAdminReqLock.Unlock()

	ch, ok := m.pendingPermissionAdminReqByID[response.RequestID]
	if !ok {
		return
	}
	select {
	case ch <- response:
	default:
	}
}

func (m *WebSocketManager) failAllPendingPermissionAdminRequests() {
	m.permissionAdminReqLock.Lock()
	defer m.permissionAdminReqLock.Unlock()
	for id, ch := range m.pendingPermissionAdminReqByID {
		select {
		case ch <- permissionAdminResponse{RequestID: id, OK: false, Error: "plugin disconnected"}:
		default:
		}
	}
}
