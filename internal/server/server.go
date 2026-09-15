package server

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	mathrand "math/rand"
	"net/http"
	"sync"
	"time"

	"rishabhdaga.com/codebreaker/internal/game"
	webassets "rishabhdaga.com/codebreaker/web"
)

type Config struct {
	Password      string
	SessionKey    []byte
	SecureCookies bool
	Now           func() time.Time
}

type App struct {
	config Config
	mu     sync.RWMutex
	game   *game.Game
	rng    *mathrand.Rand
	limit  *limiter
	broker *broker
	stop   chan struct{}
}

func New(config Config) *App {
	if config.Now == nil {
		config.Now = time.Now
	}
	var seedBytes [8]byte
	if _, err := rand.Read(seedBytes[:]); err != nil {
		panic(fmt.Sprintf("random seed: %v", err))
	}
	a := &App{
		config: config,
		rng:    mathrand.New(mathrand.NewSource(int64(binary.LittleEndian.Uint64(seedBytes[:])))),
		limit:  newLimiter(), broker: newBroker(), stop: make(chan struct{}),
	}
	go a.expireLoop()
	return a
}

func (a *App) now() time.Time { return a.config.Now() }
func (a *App) Close()         { close(a.stop) }

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", a.page)
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("POST /api/auth/logout", a.logout)
	mux.HandleFunc("GET /api/state", a.requireAccess(a.state))
	mux.HandleFunc("GET /api/events", a.requireAccess(a.events))
	mux.HandleFunc("POST /api/game/create", a.requireAccess(a.create))
	mux.HandleFunc("POST /api/game/join", a.requireAccess(a.join))
	mux.HandleFunc("POST /api/game/cancel", a.requireAccess(a.cancel))
	mux.HandleFunc("POST /api/game/question", a.requireAccess(a.ask))
	mux.HandleFunc("POST /api/game/guess", a.requireAccess(a.guess))
	mux.HandleFunc("POST /api/game/resign", a.requireAccess(a.resign))
	mux.HandleFunc("POST /api/game/close", a.requireAccess(a.closeGame))
	mux.HandleFunc("POST /api/game/rematch", a.requireAccess(a.rematch))
	mux.HandleFunc("PUT /api/notebook", a.requireAccess(a.notebook))
	staticFS, err := fs.Sub(webassets.Files, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (a *App) page(w http.ResponseWriter, r *http.Request) {
	data, err := webassets.Files.ReadFile("templates/index.html")
	if err != nil {
		http.Error(w, "page unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

type nameRequest struct {
	Name string `json:"name"`
}
type loginRequest struct {
	Password string `json:"password"`
}
type questionRequest struct {
	ID     string `json:"id"`
	Choice *int   `json:"choice"`
}
type guessRequest struct {
	Tiles []game.Tile `json:"tiles"`
}
type notebookRequest struct {
	Slots [5]game.NoteSlot `json:"slots"`
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "The request could not be read.")
		return false
	}
	return true
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "Request origin was rejected.")
		return
	}
	ip := clientIP(r)
	if !a.limit.allow(ip, a.now()) {
		writeError(w, http.StatusTooManyRequests, "Too many attempts. Try again later.")
		return
	}
	var request loginRequest
	if !decode(w, r, &request) {
		return
	}
	if !passwordMatches(request.Password, a.config.Password) {
		writeError(w, http.StatusUnauthorized, "That password is not correct.")
		return
	}
	a.limit.clear(ip)
	a.setCookie(w, accessCookie, a.accessValue(a.now().Add(24*time.Hour)), int((24 * time.Hour).Seconds()))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) logout(w http.ResponseWriter, _ *http.Request) {
	a.clearCookie(w, accessCookie)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) create(w http.ResponseWriter, r *http.Request) {
	var request nameRequest
	if !decode(w, r, &request) {
		return
	}
	token, err := game.NewToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not create the game.")
		return
	}
	a.mu.Lock()
	if a.game != nil {
		a.mu.Unlock()
		writeError(w, http.StatusConflict, "A game already exists.")
		return
	}
	a.game, err = game.NewWaiting(request.Name, token, a.now())
	a.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not create the game.")
		return
	}
	a.setCookie(w, seatCookie, token, int((2 * time.Hour).Seconds()))
	a.broker.publish()
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (a *App) join(w http.ResponseWriter, r *http.Request) {
	var request nameRequest
	if !decode(w, r, &request) {
		return
	}
	token, err := game.NewToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not join the game.")
		return
	}
	a.mu.Lock()
	if a.game == nil {
		a.mu.Unlock()
		writeError(w, http.StatusConflict, "There is no game to join.")
		return
	}
	err = a.game.Join(request.Name, token, a.rng, a.now())
	a.mu.Unlock()
	if err != nil {
		writeGameError(w, err)
		return
	}
	a.setCookie(w, seatCookie, token, int((2 * time.Hour).Seconds()))
	a.broker.publish()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) cancel(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	seat := a.seatLocked(r)
	if a.game == nil || a.game.Status != game.Waiting || seat != 0 {
		a.mu.Unlock()
		writeError(w, http.StatusConflict, "Only the waiting player can cancel this game.")
		return
	}
	a.game = nil
	a.mu.Unlock()
	a.clearCookie(w, seatCookie)
	a.broker.publish()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) ask(w http.ResponseWriter, r *http.Request) {
	var request questionRequest
	if !decode(w, r, &request) {
		return
	}
	err := a.mutate(r, func(g *game.Game, seat int) error { return g.Ask(seat, request.ID, request.Choice, a.now()) })
	if err != nil {
		writeGameError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) guess(w http.ResponseWriter, r *http.Request) {
	var request guessRequest
	if !decode(w, r, &request) {
		return
	}
	err := a.mutate(r, func(g *game.Game, seat int) error { return g.Guess(seat, request.Tiles, a.now()) })
	if err != nil {
		writeGameError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) resign(w http.ResponseWriter, r *http.Request) {
	err := a.mutate(r, func(g *game.Game, seat int) error { return g.Resign(seat, a.now()) })
	if err != nil {
		writeGameError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) notebook(w http.ResponseWriter, r *http.Request) {
	var request notebookRequest
	if !decode(w, r, &request) {
		return
	}
	err := a.mutate(r, func(g *game.Game, seat int) error { return g.UpdateNotebook(seat, request.Slots, a.now()) })
	if err != nil {
		writeGameError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) rematch(w http.ResponseWriter, r *http.Request) {
	err := a.mutate(r, func(g *game.Game, seat int) error { _, err := g.RequestRematch(seat, a.rng, a.now()); return err })
	if err != nil {
		writeGameError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) closeGame(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	seat := a.seatLocked(r)
	if a.game == nil || a.game.Status != game.Finished || seat < 0 {
		a.mu.Unlock()
		writeError(w, http.StatusConflict, "The game cannot be closed now.")
		return
	}
	a.game = nil
	a.mu.Unlock()
	a.clearCookie(w, seatCookie)
	a.broker.publish()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) mutate(r *http.Request, change func(*game.Game, int) error) error {
	a.mu.Lock()
	if a.game == nil {
		a.mu.Unlock()
		return game.ErrWrongState
	}
	seat := a.seatLocked(r)
	if seat < 0 {
		a.mu.Unlock()
		return errors.New("this browser does not own a player seat")
	}
	err := change(a.game, seat)
	a.mu.Unlock()
	if err == nil {
		a.broker.publish()
	}
	return err
}

func (a *App) seatLocked(r *http.Request) int {
	if a.game == nil {
		return -1
	}
	cookie, err := r.Cookie(seatCookie)
	if err != nil {
		return -1
	}
	return a.game.SeatForToken(cookie.Value)
}

func writeGameError(w http.ResponseWriter, err error) {
	status := http.StatusUnprocessableEntity
	if errors.Is(err, game.ErrWrongState) || errors.Is(err, game.ErrNotYourTurn) || errors.Is(err, game.ErrQuestion) {
		status = http.StatusConflict
	}
	if err.Error() == "this browser does not own a player seat" {
		status = http.StatusForbidden
	}
	writeError(w, status, err.Error())
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON: %v", err)
	}
}

func (a *App) expireLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.expire()
		case <-a.stop:
			return
		}
	}
}

func (a *App) expire() {
	a.mu.Lock()
	changed := false
	if a.game != nil {
		maxIdle := 60 * time.Minute
		if a.game.Status == game.Waiting || a.game.Status == game.Finished {
			maxIdle = 15 * time.Minute
		}
		if a.now().Sub(a.game.LastActivityAt) >= maxIdle {
			a.game = nil
			changed = true
		}
	}
	a.mu.Unlock()
	if changed {
		a.broker.publish()
	}
}
