package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/bbockelm/fabaid-manager/internal/models"
)

// API key format:  fabaid_<4 public chars>_<28 random base64url chars>
// Total ≈ 40 chars.  The prefix "fabaid_" allows easy scanning in repos.
const (
	apiKeyTokenPrefix = "fabaid_"
	apiKeyPublicLen   = 4
	apiKeyRandomLen   = 28 // base64url chars of randomness after the public part
)

// generateAPIKey returns (rawKey, keyPrefix) where keyPrefix = "fabaid_" + first 4 random chars.
func generateAPIKey() (string, string, error) {
	// base64url: 4 chars per 3 bytes → need ceil((4+28)*3/4) = 24 bytes.
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(buf) // 32 chars
	publicPart := encoded[:apiKeyPublicLen]
	secretPart := encoded[apiKeyPublicLen : apiKeyPublicLen+apiKeyRandomLen]
	rawKey := apiKeyTokenPrefix + publicPart + "_" + secretPart
	keyPrefix := apiKeyTokenPrefix + publicPart
	return rawKey, keyPrefix, nil
}

type createAPIKeyRequest struct {
	Name         string   `json:"name"`
	Roles        []string `json:"roles"`
	IdleTimeoutS *int     `json:"idle_timeout_s,omitempty"` // seconds; nil = no idle timeout
	ExpiresAt    *string  `json:"expires_at,omitempty"`     // RFC 3339; nil = no hard expiry
}

// CreateAPIKey generates a new API key and returns it once.
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req createAPIKeyRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	for _, role := range req.Roles {
		if !validRoles[role] {
			respondError(w, http.StatusBadRequest, "invalid role: "+role)
			return
		}
	}
	if len(req.Roles) == 0 {
		respondError(w, http.StatusBadRequest, "at least one role is required")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			respondError(w, http.StatusBadRequest, "expires_at must be RFC 3339")
			return
		}
		expiresAt = &t
	}

	rawKey, keyPrefix, err := generateAPIKey()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate key")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to hash key")
		return
	}

	k := &models.APIKey{
		Name:         req.Name,
		KeyHash:      string(hash),
		KeyPrefix:    keyPrefix,
		Roles:        req.Roles,
		CreatedBy:    user.ID,
		IdleTimeoutS: req.IdleTimeoutS,
		ExpiresAt:    expiresAt,
	}
	if err := h.queries.CreateAPIKey(r.Context(), k); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save key")
		return
	}

	k.CreatedByName = user.DisplayName

	resp := models.APIKeyCreateResponse{
		APIKey: *k,
		RawKey: rawKey,
	}
	respondJSON(w, http.StatusCreated, resp)
}

// ListAPIKeys returns all API keys (without hashes).
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.queries.ListAPIKeys(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list API keys")
		return
	}
	if keys == nil {
		keys = []models.APIKey{}
	}
	respondJSON(w, http.StatusOK, keys)
}

// RevokeAPIKey soft-revokes a key (sets revoked_at).
func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	keyID := chi.URLParam(r, "keyID")
	if err := h.queries.RevokeAPIKey(r.Context(), keyID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to revoke key")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// DeleteAPIKey permanently deletes a key.
func (h *Handler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	keyID := chi.URLParam(r, "keyID")
	if err := h.queries.DeleteAPIKey(r.Context(), keyID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- API Key Authentication Middleware ---

const apiKeyContextKey contextKey = "api_key"

// GetAPIKeyFromContext returns the API key if the request was authenticated via API key.
func GetAPIKeyFromContext(ctx context.Context) *models.APIKey {
	k, _ := ctx.Value(apiKeyContextKey).(*models.APIKey)
	return k
}

// RequireAuthOrAPIKey is like RequireAuth but also accepts Bearer token API keys.
// If a valid API key is found, it synthesizes a Session and User in context so
// downstream middleware (RequireRole, RequireWriteAccess) works unchanged.
func (h *Handler) RequireAuthOrAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try cookie-based session first.
		cookie, err := r.Cookie(sessionCookieName)
		if err == nil && cookie.Value != "" {
			session, sErr := h.queries.GetSession(r.Context(), cookie.Value)
			if sErr == nil {
				user, uErr := h.queries.GetUser(r.Context(), session.UserID)
				if uErr == nil && user.Status == "active" {
					ctx := r.Context()
					ctx = context.WithValue(ctx, sessionContextKey, session)
					ctx = context.WithValue(ctx, userContextKey, user)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
		}

		// Try Bearer token (API key).
		auth := r.Header.Get("Authorization")
		if len(auth) > 7 && auth[:7] == "Bearer " {
			rawKey := auth[7:]
			if apiKey := h.authenticateAPIKey(r, rawKey); apiKey != nil {
				effectiveRole := pickHighestRole(apiKey.Roles)
				syntheticSession := &models.Session{
					ID:     "apikey:" + apiKey.ID,
					UserID: apiKey.CreatedBy,
					Role:   effectiveRole,
				}
				user, uErr := h.queries.GetUser(r.Context(), apiKey.CreatedBy)
				if uErr != nil || user.Status != "active" {
					respondError(w, http.StatusUnauthorized, "API key owner not active")
					return
				}
				ctx := r.Context()
				ctx = context.WithValue(ctx, sessionContextKey, syntheticSession)
				ctx = context.WithValue(ctx, userContextKey, user)
				ctx = context.WithValue(ctx, apiKeyContextKey, apiKey)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		respondError(w, http.StatusUnauthorized, "Not authenticated")
	})
}

// authenticateAPIKey checks the provided raw key against active keys.
func (h *Handler) authenticateAPIKey(r *http.Request, rawKey string) *models.APIKey {
	if len(rawKey) < len(apiKeyTokenPrefix)+apiKeyPublicLen+1 {
		return nil
	}
	if rawKey[:len(apiKeyTokenPrefix)] != apiKeyTokenPrefix {
		return nil
	}

	keys, err := h.queries.ListActiveAPIKeyHashes(r.Context())
	if err != nil {
		return nil
	}

	presentedPrefix := rawKey[:len(apiKeyTokenPrefix)+apiKeyPublicLen]

	for i := range keys {
		k := &keys[i]
		if k.KeyPrefix != presentedPrefix {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(k.KeyHash), []byte(rawKey)) != nil {
			continue
		}
		// Check idle timeout
		if k.IdleTimeoutS != nil && k.LastUsedAt != nil {
			idleDeadline := k.LastUsedAt.Add(time.Duration(*k.IdleTimeoutS) * time.Second)
			if time.Now().After(idleDeadline) {
				return nil
			}
		}
		// Debounced last_used_at update (1 minute)
		if k.LastUsedAt == nil || time.Since(*k.LastUsedAt) > time.Minute {
			_ = h.queries.TouchAPIKeyLastUsed(r.Context(), k.ID)
		}
		return k
	}
	return nil
}

func pickHighestRole(roles []string) string {
	roleOrder := map[string]int{RoleAdmin: 3, RoleGrantAdmin: 2, RoleReadOnly: 1}
	best := ""
	bestPri := 0
	for _, r := range roles {
		if p, ok := roleOrder[r]; ok && p > bestPri {
			bestPri = p
			best = r
		}
	}
	if best == "" && len(roles) > 0 {
		best = roles[0]
	}
	return best
}
