package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	PermAccessAll            = "beacon.access.*"
	PermPackDashboard        = "beacon.access.dashboard"
	PermPackConsole          = "beacon.access.console"
	PermPackPlayers          = "beacon.access.players"
	PermPackWorlds           = "beacon.access.worlds"
	PermPackFiles            = "beacon.access.files"
	PermAccessView           = "beacon.access.access"
	PermAccessManage         = "beacon.access.access.manage"
	PermDashboardView        = "beacon.access.dashboard.view"
	PermConsoleView          = "beacon.access.console.view"
	PermConsoleUse           = "beacon.access.console.use"
	PermPlayersView          = "beacon.access.players.view"
	PermPlayersKick          = "beacon.access.players.kick"
	PermPlayersBan           = "beacon.access.players.ban"
	PermWorldsView           = "beacon.access.worlds.view"
	PermWorldsManage         = "beacon.access.worlds.manage"
	PermWorldsReset          = "beacon.access.worlds.reset"
	PermWorldsGamerules      = "beacon.access.worlds.gamerules"
	PermServerStop           = "beacon.access.stop"
	PermServerRestart        = "beacon.access.restart"
	PermServerSaveAll        = "beacon.access.saveall"
	PermFilesAll             = "beacon.access.files.all"
	PermFilesView            = "beacon.access.files.view"
	PermFilesEdit            = "beacon.access.files.edit"
	PermFilesDelete          = "beacon.access.files.delete"
	PermFilesDownload        = "beacon.access.files.download"
	fileScopedPermissionBase = "beacon.access.files."
)

type SessionClaims struct {
	PlayerUUID string `json:"sub"`
	PlayerName string `json:"name"`
	SessionID  string `json:"jti"`
	IssuedAt   int64  `json:"iat"`
	ExpiresAt  int64  `json:"exp"`
}

type SessionGrants struct {
	CanViewDashboard bool `json:"can_view_dashboard"`
	CanViewConsole   bool `json:"can_view_console"`
	CanUseConsole    bool `json:"can_use_console"`
	CanViewPlayers   bool `json:"can_view_players"`
	CanKickPlayers   bool `json:"can_kick_players"`
	CanBanPlayers    bool `json:"can_ban_players"`
	CanViewWorlds    bool `json:"can_view_worlds"`
	CanManageWorlds  bool `json:"can_manage_worlds"`
	CanResetWorlds   bool `json:"can_reset_worlds"`
	CanEditGamerules bool `json:"can_edit_gamerules"`
	CanStopServer    bool `json:"can_stop_server"`
	CanRestartServer bool `json:"can_restart_server"`
	CanSaveAll       bool `json:"can_save_all"`
	CanViewFiles     bool `json:"can_view_files"`
	CanEditFiles     bool `json:"can_edit_files"`
	CanDeleteFiles   bool `json:"can_delete_files"`
	CanDownloadFiles bool `json:"can_download_files"`
	CanViewAccess    bool `json:"can_view_access"`
	CanManageAccess  bool `json:"can_manage_access"`
}

type AuthManager struct {
	mu sync.RWMutex

	magicTokens map[string]magicToken
	permCache   map[string]permissionCache
	sessions    map[string]webSession
	users       map[string]knownUser

	sessionTTL  time.Duration
	permTTL     time.Duration
	cookieName  string
	signingKey  []byte
	statePath   string
	stateLoaded bool
}

type magicToken struct {
	PlayerUUID  string
	PlayerName  string
	ExpiresAt   time.Time
	Permissions []string
}

type permissionCache struct {
	Permissions []string
	Online      bool
	FetchedAt   time.Time
}

type webSession struct {
	ID         string
	PlayerUUID string
	PlayerName string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	Revoked    bool
}

type knownUser struct {
	PlayerUUID string
	PlayerName string
	FirstSeen  time.Time
	LastSeen   time.Time
}

type persistedAuthState struct {
	SigningKey string       `json:"signing_key"`
	Sessions   []webSession `json:"sessions"`
	Users      []knownUser  `json:"users"`
}

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrExpiredToken = errors.New("token expired")
	ErrInvalidToken = errors.New("invalid token")
	ErrNoSession    = errors.New("session missing")
)

