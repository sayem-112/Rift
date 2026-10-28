package httpapi

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"rift/server/internal/auth"
	"rift/server/internal/mail"
	"rift/server/internal/metrics"
)

const (
	refreshCookie = "rift_refresh"
	oauthCookie   = "rift_oauth"
	authPath      = "/api/auth"
)

// The refresh token lives only in an HttpOnly cookie, so page scripts (and any
// XSS) can never read it. The short-lived access token is returned in JSON
// and kept in memory by the client.
type sessionResponse struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
	UserID      string    `json:"user_id"`
	DeviceID    string    `json:"device_id"`
}

func (s *Server) startSession(w http.ResponseWriter, sess auth.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookie,
		Value:    sess.RefreshToken,
		Path:     authPath,
		Expires:  sess.RefreshExpiresAt,
		HttpOnly: true,
		Secure:   s.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookie, Value: "", Path: authPath, MaxAge: -1,
		HttpOnly: true, Secure: s.CookieSecure, SameSite: http.SameSiteStrictMode,
	})
}

func writeSession(w http.ResponseWriter, sess auth.Session) {
	writeJSON(w, http.StatusOK, sessionResponse{
		AccessToken: sess.AccessToken,
		ExpiresAt:   sess.AccessExpiresAt,
		UserID:      sess.UserID.String(),
		DeviceID:    sess.DeviceID.String(),
	})
}

