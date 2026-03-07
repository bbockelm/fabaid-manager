package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"

	"github.com/bbockelm/fabaid-manager/internal/models"
)

const (
	sessionCookieName  = "fabaid_session"
	sessionDuration    = 7 * 24 * time.Hour
	inviteExpiry       = 7 * 24 * time.Hour
	RoleAdmin          = "admin"
	RoleGrantAdmin     = "grant_admin"
	RoleSubawardAdmin  = "subaward_admin"
	RoleReadOnly       = "read_only"
	sessionTokenBytes  = 32 // 256 bits of entropy for session/invite tokens
)

var validRoles = map[string]bool{
	RoleAdmin:         true,
	RoleGrantAdmin:    true,
	RoleSubawardAdmin: true,
	RoleReadOnly:      true,
}

type contextKey string

// hashToken computes SHA-256 of a raw token string.
// SHA-256 is appropriate here (not bcrypt) because the input has 256 bits
// of cryptographic randomness — there is nothing to brute-force.
func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// generateToken creates a cryptographically random token and its SHA-256 hash.
// The raw token is URL-safe base64 (no padding); the hash is stored in the DB.
func generateToken() (rawToken string, tokenHash []byte, err error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err = rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generating random token: %w", err)
	}
	rawToken = base64.RawURLEncoding.EncodeToString(buf)
	tokenHash = hashToken(rawToken)
	return rawToken, tokenHash, nil
}

const sessionContextKey contextKey = "session"
const userContextKey contextKey = "user"

// GetSessionFromContext returns the session stored in context by the auth middleware.
func GetSessionFromContext(ctx context.Context) *models.Session {
	s, _ := ctx.Value(sessionContextKey).(*models.Session)
	return s
}

// GetUserFromContext returns the user stored in context by the auth middleware.
func GetUserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(userContextKey).(*models.User)
	return u
}

// --- Auth middleware ---