func NewAuthManager() *AuthManager {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	statePath := strings.TrimSpace(os.Getenv("BEACON_AUTH_STATE_PATH"))

	return &AuthManager{
		magicTokens: make(map[string]magicToken),
		permCache:   make(map[string]permissionCache),
		sessions:    make(map[string]webSession),
		users:       make(map[string]knownUser),
		sessionTTL:  24 * time.Hour,
		permTTL:     10 * time.Second,
		cookieName:  "beacon_session",
		signingKey:  key,
		statePath:   statePath,
	}
}

func (a *AuthManager) loadPersistedState() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if strings.TrimSpace(a.statePath) == "" {
		return
	}

	path := filepath.Clean(a.statePath)
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("beacon auth: failed reading state file: %v", err)
		}
		return
	}

	var state persistedAuthState
	if err := json.Unmarshal(data, &state); err != nil {
		log.Printf("beacon auth: failed parsing state file: %v", err)
		return
	}

	if key, err := base64.StdEncoding.DecodeString(state.SigningKey); err == nil && len(key) > 0 {
		a.signingKey = key
	}

	now := time.Now()
	a.sessions = make(map[string]webSession)
	for _, session := range state.Sessions {
		if session.ID == "" || session.Revoked || now.After(session.ExpiresAt) {
			continue
		}
		a.sessions[session.ID] = session
	}

	a.users = make(map[string]knownUser)
	for _, user := range state.Users {
		if user.PlayerUUID == "" {
			continue
		}
		a.users[user.PlayerUUID] = user
	}
	a.stateLoaded = true
}

func (a *AuthManager) LoadPersistedState() {
	a.loadPersistedState()
}

func (a *AuthManager) SetPluginDataDir(pluginDataDir string) {
	pluginDataDir = strings.TrimSpace(pluginDataDir)
	if pluginDataDir == "" {
		return
	}
	newPath := filepath.Join(pluginDataDir, "auth_state.json")

	a.mu.Lock()
	currentPath := a.statePath
	alreadyLoaded := a.stateLoaded
	if currentPath == newPath {
		a.mu.Unlock()
		return
	}
	a.statePath = newPath
	a.stateLoaded = false
	a.mu.Unlock()

	if strings.TrimSpace(currentPath) != "" {
		if _, err := os.Stat(newPath); errors.Is(err, os.ErrNotExist) {
			if data, readErr := os.ReadFile(filepath.Clean(currentPath)); readErr == nil {
				_ = os.MkdirAll(filepath.Dir(newPath), 0o755)
				_ = os.WriteFile(newPath, data, 0o600)
			}
		}
	}

	if !alreadyLoaded {
		a.loadPersistedState()
		return
	}
	if _, err := os.Stat(newPath); err == nil {
		a.loadPersistedState()
	} else {
		a.persistState()
	}
}

func (a *AuthManager) persistState() {
	a.mu.RLock()
	if strings.TrimSpace(a.statePath) == "" {
		a.mu.RUnlock()
		return
	}
	state := persistedAuthState{
		SigningKey: base64.StdEncoding.EncodeToString(a.signingKey),
		Sessions:   make([]webSession, 0, len(a.sessions)),
		Users:      make([]knownUser, 0, len(a.users)),
	}
	for _, session := range a.sessions {
		if session.Revoked {
			continue
		}
		state.Sessions = append(state.Sessions, session)
	}
	for _, user := range a.users {
		state.Users = append(state.Users, user)
	}
	a.mu.RUnlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Printf("beacon auth: failed encoding state: %v", err)
		return
	}

	cleanPath := filepath.Clean(a.statePath)

	// Ensure the auth state file path stays within the current working directory.
	baseDir, err := os.Getwd()
	if err != nil {
		log.Printf("beacon auth: failed to determine working directory: %v", err)
		return
	}
	baseDir = filepath.Clean(baseDir)

	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		log.Printf("beacon auth: failed to resolve absolute state path: %v", err)
		return
	}

	if absPath != baseDir && !strings.HasPrefix(absPath, baseDir+string(os.PathSeparator)) {
		log.Printf("beacon auth: refusing to write state outside working directory: %s", absPath)
		return
	}

	path := absPath
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("beacon auth: failed creating state directory: %v", err)
		return
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		log.Printf("beacon auth: failed writing temp state: %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Printf("beacon auth: failed replacing state file: %v", err)
	}
}

