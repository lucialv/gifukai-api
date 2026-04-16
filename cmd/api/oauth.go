package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lucialv/gifukai-api/pkg/store"
	u "github.com/lucialv/gifukai-api/pkg/utils"
)

const (
	userSessionDuration = 30 * 24 * time.Hour
	oauthStateTTL       = 10 * time.Minute
)

func (s *APIServer) generateUserToken(userID int64) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)
	s.userSessions.Store(token, userSession{
		UserID:    userID,
		ExpiresAt: time.Now().Add(userSessionDuration),
	})
	return token, nil
}

func (s *APIServer) googleOneTapHandler(w http.ResponseWriter, r *http.Request) error {
	if s.Config.GoogleClientID == "" {
		u.WriteError(w, http.StatusNotImplemented, "Google OAuth not configured")
		return nil
	}

	var body struct {
		Credential string `json:"credential"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Credential == "" {
		u.WriteError(w, http.StatusBadRequest, "missing credential")
		return nil
	}

	// Verify the ID token with Google
	resp, err := http.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(body.Credential))
	if err != nil {
		return fmt.Errorf("failed to verify Google token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		u.WriteError(w, http.StatusUnauthorized, "invalid Google token")
		return nil
	}

	var claims struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
		Aud     string `json:"aud"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return fmt.Errorf("failed to decode Google token claims: %w", err)
	}

	// Verify the token was issued for our client ID
	if claims.Aud != s.Config.GoogleClientID {
		u.WriteError(w, http.StatusUnauthorized, "token audience mismatch")
		return nil
	}

	return s.completeOAuthJSON(w, r, "google", claims.Sub, claims.Email, claims.Name, claims.Picture)
}

func (s *APIServer) githubAuthHandler(w http.ResponseWriter, r *http.Request) error {
	if s.Config.GitHubClientID == "" {
		u.WriteError(w, http.StatusNotImplemented, "GitHub OAuth not configured")
		return nil
	}

	state := generateState()
	s.oauthStates.Store(state, time.Now().Add(oauthStateTTL))

	params := url.Values{
		"client_id":    {s.Config.GitHubClientID},
		"redirect_uri": {s.Config.GitHubRedirectURL},
		"scope":        {"read:user user:email"},
		"state":        {state},
	}

	http.Redirect(w, r, "https://github.com/login/oauth/authorize?"+params.Encode(), http.StatusTemporaryRedirect)
	return nil
}

func (s *APIServer) githubCallbackHandler(w http.ResponseWriter, r *http.Request) error {
	if err := s.validateOAuthState(r.URL.Query().Get("state")); err != nil {
		u.WriteError(w, http.StatusBadRequest, err.Error())
		return nil
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		u.WriteError(w, http.StatusBadRequest, "missing code parameter")
		return nil
	}

	// Exchange code for token
	data := url.Values{
		"client_id":     {s.Config.GitHubClientID},
		"client_secret": {s.Config.GitHubClientSecret},
		"code":          {code},
		"redirect_uri":  {s.Config.GitHubRedirectURL},
	}
	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode GitHub token: %w", err)
	}

	// Fetch user info
	req, _ = http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	userResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch GitHub user info: %w", err)
	}
	defer userResp.Body.Close()

	var ghUser struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&ghUser); err != nil {
		return fmt.Errorf("failed to decode GitHub user info: %w", err)
	}

	displayName := ghUser.Name
	if displayName == "" {
		displayName = ghUser.Login
	}

	// If email not public, fetch from emails endpoint
	email := ghUser.Email
	if email == "" {
		email = s.fetchGitHubEmail(tokenResp.AccessToken)
	}

	return s.completeOAuth(w, r, "github", fmt.Sprintf("%d", ghUser.ID), email, displayName, ghUser.AvatarURL)
}

func (s *APIServer) fetchGitHubEmail(accessToken string) string {
	req, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &emails); err != nil {
		return ""
	}
	for _, e := range emails {
		if e.Primary {
			return e.Email
		}
	}
	if len(emails) > 0 {
		return emails[0].Email
	}
	return ""
}

