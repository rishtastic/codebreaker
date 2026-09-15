package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"rishabhdaga.com/codebreaker/internal/game"
)

type playerSummary struct {
	Name string `json:"name"`
	Seat int    `json:"seat"`
}

type stateView struct {
	Lobby            string            `json:"lobby"`
	Status           game.Status       `json:"status,omitempty"`
	Version          uint64            `json:"version,omitempty"`
	Seat             int               `json:"seat"`
	IsPlayer         bool              `json:"isPlayer"`
	CanCreate        bool              `json:"canCreate"`
	CanJoin          bool              `json:"canJoin"`
	CanAct           bool              `json:"canAct"`
	CanCancel        bool              `json:"canCancel"`
	CanClose         bool              `json:"canClose"`
	CanRematch       bool              `json:"canRematch"`
	RematchRequested bool              `json:"rematchRequested"`
	Turn             int               `json:"turn,omitempty"`
	ActivePlayer     int               `json:"activePlayer,omitempty"`
	StartingPlayer   int               `json:"startingPlayer,omitempty"`
	Players          []playerSummary   `json:"players,omitempty"`
	OwnTiles         []game.Tile       `json:"ownTiles,omitempty"`
	OpponentTiles    []game.Tile       `json:"opponentTiles,omitempty"`
	Available        []game.Question   `json:"available,omitempty"`
	History          []game.Event      `json:"history,omitempty"`
	Notebook         *[5]game.NoteSlot `json:"notebook,omitempty"`
	Winner           *int              `json:"winner,omitempty"`
	Tie              bool              `json:"tie,omitempty"`
}

func (a *App) state(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, a.view(r)) }

func (a *App) view(r *http.Request) stateView {
	a.mu.RLock()
	defer a.mu.RUnlock()
	view := stateView{Lobby: "empty", Seat: -1, CanCreate: a.game == nil}
	if a.game == nil {
		return view
	}
	g, seat := a.game, a.seatLocked(r)
	view.Lobby, view.Status, view.Version, view.Seat = "full", g.Status, g.Version, seat
	view.IsPlayer, view.Turn, view.ActivePlayer, view.StartingPlayer = seat >= 0, g.Turn, g.ActivePlayer, g.StartingPlayer
	view.History = append([]game.Event(nil), g.History...)
	for index, player := range g.Players {
		if player != nil {
			view.Players = append(view.Players, playerSummary{Name: player.Name, Seat: index})
		}
	}
	if g.Status == game.Waiting {
		view.Lobby, view.CanJoin, view.CanCancel = "waiting", seat < 0, seat == 0
	}
	if seat < 0 {
		return view
	}
	view.OwnTiles = append([]game.Tile(nil), g.Players[seat].Tiles...)
	notebook := g.Players[seat].Notebook
	view.Notebook = &notebook
	view.Available = append([]game.Question(nil), g.Available...)
	view.CanAct = (g.Status == game.Active && g.ActivePlayer == seat) || (g.Status == game.FinalResponse && g.FinalResponder == seat)
	if g.Status == game.Finished {
		view.OpponentTiles = append([]game.Tile(nil), g.Players[1-seat].Tiles...)
		view.CanClose, view.CanRematch, view.RematchRequested, view.Tie = true, true, g.RematchRequests[seat], g.Tie
		if g.Winner >= 0 {
			winner := g.Winner
			view.Winner = &winner
		}
	}
	return view
}

func (a *App) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "Live updates are unavailable.")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	updates := a.broker.subscribe()
	defer a.broker.unsubscribe(updates)
	send := func() bool {
		data, err := json.Marshal(a.view(r))
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "event: state\ndata: %s\n\n", data); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !send() {
		return
	}
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-updates:
			if !send() {
				return
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