// POST /api/auth/email {email}
// Always answers 202 for a well-formed address, whether or not an account
// exists, so the endpoint can't be used to discover who uses rift.
func (s *Server) handleEmailLink(w http.ResponseWriter, r *http.Request) {
	if ok, retry := s.ipSignIn.Allow(s.clientIP(r)); !ok {
		metrics.AuthEvents.WithLabelValues("rate_limited").Inc()
		writeRateLimited(w, retry)
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	email, err := auth.NormalizeEmail(body.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_email", "Enter an email address like name@example.com.")
		return
	}
	if ok, retry := s.emailSignIn.Allow(strings.ToLower(email)); !ok {
		metrics.AuthEvents.WithLabelValues("rate_limited").Inc()
		writeRateLimited(w, retry)
		return
	}

	token, err := s.Auth.CreateMagicLink(r.Context(), email)
	if err != nil {
		s.logger(r).Error("create magic link", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "Couldn't send the link. Try again.")
		return
	}
	// Sent inline rather than through the job queue: a queued job would have
	// to store the raw token, and the database must only ever hold its hash.
	link := s.WebURL + "/auth/verify?token=" + url.QueryEscape(token)
	if err := s.Mailer.Send(r.Context(), mail.SignInLink(email, link)); err != nil {
		s.logger(r).Error("send magic link", "err", err)
		writeError(w, http.StatusBadGateway, "mail_failed", "Couldn't send the email. Try again in a minute.")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// POST /api/auth/email/verify {token, device}
func (s *Server) handleEmailVerify(w http.ResponseWriter, r *http.Request) {
	if ok, retry := s.ipSignIn.Allow(s.clientIP(r)); !ok {
		writeRateLimited(w, retry)
		return
	}
	var body struct {
		Token  string          `json:"token"`
		Device auth.DeviceInfo `json:"device"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	sess, err := s.Auth.RedeemMagicLink(r.Context(), body.Token, body.Device)
	switch {
	case errors.Is(err, auth.ErrLinkInvalid):
		writeError(w, http.StatusUnauthorized, "link_invalid", "This sign-in link was already used or has expired. Request a new one.")
		return
	case errors.Is(err, auth.ErrInvalidDevice):
		writeError(w, http.StatusBadRequest, "invalid_device", "The device details are missing or too long.")
		return
	case err != nil:
		s.logger(r).Error("redeem magic link", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "Couldn't sign you in. Try again.")
		return
	}
	s.startSession(w, sess)
	writeSession(w, sess)
}

// The OAuth round trip carries its secrets in a short-lived HttpOnly cookie
// scoped to the Google endpoints: the state (CSRF protection), the PKCE
// verifier, and the device details to register once Google says yes.
type oauthState struct {
	State    string          `json:"s"`
	Verifier string          `json:"v"`
	Device   auth.DeviceInfo `json:"d"`
}

// GET /api/auth/google/start?name=&kind=&client=
func (s *Server) handleGoogleStart(w http.ResponseWriter, r *http.Request) {
	if s.Google == nil {
		writeError(w, http.StatusNotFound, "google_disabled", "Google sign-in isn't set up on this server.")
		return
	}
	q := r.URL.Query()
	st := oauthState{
		State:    auth.NewOpaqueToken(),
		Verifier: auth.NewOpaqueToken() + auth.NewOpaqueToken(), // 86 chars; PKCE allows 43-128
		Device:   auth.DeviceInfo{Name: q.Get("name"), Kind: q.Get("kind"), Client: q.Get("client")},
	}
	raw, _ := json.Marshal(st)
	http.SetCookie(w, &http.Cookie{
		Name:     oauthCookie,
		Value:    base64.RawURLEncoding.EncodeToString(raw),
		Path:     authPath + "/google",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   s.CookieSecure,
		// Lax, not Strict: the callback is a top-level redirect from Google.
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, s.Google.AuthCodeURL(st.State, st.Verifier), http.StatusFound)
}

// GET /api/auth/google/callback?code=&state=
func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	fail := func(reason string) {
		http.Redirect(w, r, s.WebURL+"/signin?error="+url.QueryEscape(reason), http.StatusFound)
	}
	if s.Google == nil {
		fail("google_disabled")
		return
	}
	cookie, err := r.Cookie(oauthCookie)
	http.SetCookie(w, &http.Cookie{Name: oauthCookie, Path: authPath + "/google", MaxAge: -1, HttpOnly: true, Secure: s.CookieSecure})
	if err != nil {
		fail("expired")
		return
	}
	var st oauthState
	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil || json.Unmarshal(raw, &st) != nil {
		fail("expired")
		return
	}
	q := r.URL.Query()
	if q.Get("error") != "" {
		fail("cancelled")
		return
	}
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(st.State)) != 1 {
		fail("state_mismatch")
		return
	}

	identity, err := s.Google.Exchange(r.Context(), q.Get("code"), st.Verifier)
	if err != nil {
		s.logger(r).Warn("google exchange", "err", err)
		fail("google")
		return
	}
	sess, err := s.Auth.SignInWithGoogle(r.Context(), identity, st.Device)
	switch {
	case errors.Is(err, auth.ErrUnverifiedEmail):
		fail("unverified_email")
		return
	case errors.Is(err, auth.ErrInvalidDevice):
		fail("invalid_device")
		return
	case err != nil:
		s.logger(r).Error("google sign-in", "err", err)
		fail("internal")
		return
	}
	s.startSession(w, sess)
	// The web app calls /api/auth/refresh on load to get its access token.
	http.Redirect(w, r, s.WebURL+"/", http.StatusFound)
}

// POST /api/auth/refresh  (refresh cookie)
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if !s.sameOrigin(r) {
		writeError(w, http.StatusForbidden, "bad_origin", "Request origin not allowed.")
		return
	}
	if ok, retry := s.ipRefresh.Allow(s.clientIP(r)); !ok {
		writeRateLimited(w, retry)
		return
	}
	cookie, err := r.Cookie(refreshCookie)
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "Sign in to continue.")
		return
	}
	sess, err := s.Auth.Refresh(r.Context(), cookie.Value)
	switch {
	case errors.Is(err, auth.ErrRefreshReplayed):
		s.logger(r).Warn("refresh token replay detected; family revoked")
		s.clearSession(w)
		writeError(w, http.StatusUnauthorized, "session_revoked", "For your security this device was signed out. Sign in again.")
		return
	case errors.Is(err, auth.ErrRefreshInvalid):
		s.clearSession(w)
		writeError(w, http.StatusUnauthorized, "unauthenticated", "Your session expired. Sign in again.")
		return
	case err != nil:
		s.logger(r).Error("refresh", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "Something went wrong. Try again.")
		return
	}
	s.startSession(w, sess)
	writeSession(w, sess)
}

// POST /api/auth/logout  (refresh cookie)
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !s.sameOrigin(r) {
		writeError(w, http.StatusForbidden, "bad_origin", "Request origin not allowed.")
		return
	}
	if cookie, err := r.Cookie(refreshCookie); err == nil && cookie.Value != "" {
		if err := s.Auth.SignOut(r.Context(), cookie.Value); err != nil {
			s.logger(r).Error("sign out", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "Couldn't sign out. Try again.")
			return
		}
	}
	s.clearSession(w)
	w.WriteHeader(http.StatusNoContent)
}