// resolveOAuthUser handles the shared create-or-get user + token flow
// useRedirect = true for browser redirects (GitHub), false for JSON (Google One Tap)
func (s *APIServer) resolveOAuthUser(w http.ResponseWriter, r *http.Request, provider, providerID, email, displayName, avatarURL string, useRedirect bool) (token string, handled bool, err error) {
	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}
	var avatarPtr *string
	if avatarURL != "" {
		avatarPtr = &avatarURL
	}

	user, err := s.Store.CreateOrGetUser(&store.User{
		Provider:    provider,
		ProviderID:  providerID,
		Email:       emailPtr,
		DisplayName: displayName,
		AvatarURL:   avatarPtr,
	})
	if err != nil {
		var mismatch *store.ErrProviderMismatch
		if errors.As(err, &mismatch) {
			if useRedirect {
				frontendURL := strings.TrimRight(s.Config.FrontendURL, "/")
				redirectURL := fmt.Sprintf("%s/gifs?auth_error=provider_mismatch&use_provider=%s",
					frontendURL, url.QueryEscape(mismatch.ExistingProvider))
				http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
			} else {
				u.WriteError(w, http.StatusConflict, fmt.Sprintf("You already have an account with %s. Please sign in with %s to continue.", mismatch.ExistingProvider, mismatch.ExistingProvider))
			}
			return "", true, nil
		}
		return "", false, fmt.Errorf("failed to create/get user: %w", err)
	}

	token, err = s.generateUserToken(user.ID)
	return token, false, err
}

func (s *APIServer) completeOAuthJSON(w http.ResponseWriter, r *http.Request, provider, providerID, email, displayName, avatarURL string) error {
	token, handled, err := s.resolveOAuthUser(w, r, provider, providerID, email, displayName, avatarURL, false)
	if handled || err != nil {
		return err
	}
	return u.WriteJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *APIServer) completeOAuth(w http.ResponseWriter, r *http.Request, provider, providerID, email, displayName, avatarURL string) error {
	token, handled, err := s.resolveOAuthUser(w, r, provider, providerID, email, displayName, avatarURL, true)
	if handled || err != nil {
		return err
	}
	frontendURL := strings.TrimRight(s.Config.FrontendURL, "/")
	redirectURL := fmt.Sprintf("%s/gifs?token=%s", frontendURL, token)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	return nil
}

func (s *APIServer) authMeHandler(w http.ResponseWriter, r *http.Request) error {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		u.WriteError(w, http.StatusUnauthorized, "authentication required")
		return nil
	}

	token := strings.TrimPrefix(auth, "Bearer ")
	val, ok := s.userSessions.Load(token)
	if !ok {
		u.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
		return nil
	}

	sess := val.(userSession)
	if time.Now().After(sess.ExpiresAt) {
		s.userSessions.Delete(token)
		u.WriteError(w, http.StatusUnauthorized, "token expired")
		return nil
	}

	user, err := s.Store.GetUserByID(sess.UserID)
	if err != nil {
		return err
	}
	if user == nil {
		u.WriteError(w, http.StatusNotFound, "user not found")
		return nil
	}

	return u.WriteJSON(w, http.StatusOK, user)
}

func (s *APIServer) authLogoutHandler(w http.ResponseWriter, r *http.Request) error {
	auth := r.Header.Get("Authorization")
	if auth != "" {
		token := strings.TrimPrefix(auth, "Bearer ")
		s.userSessions.Delete(token)
	}
	return u.WriteJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (s *APIServer) validateOAuthState(state string) error {
	if state == "" {
		return fmt.Errorf("missing state parameter")
	}
	val, ok := s.oauthStates.Load(state)
	if !ok {
		return fmt.Errorf("invalid state parameter")
	}
	s.oauthStates.Delete(state)
	expiresAt := val.(time.Time)
	if time.Now().After(expiresAt) {
		return fmt.Errorf("state parameter expired")
	}
	return nil
}

func generateState() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}