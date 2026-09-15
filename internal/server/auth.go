package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	accessCookie = "codebreaker_access"
	seatCookie   = "codebreaker_seat"
)

type attempt struct {
	count int
	since time.Time
}

type limiter struct {
	mu       sync.Mutex
	attempts map[string]attempt
}

func newLimiter() *limiter { return &limiter{attempts: make(map[string]attempt)} }

func (l *limiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	value := l.attempts[key]
	if now.Sub(value.since) >= 10*time.Minute {
		value = attempt{since: now}
	}
	if value.since.IsZero() {
		value.since = now
	}
	if value.count >= 8 {
		return false
	}
	value.count++
	l.attempts[key] = value
	return true
}

func (l *limiter) clear(key string) {
	l.mu.Lock()
	delete(l.attempts, key)
	l.mu.Unlock()
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func passwordMatches(got, want string) bool {
	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}

func (a *App) accessValue(expiry time.Time) string {
	payload := strconv.FormatInt(expiry.Unix(), 10)
	mac := hmac.New(sha256.New, a.config.SessionKey)
	_, _ = mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

func (a *App) validAccess(r *http.Request) bool {
	cookie, err := r.Cookie(accessCookie)
	if err != nil {
		return false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return false
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || a.now().After(time.Unix(expires, 0)) {
		return false
	}
	want := a.accessValue(time.Unix(expires, 0))
	return hmac.Equal([]byte(cookie.Value), []byte(want))
}

func (a *App) setCookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: a.config.SecureCookies, SameSite: http.SameSiteStrictMode,
	})
}

func (a *App) clearCookie(w http.ResponseWriter, name string) { a.setCookie(w, name, "", -1) }

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	wantHTTP := "http://" + r.Host
	wantHTTPS := "https://" + r.Host
	return subtle.ConstantTimeCompare([]byte(origin), []byte(wantHTTP)) == 1 ||
		subtle.ConstantTimeCompare([]byte(origin), []byte(wantHTTPS)) == 1
}

func (a *App) requireAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.validAccess(r) {
			writeError(w, http.StatusUnauthorized, "Enter the game password to continue.")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !sameOrigin(r) {
			writeError(w, http.StatusForbidden, "Request origin was rejected.")
			return
		}
		next(w, r)
	}
}

func method(w http.ResponseWriter, r *http.Request, allowed string) bool {
	if r.Method == allowed {
		return true
	}
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, fmt.Sprintf("Use %s for this action.", allowed))
	return false
}