func (a *AuthManager) StoreMagicToken(rawToken, playerUUID, playerName string, expiresAtUnix int64, permissions []string) {
	if rawToken == "" || playerUUID == "" {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.magicTokens[hashToken(rawToken)] = magicToken{
		PlayerUUID:  playerUUID,
		PlayerName:  playerName,
		ExpiresAt:   time.Unix(expiresAtUnix, 0),
		Permissions: normalizePermissions(permissions),
	}
}

func (a *AuthManager) ConsumeMagicToken(rawToken string) (SessionClaims, error) {
	if rawToken == "" {
		return SessionClaims{}, ErrInvalidToken
	}

	tokenHash := hashToken(rawToken)

	a.mu.Lock()
	defer a.mu.Unlock()

	entry, ok := a.magicTokens[tokenHash]
	if !ok {
		return SessionClaims{}, ErrInvalidToken
	}
	delete(a.magicTokens, tokenHash)

	if time.Now().After(entry.ExpiresAt) {
		return SessionClaims{}, ErrExpiredToken
	}

	now := time.Now().Unix()
	nowTime := time.Now()
	if len(entry.Permissions) > 0 {
		a.permCache[entry.PlayerUUID] = permissionCache{
			Permissions: entry.Permissions,
			Online:      true,
			FetchedAt:   nowTime,
		}
	}

	sessionID, err := randomHex(16)
	if err != nil || sessionID == "" {
		sessionID = hashToken(entry.PlayerUUID + "|" + nowTime.String())
	}
	a.sessions[sessionID] = webSession{
		ID:         sessionID,
		PlayerUUID: entry.PlayerUUID,
		PlayerName: entry.PlayerName,
		CreatedAt:  nowTime,
		LastSeenAt: nowTime,
		ExpiresAt:  nowTime.Add(a.sessionTTL),
		Revoked:    false,
	}
	existingUser, exists := a.users[entry.PlayerUUID]
	if !exists {
		a.users[entry.PlayerUUID] = knownUser{
			PlayerUUID: entry.PlayerUUID,
			PlayerName: entry.PlayerName,
			FirstSeen:  nowTime,
			LastSeen:   nowTime,
		}
	} else {
		existingUser.PlayerName = entry.PlayerName
		existingUser.LastSeen = nowTime
		a.users[entry.PlayerUUID] = existingUser
	}
	go a.persistState()

	return SessionClaims{
		PlayerUUID: entry.PlayerUUID,
		PlayerName: entry.PlayerName,
		SessionID:  sessionID,
		IssuedAt:   now,
		ExpiresAt:  now + int64(a.sessionTTL.Seconds()),
	}, nil
}

func (a *AuthManager) SessionCookieName() string {
	return a.cookieName
}

func (a *AuthManager) EncodeSession(claims SessionClaims) (string, error) {
	headerBytes, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	header := base64.RawURLEncoding.EncodeToString(headerBytes)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	unsigned := header + "." + payload

	mac := hmac.New(sha256.New, a.signingKey)
	_, _ = mac.Write([]byte(unsigned))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + signature, nil
}

func (a *AuthManager) DecodeSession(token string) (SessionClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return SessionClaims{}, ErrUnauthorized
	}

	unsigned := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, a.signingKey)
	_, _ = mac.Write([]byte(unsigned))
	expected := mac.Sum(nil)

	actual, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(actual, expected) {
		return SessionClaims{}, ErrUnauthorized
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return SessionClaims{}, ErrUnauthorized
	}

	var claims SessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return SessionClaims{}, ErrUnauthorized
	}
	if claims.PlayerUUID == "" || time.Now().Unix() >= claims.ExpiresAt {
		return SessionClaims{}, ErrUnauthorized
	}
	if claims.SessionID == "" {
		return SessionClaims{}, ErrUnauthorized
	}

	a.mu.RLock()
	session, ok := a.sessions[claims.SessionID]
	a.mu.RUnlock()
	if !ok || session.Revoked || time.Now().After(session.ExpiresAt) {
		return SessionClaims{}, ErrUnauthorized
	}

	return claims, nil
}

func (a *AuthManager) SetSessionCookie(w http.ResponseWriter, token string, expiresAt int64) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(expiresAt, 0),
	})
}

