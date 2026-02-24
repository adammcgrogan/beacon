package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"
)

func (h *UIHandler) HandleFilesMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	response, err := h.fileRequest(r.Context(), "meta", r.URL.Query().Get("path"), "")
	if err != nil {
		writeFileError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *UIHandler) HandleFilesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	response, err := h.fileRequest(r.Context(), "list", r.URL.Query().Get("path"), "")
	if err != nil {
		writeFileError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *UIHandler) HandleFilesContent(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")

	switch r.Method {
	case http.MethodGet:
		response, err := h.fileRequest(r.Context(), "read_text", path, "")
		if err != nil {
			writeFileError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPut:
		var req struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		response, err := h.fileRequest(r.Context(), "write_text", path, req.Content)
		if err != nil {
			writeFileError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, response)
	default:
		methodNotAllowed(w)
	}
}

func (h *UIHandler) HandleFilesDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}

	response, err := h.fileRequest(r.Context(), "delete", r.URL.Query().Get("path"), "")
	if err != nil {
		writeFileError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *UIHandler) HandleFilesDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	raw, err := h.fileRequest(r.Context(), "download", r.URL.Query().Get("path"), "")
	if err != nil {
		writeFileDownloadError(w, err)
		return
	}

	var resp struct {
		FileName      string `json:"file_name"`
		ContentBase64 string `json:"content_base64"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		http.Error(w, "invalid download response", http.StatusInternalServerError)
		return
	}

	content, err := base64.StdEncoding.DecodeString(resp.ContentBase64)
	if err != nil {
		http.Error(w, "invalid download data", http.StatusInternalServerError)
		return
	}

	fileName := resp.FileName
	if fileName == "" {
		fileName = "download.bin"
	}

	w.Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (h *UIHandler) fileRequest(requestCtx context.Context, action string, path string, content string) (json.RawMessage, error) {
	if h.WS == nil {
		return nil, ErrPluginOffline
	}

	ctx, cancel := context.WithTimeout(requestCtx, 12*time.Second)
	defer cancel()

	response, err := h.WS.RequestFileManagerOperation(ctx, action, path, content)
	if err != nil {
		return nil, err
	}
	if !response.OK {
		if response.Error == "" {
			return nil, ErrPluginOffline
		}
		return nil, &fileAPIError{message: response.Error}
	}

	return response.Data, nil
}

type fileAPIError struct {
	message string
}

func (e *fileAPIError) Error() string {
	return e.message
}

func writeFileError(w http.ResponseWriter, err error) {
	if err == ErrPluginOffline {
		writeJSONError(w, http.StatusServiceUnavailable, "server is offline")
		return
	}
	if err == context.DeadlineExceeded || err == context.Canceled {
		writeJSONError(w, http.StatusGatewayTimeout, "file operation timed out")
		return
	}
	if apiErr, ok := err.(*fileAPIError); ok {
		writeJSONError(w, http.StatusBadRequest, apiErr.message)
		return
	}
	writeJSONError(w, http.StatusInternalServerError, "file operation failed")
}

func writeFileDownloadError(w http.ResponseWriter, err error) {
	if err == ErrPluginOffline {
		http.Error(w, "server is offline", http.StatusServiceUnavailable)
		return
	}
	if err == context.DeadlineExceeded || err == context.Canceled {
		http.Error(w, "file operation timed out", http.StatusGatewayTimeout)
		return
	}
	if apiErr, ok := err.(*fileAPIError); ok {
		http.Error(w, apiErr.message, http.StatusBadRequest)
		return
	}
	http.Error(w, "file operation failed", http.StatusInternalServerError)
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
