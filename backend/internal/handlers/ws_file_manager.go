package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var ErrPluginOffline = errors.New("server is offline")

type fileManagerResponse struct {
	RequestID string          `json:"request_id"`
	OK        bool            `json:"ok"`
	Error     string          `json:"error"`
	Data      json.RawMessage `json:"data"`
}

func (m *WebSocketManager) RequestFileManagerOperation(ctx context.Context, action string, path string, content string) (fileManagerResponse, error) {
	if !m.isMinecraftConnected() {
		return fileManagerResponse{}, ErrPluginOffline
	}

	requestID, err := randomHex(16)
	if err != nil {
		return fileManagerResponse{}, err
	}

	responseChan := make(chan fileManagerResponse, 1)
	m.fileReqLock.Lock()
	if m.pendingFileReqByID == nil {
		m.pendingFileReqByID = make(map[string]chan fileManagerResponse)
	}
	m.pendingFileReqByID[requestID] = responseChan
	m.fileReqLock.Unlock()

	defer func() {
		m.fileReqLock.Lock()
		delete(m.pendingFileReqByID, requestID)
		m.fileReqLock.Unlock()
	}()

	payload := map[string]string{
		"request_id": requestID,
		"action":     action,
		"path":       path,
	}
	if content != "" {
		payload["content"] = content
	}

	message, err := json.Marshal(map[string]interface{}{
		"event":   "file_manager_request",
		"payload": payload,
	})
	if err != nil {
		return fileManagerResponse{}, err
	}

	m.mcConnLock.RLock()
	mcConn := m.mcConn
	m.mcConnLock.RUnlock()
	if mcConn == nil {
		return fileManagerResponse{}, ErrPluginOffline
	}
	m.mcWriteLock.Lock()
	err = mcConn.WriteMessage(websocket.TextMessage, message)
	m.mcWriteLock.Unlock()
	if err != nil {
		return fileManagerResponse{}, ErrPluginOffline
	}

	select {
	case response := <-responseChan:
		return response, nil
	case <-ctx.Done():
		return fileManagerResponse{}, ctx.Err()
	}
}

func (m *WebSocketManager) resolvePendingFileRequest(response fileManagerResponse) {
	m.fileReqLock.Lock()
	defer m.fileReqLock.Unlock()

	ch, ok := m.pendingFileReqByID[response.RequestID]
	if !ok {
		return
	}

	select {
	case ch <- response:
	default:
	}
}

func (m *WebSocketManager) failAllPendingFileRequests(message string) {
	m.fileReqLock.Lock()
	defer m.fileReqLock.Unlock()
	for id, ch := range m.pendingFileReqByID {
		select {
		case ch <- fileManagerResponse{RequestID: id, OK: false, Error: message}:
		default:
		}
	}
}

func randomHex(numBytes int) (string, error) {
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (m *WebSocketManager) sendLatestLogSnapshot(conn *websocket.Conn) error {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	resp, err := m.RequestFileManagerOperation(ctx, "read_text", "logs/latest.log", "")
	if err != nil {
		return err
	}
	if !resp.OK {
		if resp.Error == "" {
			return errors.New("failed to read latest.log")
		}
		return errors.New(resp.Error)
	}

	var payload struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		return err
	}

	if payload.Content == "" {
		return nil
	}

	lines := strings.Split(payload.Content, "\n")
	for _, line := range lines {
		clean := strings.TrimSuffix(line, "\r")
		if clean == "" {
			continue
		}

		msgPayload := map[string]string{
			"message": clean,
			"level":   detectLogLevel(clean),
		}

		envelope, err := json.Marshal(map[string]interface{}{
			"event":   "console_log",
			"payload": msgPayload,
		})
		if err != nil {
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, envelope); err != nil {
			return err
		}
	}

	return nil
}

func detectLogLevel(line string) string {
	upper := strings.ToUpper(line)
	switch {
	case strings.Contains(upper, "/SEVERE]"), strings.Contains(upper, " SEVERE "):
		return "SEVERE"
	case strings.Contains(upper, "/ERROR]"), strings.Contains(upper, " ERROR "):
		return "ERROR"
	case strings.Contains(upper, "/WARN]"), strings.Contains(upper, " WARN "):
		return "WARN"
	case strings.Contains(upper, "/DEBUG]"), strings.Contains(upper, " DEBUG "):
		return "DEBUG"
	default:
		return "INFO"
	}
}