func (a *AuthManager) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func (a *AuthManager) ReadSessionClaims(r *http.Request) (SessionClaims, error) {
	cookie, err := r.Cookie(a.cookieName)
	if err != nil {
		return SessionClaims{}, ErrNoSession
	}
	claims, err := a.DecodeSession(cookie.Value)
	if err != nil {
		return SessionClaims{}, err
	}

	a.mu.Lock()
	if session, ok := a.sessions[claims.SessionID]; ok {
		session.LastSeenAt = time.Now()
		a.sessions[claims.SessionID] = session
	}
	if user, ok := a.users[claims.PlayerUUID]; ok {
		user.LastSeen = time.Now()
		a.users[claims.PlayerUUID] = user
	}
	a.mu.Unlock()
	return claims, nil
}

func (a *AuthManager) StartJanitor() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			changed := false
			a.mu.Lock()
			for key, token := range a.magicTokens {
				if now.After(token.ExpiresAt) {
					delete(a.magicTokens, key)
					changed = true
				}
			}
			for id, session := range a.sessions {
				if session.Revoked || now.After(session.ExpiresAt) {
					delete(a.sessions, id)
					changed = true
				}
			}
			for uuid, cache := range a.permCache {
				if now.Sub(cache.FetchedAt) > 30*time.Second {
					delete(a.permCache, uuid)
				}
			}
			a.mu.Unlock()
			if changed {
				a.persistState()
			}
		}
	}()
}

func (a *AuthManager) GetPermissions(ctx context.Context, ws *WebSocketManager, playerUUID string) ([]string, bool, error) {
	now := time.Now()
	a.mu.RLock()
	cached, ok := a.permCache[playerUUID]
	a.mu.RUnlock()

	if ok && now.Sub(cached.FetchedAt) <= a.permTTL {
		return cached.Permissions, cached.Online, nil
	}
	if ws == nil {
		return nil, false, ErrPluginOffline
	}

	permissions, online, err := ws.RequestPlayerPermissions(ctx, playerUUID)
	if err != nil {
		if ok {
			return cached.Permissions, cached.Online, nil
		}
		return nil, false, err
	}

	normalized := normalizePermissions(permissions)
	a.mu.Lock()
	a.permCache[playerUUID] = permissionCache{
		Permissions: normalized,
		Online:      online,
		FetchedAt:   now,
	}
	a.mu.Unlock()
	return normalized, online, nil
}

func (a *AuthManager) InvalidatePermissionCache(playerUUID string) {
	a.mu.Lock()
	delete(a.permCache, playerUUID)
	a.mu.Unlock()
}

func DeriveSessionGrants(permissions []string) SessionGrants {
	return SessionGrants{
		CanViewDashboard: HasPermission(permissions, PermDashboardView),
		CanViewConsole:   HasPermission(permissions, PermConsoleView),
		CanUseConsole:    HasPermission(permissions, PermConsoleUse),
		CanViewPlayers:   HasPermission(permissions, PermPlayersView),
		CanKickPlayers:   HasPermission(permissions, PermPlayersKick),
		CanBanPlayers:    HasPermission(permissions, PermPlayersBan),
		CanViewWorlds:    HasPermission(permissions, PermWorldsView),
		CanManageWorlds:  HasPermission(permissions, PermWorldsManage),
		CanResetWorlds:   HasPermission(permissions, PermWorldsReset),
		CanEditGamerules: HasPermission(permissions, PermWorldsGamerules),
		CanStopServer:    HasPermission(permissions, PermServerStop),
		CanRestartServer: HasPermission(permissions, PermServerRestart),
		CanSaveAll:       HasPermission(permissions, PermServerSaveAll),
		CanViewFiles:     CanAccessAnyFileView(permissions),
		CanEditFiles:     HasPermission(permissions, PermFilesEdit),
		CanDeleteFiles:   HasPermission(permissions, PermFilesDelete),
		CanDownloadFiles: HasPermission(permissions, PermFilesDownload),
		CanViewAccess:    HasAnyPermission(permissions, PermAccessAll, PermAccessView, PermAccessManage),
		CanManageAccess:  HasAnyPermission(permissions, PermAccessAll, PermAccessManage),
	}
}