// RequireAuth checks for a valid session cookie.
func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			respondError(w, http.StatusUnauthorized, "Not authenticated")
			return
		}

		session, err := h.queries.GetSession(r.Context(), hashToken(cookie.Value))
		if err != nil {
			http.SetCookie(w, &http.Cookie{
				Name: sessionCookieName, Value: "", Path: "/",
				MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode,
			})
			respondError(w, http.StatusUnauthorized, "Session expired")
			return
		}

		user, err := h.queries.GetUser(r.Context(), session.UserID)
		if err != nil || user.Status != "active" {
			respondError(w, http.StatusUnauthorized, "User not active")
			return
		}

		ctx := context.WithValue(r.Context(), sessionContextKey, session)
		ctx = context.WithValue(ctx, userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole checks that the session role is one of the allowed roles.
func RequireRole(allowed ...string) func(http.Handler) http.Handler {
	allowedSet := make(map[string]bool)
	for _, r := range allowed {
		allowedSet[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session := GetSessionFromContext(r.Context())
			if session == nil || !allowedSet[session.Role] {
				respondError(w, http.StatusForbidden, "Insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// effectiveRole returns the highest-priority role from a list of UserRoles.
// Priority: admin > grant_admin > subaward_admin > read_only
func effectiveRole(roles []models.UserRole) string {
	best := RoleReadOnly
	for _, ur := range roles {
		switch ur.Role {
		case RoleAdmin:
			return RoleAdmin
		case RoleGrantAdmin:
			best = RoleGrantAdmin
		case RoleSubawardAdmin:
			if best == RoleReadOnly {
				best = RoleSubawardAdmin
			}
		}
	}
	return best
}

// RequireWriteAccess blocks read_only users from mutating endpoints.
func RequireWriteAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := GetSessionFromContext(r.Context())
		if session == nil {
			respondError(w, http.StatusUnauthorized, "Not authenticated")
			return
		}
		if session.Role == RoleReadOnly && r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			respondError(w, http.StatusForbidden, "Read-only access: cannot modify data")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Auth endpoints ---

// GetCurrentSession returns info about the current user/session.
func (h *Handler) GetCurrentSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		respondJSON(w, http.StatusOK, models.SessionInfo{})
		return
	}
	session, err := h.queries.GetSession(r.Context(), hashToken(cookie.Value))
	if err != nil {
		respondJSON(w, http.StatusOK, models.SessionInfo{})
		return
	}
	user, _ := h.queries.GetUser(r.Context(), session.UserID)
	roles, _ := h.queries.ListUserRoles(r.Context(), session.UserID)
	roleStrs := make([]string, len(roles))
	for i, rl := range roles {
		roleStrs[i] = rl.Role
	}
	info := models.SessionInfo{
		User:  user,
		Role:  session.Role,
		Roles: roleStrs,
	}
	// If the user has subaward_admin role, include their permitted institutions
	for _, rs := range roleStrs {
		if rs == RoleSubawardAdmin {
			insts, _ := h.queries.ListUserInstitutionNames(r.Context(), session.UserID)
			if insts == nil {
				insts = []string{}
			}
			info.Institutions = insts
			break
		}
	}
	respondJSON(w, http.StatusOK, info)
}

// Logout destroys the current session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && cookie.Value != "" {
		_ = h.queries.DeleteSession(r.Context(), hashToken(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/",
		MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	respondJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// --- Dev login (development mode only) ---

// DevLogin creates a session with a chosen role in development mode.
func (h *Handler) DevLogin(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.IsDevelopment() {
		respondError(w, http.StatusNotFound, "Not found")
		return
	}

	var req struct {
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = "Dev User"
	}
	if !validRoles[req.Role] {
		req.Role = RoleAdmin
	}

	// Find or create a dev user
	devIssuer := "dev"
	devSubject := "dev-user"
	identity, err := h.queries.FindIdentity(r.Context(), devIssuer, devSubject)
	var userID string
	if err != nil {
		// Create a new dev user
		user := &models.User{DisplayName: req.DisplayName, Status: "active"}
		if err := h.queries.CreateUser(r.Context(), user); err != nil {
			log.Error().Err(err).Msg("Failed to create dev user")
			respondError(w, http.StatusInternalServerError, "Failed to create dev user")
			return
		}
		userID = user.ID
		ident := &models.UserIdentity{UserID: userID, Issuer: devIssuer, Subject: devSubject}
		_ = h.queries.CreateIdentity(r.Context(), ident)
	} else {
		userID = identity.UserID
		user, _ := h.queries.GetUser(r.Context(), userID)
		if user != nil {
			user.DisplayName = req.DisplayName
			_ = h.queries.UpdateUser(r.Context(), user)
		}
	}

	// Ensure the requested role is assigned
	_ = h.queries.AddUserRole(r.Context(), userID, req.Role)
	_ = h.queries.UpdateUserLastLogin(r.Context(), userID)

	// Delete any existing sessions for this user
	_ = h.queries.DeleteUserSessions(r.Context(), userID)

	// Create session with hashed token
	rawToken, tokenHash, genErr := generateToken()
	if genErr != nil {
		log.Error().Err(err).Msg("Failed to generate session token")
		respondError(w, http.StatusInternalServerError, "Failed to generate session token")
		return
	}
	session := &models.Session{
		UserID:    userID,
		Role:      req.Role,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(sessionDuration),
	}
	if err := h.queries.CreateSession(r.Context(), session); err != nil {
		log.Error().Err(err).Msg("Failed to create session")
		respondError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: rawToken, Path: "/",
		MaxAge: int(sessionDuration.Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	respondJSON(w, http.StatusOK, map[string]any{
		"session_id": session.ID,
		"role":       req.Role,
		"user_id":    userID,
	})
}

// GetAuthMode returns the auth configuration for the frontend.
func (h *Handler) GetAuthMode(w http.ResponseWriter, r *http.Request) {
	mode := "oidc"
	if h.cfg.IsDevelopment() {
		mode = "dev"
	}
	oidcConfigured := false

	issuer, _ := h.queries.GetAppConfig(r.Context(), "oidc_issuer")
	clientID, _ := h.queries.GetAppConfig(r.Context(), "oidc_client_id")
	if issuer != "" && clientID != "" {
		oidcConfigured = true
	} else if h.cfg.OIDCIssuer != "" && h.cfg.OIDCClientID != "" {
		oidcConfigured = true
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"mode":            mode,
		"oidc_configured": oidcConfigured,
		"callback_url":    h.cfg.BaseURL + "/api/v1/auth/oidc/callback",
	})
}

// --- OIDC flow ---

func (h *Handler) getOIDCConfig(ctx context.Context) (issuer, clientID, clientSecret string, err error) {
	issuer, _ = h.queries.GetAppConfig(ctx, "oidc_issuer")
	clientID, _ = h.queries.GetAppConfig(ctx, "oidc_client_id")
	clientSecret, _ = h.getDecryptedConfig(ctx, "oidc_client_secret")
	if issuer != "" && clientID != "" {
		return
	}
	issuer = h.cfg.OIDCIssuer
	clientID = h.cfg.OIDCClientID
	clientSecret = h.cfg.OIDCClientSecret
	if issuer == "" || clientID == "" {
		err = fmt.Errorf("OIDC not configured")
	}
	return
}

// getDecryptedConfig reads a config value and decrypts it if the encryptor is available.
func (h *Handler) getDecryptedConfig(ctx context.Context, key string) (string, error) {
	val, err := h.queries.GetAppConfig(ctx, key)
	if err != nil {
		return "", err
	}
	if h.encryptor != nil {
		return h.encryptor.DecryptConfigValue(val)
	}
	return val, nil
}

// setEncryptedConfig encrypts a config value before storing it.
func (h *Handler) setEncryptedConfig(ctx context.Context, key, plaintext string) error {
	if h.encryptor != nil && plaintext != "" {
		encrypted, err := h.encryptor.EncryptConfigValue(plaintext)
		if err != nil {
			return fmt.Errorf("encrypting config %s: %w", key, err)
		}
		return h.queries.SetAppConfig(ctx, key, encrypted)
	}
	return h.queries.SetAppConfig(ctx, key, plaintext)
}

// OIDCLogin initiates the OIDC authorization code flow.
func (h *Handler) OIDCLogin(w http.ResponseWriter, r *http.Request) {
	issuer, clientID, _, err := h.getOIDCConfig(r.Context())
	if err != nil {
		respondError(w, http.StatusBadRequest, "OIDC not configured")
		return
	}

	inviteToken := r.URL.Query().Get("invite")

	stateBytes := make([]byte, 16)
	_, _ = rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)

	statePayload := state
	if inviteToken != "" {
		statePayload = state + "|" + inviteToken
	}
	mac := hmac.New(sha256.New, []byte(h.cfg.SessionSecret))
	mac.Write([]byte(statePayload))
	sig := hex.EncodeToString(mac.Sum(nil))

	http.SetCookie(w, &http.Cookie{
		Name: "fabaid_oidc_state", Value: sig + ":" + statePayload, Path: "/",
		MaxAge: 600, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})

	callbackURL := h.cfg.BaseURL + "/api/v1/auth/oidc/callback"

	authURL := issuer + "/authorize"
	wellKnown, wkErr := fetchWellKnown(issuer)
	if wkErr == nil && wellKnown.AuthorizationEndpoint != "" {
		authURL = wellKnown.AuthorizationEndpoint
	}

	redirectURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
		authURL, clientID, callbackURL, buildOIDCScopes(issuer), state)

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// OIDCCallback handles the OIDC authorization code callback.
func (h *Handler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		respondError(w, http.StatusBadRequest, "Missing code or state")
		return
	}

	stateCookie, err := r.Cookie("fabaid_oidc_state")
	if err != nil || stateCookie.Value == "" {
		respondError(w, http.StatusBadRequest, "Missing state cookie")
		return
	}

	parts := strings.SplitN(stateCookie.Value, ":", 2)
	if len(parts) != 2 {
		respondError(w, http.StatusBadRequest, "Invalid state cookie")
		return
	}
	storedSig := parts[0]
	statePayload := parts[1]

	mac := hmac.New(sha256.New, []byte(h.cfg.SessionSecret))
	mac.Write([]byte(statePayload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(storedSig), []byte(expectedSig)) {
		respondError(w, http.StatusBadRequest, "State verification failed")
		return
	}

	stateParts := strings.SplitN(statePayload, "|", 2)
	originalState := stateParts[0]
	var inviteToken string
	if len(stateParts) == 2 {
		inviteToken = stateParts[1]
	}

	if state != originalState {
		respondError(w, http.StatusBadRequest, "State mismatch")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name: "fabaid_oidc_state", Value: "", Path: "/", MaxAge: -1,
	})

	issuer, clientID, clientSecret, err := h.getOIDCConfig(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("OIDC not configured")
		respondError(w, http.StatusInternalServerError, "OIDC not configured")
		return
	}

	wellKnown, wkErr := fetchWellKnown(issuer)
	tokenURL := issuer + "/token"
	if wkErr == nil && wellKnown.TokenEndpoint != "" {
		tokenURL = wellKnown.TokenEndpoint
	}

	callbackURL := h.cfg.BaseURL + "/api/v1/auth/oidc/callback"
	tokenResp, err := exchangeCode(tokenURL, code, clientID, clientSecret, callbackURL)
	if err != nil {
		log.Error().Err(err).Msg("Failed to exchange OIDC code")
		respondError(w, http.StatusInternalServerError, "Failed to exchange code")
		return
	}

	claims, err := parseIDToken(tokenResp.IDToken)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse ID token")
		respondError(w, http.StatusInternalServerError, "Failed to parse ID token")
		return
	}

	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	oidcName, _ := claims["name"].(string)
	if sub == "" {
		respondError(w, http.StatusBadRequest, "ID token missing subject")
		return
	}

	// Fetch userinfo for extra claims (especially CILogon)
	var userinfoData map[string]any
	if wellKnown != nil && wellKnown.UserinfoEndpoint != "" && tokenResp.AccessToken != "" {
		ui, uiErr := fetchUserinfo(wellKnown.UserinfoEndpoint, tokenResp.AccessToken)
		if uiErr == nil {
			userinfoData = ui
			// Fill in name from userinfo if not in ID token
			if oidcName == "" {
				if n, ok := ui["name"].(string); ok && n != "" {
					oidcName = n
				}
			}
			if email == "" {
				if e, ok := ui["email"].(string); ok && e != "" {
					email = e
				}
			}
		} else {
			log.Warn().Err(uiErr).Msg("Failed to fetch userinfo")
		}
	}

	// Extract CILogon-specific claims
	var cilogonID, eppn, oidcClaim, idpName string
	if strings.Contains(issuer, "cilogon.org") && userinfoData != nil {
		if v, ok := userinfoData["id"].(string); ok {
			cilogonID = v
		}
		if v, ok := userinfoData["eppn"].(string); ok {
			eppn = v
		}
		if v, ok := userinfoData["oidc"].(string); ok {
			oidcClaim = v
		}
		if v, ok := userinfoData["idp_name"].(string); ok {
			idpName = v
		}
	}

	ctx := r.Context()
	identity, err := h.queries.FindIdentity(ctx, issuer, sub)

	if err != nil && inviteToken != "" {
		// New identity + invite
		invite, invErr := h.queries.GetInviteByToken(ctx, hashToken(inviteToken))
		if invErr != nil || invite.Used || invite.ExpiresAt.Before(time.Now()) {
			http.Redirect(w, r, h.cfg.BaseURL+"/login?error=invalid_invite", http.StatusFound)
			return
		}

		ident := &models.UserIdentity{
			UserID: invite.UserID, Issuer: issuer, Subject: sub, Email: email,
			EPPN: eppn, OIDC: oidcClaim, CILogonID: cilogonID, IdPName: idpName, DisplayName: oidcName,
		}
		if createErr := h.queries.CreateIdentity(ctx, ident); createErr != nil {
			if pgErr, ok := createErr.(*pgconn.PgError); ok && pgErr.Code == "23505" {
				log.Warn().Str("issuer", issuer).Str("subject", sub).Msg("Identity already linked to another account")
				http.Redirect(w, r, h.cfg.BaseURL+"/login?error=identity_already_linked", http.StatusFound)
				return
			}
			log.Error().Err(createErr).Msg("Failed to create identity")
			respondError(w, http.StatusInternalServerError, "Failed to link identity")
			return
		}

		// Use the user's existing highest role (roles were set when user was created)
		userRoles, _ := h.queries.ListUserRoles(ctx, invite.UserID)
		sessRole := effectiveRole(userRoles)

		_ = h.queries.MarkInviteUsed(ctx, invite.ID)
		_ = h.queries.UpdateUserLastLogin(ctx, invite.UserID)

		// If OIDC provided a name and user still has the admin-given placeholder, update it
		if oidcName != "" {
			invUser, _ := h.queries.GetUser(ctx, invite.UserID)
			if invUser != nil && invUser.DisplayName != "" {
				// Keep existing — user will be able to change on welcome page
			}
		}

		rawToken, tokenHash, genErr := generateToken()
		if genErr != nil {
			log.Error().Err(err).Msg("Failed to generate session token")
			respondError(w, http.StatusInternalServerError, "Failed to generate session token")
			return
		}
		session := &models.Session{
			UserID: invite.UserID, Role: sessRole,
			TokenHash: tokenHash,
			ExpiresAt: time.Now().Add(sessionDuration),
		}
		if sessErr := h.queries.CreateSession(ctx, session); sessErr != nil {
			log.Error().Err(err).Msg("Failed to create session")
			respondError(w, http.StatusInternalServerError, "Failed to create session")
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name: sessionCookieName, Value: rawToken, Path: "/",
			MaxAge: int(sessionDuration.Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode,
		})

		// Pass OIDC-supplied name to the welcome page so it can auto-populate
		if oidcName != "" {
			http.SetCookie(w, &http.Cookie{
				Name: "fabaid_oidc_name", Value: oidcName, Path: "/",
				MaxAge: 300, HttpOnly: false, SameSite: http.SameSiteLaxMode,
			})
		}

		http.Redirect(w, r, h.cfg.BaseURL+"/login/welcome", http.StatusFound)
		return
	}

	if err != nil {
		http.Redirect(w, r, h.cfg.BaseURL+"/login?error=no_account", http.StatusFound)
		return
	}

	// Existing identity
	_ = h.queries.UpdateUserLastLogin(ctx, identity.UserID)
	roles, _ := h.queries.ListUserRoles(ctx, identity.UserID)
	sessRole := effectiveRole(roles)

	rawToken, tokenHash, genErr := generateToken()
	if genErr != nil {
		log.Error().Err(err).Msg("Failed to generate session token")
		respondError(w, http.StatusInternalServerError, "Failed to generate session token")
		return
	}
	session := &models.Session{
		UserID: identity.UserID, Role: sessRole,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(sessionDuration),
	}
	if sessErr := h.queries.CreateSession(ctx, session); sessErr != nil {
		log.Error().Err(err).Msg("Failed to create session")
		respondError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: rawToken, Path: "/",
		MaxAge: int(sessionDuration.Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, h.cfg.BaseURL+"/", http.StatusFound)
}

// --- OIDC Config management (admin only) ---

func (h *Handler) GetOIDCConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	issuer, _ := h.queries.GetAppConfig(ctx, "oidc_issuer")
	clientID, _ := h.queries.GetAppConfig(ctx, "oidc_client_id")
	secretSet := false
	if s, _ := h.queries.GetAppConfig(ctx, "oidc_client_secret"); s != "" {
		secretSet = true
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"oidc_issuer":    issuer,
		"oidc_client_id": clientID,
		"secret_set":     secretSet,
		"callback_url":   h.cfg.BaseURL + "/api/v1/auth/oidc/callback",
	})
}

func (h *Handler) UpdateOIDCConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Issuer       string `json:"oidc_issuer"`
		ClientID     string `json:"oidc_client_id"`
		ClientSecret string `json:"oidc_client_secret"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	ctx := r.Context()
	if req.Issuer != "" {
		_ = h.queries.SetAppConfig(ctx, "oidc_issuer", req.Issuer)
	}
	if req.ClientID != "" {
		_ = h.queries.SetAppConfig(ctx, "oidc_client_id", req.ClientID)
	}
	if req.ClientSecret != "" {
		_ = h.setEncryptedConfig(ctx, "oidc_client_secret", req.ClientSecret)
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- User management (admin only) ---

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.queries.ListUsers(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list users")
		respondError(w, http.StatusInternalServerError, "Failed to list users")
		return
	}

	type userInfo struct {
		models.User
		Roles        []string              `json:"roles"`
		Identities   []models.UserIdentity `json:"identities"`
		Institutions []string              `json:"institutions"`
	}
	result := make([]userInfo, 0, len(users))
	for _, u := range users {
		roles, _ := h.queries.ListUserRoles(r.Context(), u.ID)
		idents, _ := h.queries.ListUserIdentities(r.Context(), u.ID)
		insts, _ := h.queries.ListUserInstitutionNames(r.Context(), u.ID)
		roleStrs := make([]string, len(roles))
		for i, rl := range roles {
			roleStrs[i] = rl.Role
		}
		if idents == nil {
			idents = []models.UserIdentity{}
		}
		if insts == nil {
			insts = []string{}
		}
		result = append(result, userInfo{User: u, Roles: roleStrs, Identities: idents, Institutions: insts})
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *Handler) CreateUserAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	if req.DisplayName == "" {
		respondError(w, http.StatusBadRequest, "Display name required")
		return
	}
	if !validRoles[req.Role] {
		respondError(w, http.StatusBadRequest, "Invalid role: must be admin, grant_admin, or read_only")
		return
	}

	user := &models.User{DisplayName: req.DisplayName, Status: "active"}
	if err := h.queries.CreateUser(r.Context(), user); err != nil {
		log.Error().Err(err).Msg("Failed to create user")
		respondError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}
	_ = h.queries.AddUserRole(r.Context(), user.ID, req.Role)

	respondJSON(w, http.StatusCreated, user)
}

func (h *Handler) UpdateUserAccount(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	var req struct {
		DisplayName string `json:"display_name"`
		Status      string `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	user, err := h.queries.GetUser(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}
	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Status != "" {
		user.Status = req.Status
	}
	if err := h.queries.UpdateUser(r.Context(), user); err != nil {
		log.Error().Err(err).Msg("Failed to update user")
		respondError(w, http.StatusInternalServerError, "Failed to update user")
		return
	}
	respondJSON(w, http.StatusOK, user)
}

func (h *Handler) DeleteUserAccount(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	_ = h.queries.DeleteUserSessions(r.Context(), userID)
	if err := h.queries.DeleteUser(r.Context(), userID); err != nil {
		log.Error().Err(err).Msg("Failed to delete user")
		respondError(w, http.StatusInternalServerError, "Failed to delete user")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *Handler) AddUserRoleHandler(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	var req struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	if !validRoles[req.Role] {
		respondError(w, http.StatusBadRequest, "Invalid role")
		return
	}
	if err := h.queries.AddUserRole(r.Context(), userID, req.Role); err != nil {
		log.Error().Err(err).Msg("Failed to add role")
		respondError(w, http.StatusInternalServerError, "Failed to add role")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) RemoveUserRoleHandler(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	role := chi.URLParam(r, "role")
	if err := h.queries.RemoveUserRole(r.Context(), userID, role); err != nil {
		log.Error().Err(err).Msg("Failed to remove role")
		respondError(w, http.StatusInternalServerError, "Failed to remove role")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) RemoveUserIdentityHandler(w http.ResponseWriter, r *http.Request) {
	identityID := chi.URLParam(r, "identityID")
	if err := h.queries.DeleteIdentity(r.Context(), identityID); err != nil {
		log.Error().Err(err).Msg("Failed to remove identity")
		respondError(w, http.StatusInternalServerError, "Failed to remove identity")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Invites ---

func (h *Handler) CreateInviteHandler(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")

	user, err := h.queries.GetUser(r.Context(), userID)
	if err != nil || user == nil {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}

	tokenBytes := make([]byte, 32)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	invite := &models.Invite{
		TokenHash: hashToken(token),
		UserID:    userID,
		ExpiresAt: time.Now().Add(inviteExpiry),
	}
	if err := h.queries.CreateInvite(r.Context(), invite); err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to create invite")
		respondError(w, http.StatusInternalServerError, "Failed to create invite")
		return
	}

	inviteURL := fmt.Sprintf("%s/login/invite?token=%s", h.cfg.BaseURL, token)

	respondJSON(w, http.StatusCreated, map[string]any{
		"invite":     invite,
		"invite_url": inviteURL,
	})
}

func (h *Handler) ListInvitesHandler(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	invites, err := h.queries.ListInvites(r.Context(), userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list invites")
		respondError(w, http.StatusInternalServerError, "Failed to list invites")
		return
	}
	if invites == nil {
		invites = []models.Invite{}
	}
	respondJSON(w, http.StatusOK, invites)
}

func (h *Handler) DeleteInviteHandler(w http.ResponseWriter, r *http.Request) {
	inviteID := chi.URLParam(r, "inviteID")
	if err := h.queries.DeleteInvite(r.Context(), inviteID); err != nil {
		log.Error().Err(err).Msg("Failed to delete invite")
		respondError(w, http.StatusInternalServerError, "Failed to delete invite")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

// UpdateMyProfile allows the logged-in user to update their display name.
func (h *Handler) UpdateMyProfile(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	var req struct {
		DisplayName string `json:"display_name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
		_ = h.queries.UpdateUser(r.Context(), user)
	}
	respondJSON(w, http.StatusOK, user)
}

// --- OIDC helper types and functions ---

type oidcWellKnown struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	Issuer                string `json:"issuer"`
}

type oidcTokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func fetchWellKnown(issuer string) (*oidcWellKnown, error) {
	url := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var wk oidcWellKnown
	return &wk, json.Unmarshal(body, &wk)
}

func exchangeCode(tokenURL, code, clientID, clientSecret, redirectURI string) (*oidcTokenResponse, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
	}
	resp, err := http.PostForm(tokenURL, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token exchange failed (HTTP %d): %s", resp.StatusCode, string(body))
	}
	var tr oidcTokenResponse
	return &tr, json.Unmarshal(body, &tr)
}

// parseIDToken does a simplified base64 decode of the JWT payload.
func parseIDToken(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decoding JWT payload: %w", err)
	}
	var claims map[string]any
	return claims, json.Unmarshal(decoded, &claims)
}

// buildOIDCScopes returns the scope string for the OIDC authorization request.
// For CILogon issuers, the org.cilogon.userinfo scope is added.
func buildOIDCScopes(issuer string) string {
	scopes := "openid+email+profile"
	if strings.Contains(issuer, "cilogon.org") {
		scopes += "+org.cilogon.userinfo"
	}
	return scopes
}

// fetchUserinfo calls the OIDC userinfo endpoint with the given access token.
func fetchUserinfo(userinfoURL, accessToken string) (map[string]any, error) {
	req, err := http.NewRequest("GET", userinfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("userinfo request failed (%d): %s", resp.StatusCode, string(body))
	}
	var data map[string]any
	return data, json.Unmarshal(body, &data)
}

// --- User institution access management (admin / grant_admin) ---

func (h *Handler) AddUserInstitutionHandler(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	var req struct {
		Institution string `json:"institution"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Institution == "" {
		respondError(w, http.StatusBadRequest, "Institution name required")
		return
	}
	if err := h.queries.AddUserInstitution(r.Context(), userID, req.Institution); err != nil {
		log.Error().Err(err).Msg("Failed to add institution access")
		respondError(w, http.StatusInternalServerError, "Failed to add institution access")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) RemoveUserInstitutionHandler(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	institution := chi.URLParam(r, "institution")
	if err := h.queries.RemoveUserInstitution(r.Context(), userID, institution); err != nil {
		log.Error().Err(err).Msg("Failed to remove institution access")
		respondError(w, http.StatusInternalServerError, "Failed to remove institution access")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ListUserInstitutionsHandler(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	insts, err := h.queries.ListUserInstitutionNames(r.Context(), userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list institution access")
		respondError(w, http.StatusInternalServerError, "Failed to list institution access")
		return
	}
	if insts == nil {
		insts = []string{}
	}
	respondJSON(w, http.StatusOK, insts)
}