func (a *AuthManager) ListKnownUsersWithSessions() ([]knownUser, []webSession) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	users := make([]knownUser, 0, len(a.users))
	for _, u := range a.users {
		users = append(users, u)
	}

	sessions := make([]webSession, 0, len(a.sessions))
	for _, s := range a.sessions {
		sessions = append(sessions, s)
	}
	return users, sessions
}

func (a *AuthManager) RevokeSession(sessionID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	session, ok := a.sessions[sessionID]
	if !ok {
		return false
	}
	session.Revoked = true
	a.sessions[sessionID] = session
	go a.persistState()
	return true
}

func HasPermission(permissions []string, required string) bool {
	required = strings.ToLower(strings.TrimSpace(required))
	if required == "" {
		return false
	}

	for _, granted := range permissions {
		granted = strings.ToLower(strings.TrimSpace(granted))
		if granted == "" {
			continue
		}
		if granted == required {
			return true
		}
		if permissionPackImplies(granted, required) {
			return true
		}
		if strings.HasSuffix(granted, ".*") {
			prefix := strings.TrimSuffix(granted, ".*")
			if required == prefix || strings.HasPrefix(required, prefix+".") {
				return true
			}
		}
	}
	return false
}

func permissionPackImplies(granted, required string) bool {
	switch granted {
	case PermPackDashboard:
		return required == PermDashboardView ||
			required == PermServerStop ||
			required == PermServerRestart ||
			required == PermServerSaveAll
	case PermPackConsole:
		return required == PermConsoleView ||
			required == PermConsoleUse
	case PermPackPlayers:
		return required == PermPlayersView ||
			required == PermPlayersKick ||
			required == PermPlayersBan
	case PermPackWorlds:
		return required == PermWorldsView ||
			required == PermWorldsManage ||
			required == PermWorldsReset ||
			required == PermWorldsGamerules
	case PermPackFiles:
		return required == PermFilesAll ||
			required == PermFilesView ||
			required == PermFilesEdit ||
			required == PermFilesDelete ||
			required == PermFilesDownload
	case PermAccessManage:
		return required == PermAccessView
	default:
		return false
	}
}

func HasAnyPermission(permissions []string, required ...string) bool {
	for _, permission := range required {
		if HasPermission(permissions, permission) {
			return true
		}
	}
	return false
}

func CanAccessAnyFileView(permissions []string) bool {
	if HasAnyPermission(permissions, PermAccessAll, PermPackFiles, PermFilesAll, PermFilesView) {
		return true
	}
	for _, granted := range permissions {
		if strings.HasPrefix(strings.ToLower(granted), "beacon.access.files.view.") {
			return true
		}
	}
	return false
}

func CanAccessFilePath(permissions []string, action string, rawPath string) bool {
	if HasAnyPermission(permissions, PermAccessAll, PermPackFiles, PermFilesAll) {
		return true
	}

	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "view", "edit", "delete", "download":
	default:
		return false
	}

	globalPermission := fileScopedPermissionBase + action
	if HasPermission(permissions, globalPermission) {
		return true
	}

	keys := filePermissionKeys(rawPath)
	if len(keys) == 0 {
		return action == "view" && CanAccessAnyFileView(permissions)
	}

	for _, key := range keys {
		if HasPermission(permissions, globalPermission+"."+key) {
			return true
		}
	}
	return false
}

func filePermissionKeys(rawPath string) []string {
	trimmed := strings.TrimSpace(strings.Trim(rawPath, "/"))
	if trimmed == "" {
		return nil
	}

	cleaned := path.Clean("/" + trimmed)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" || cleaned == "." || strings.HasPrefix(cleaned, "..") {
		return nil
	}

	segments := strings.Split(cleaned, "/")
	sanitized := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" || segment == "." {
			continue
		}
		if segment == ".." {
			return nil
		}
		sanitized = append(sanitized, sanitizePermissionSegment(segment))
	}
	if len(sanitized) == 0 {
		return nil
	}

	keys := make([]string, 0, len(sanitized))
	for i := range sanitized {
		keys = append(keys, strings.Join(sanitized[:i+1], "."))
	}
	return keys
}

func normalizePermissions(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func sanitizePermissionSegment(value string) string {
	lower := strings.ToLower(value)
	var b strings.Builder
	b.Grow(len(lower))
	lastUnderscore := false
	for _, r := range lower {
		isAllowed := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-'
		if isAllowed {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "path"
	}
	return result
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
