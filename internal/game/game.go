package game

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	mathrand "math/rand"
	"sort"
	"strings"
	"time"
)

type Color string

const (
	Black Color = "black"
	White Color = "white"
	Green Color = "green"
)

type Tile struct {
	Number int   `json:"number"`
	Color  Color `json:"color"`
}

type Status string

const (
	Waiting       Status = "waiting"
	Active        Status = "active"
	FinalResponse Status = "final_response"
	Finished      Status = "finished"
)

type Event struct {
	Turn    int    `json:"turn"`
	Message string `json:"message"`
	Kind    string `json:"kind"`
}

type NoteSlot struct {
	Numbers []int   `json:"numbers"`
	Colors  []Color `json:"colors"`
	Text    string  `json:"text"`
}

type Player struct {
	ID       string
	Token    string
	Name     string
	Tiles    []Tile
	Notebook [5]NoteSlot
}

type Game struct {
	ID              string
	Status          Status
	Players         [2]*Player
	StartingPlayer  int
	ActivePlayer    int
	Questions       []Question
	Available       []Question
	History         []Event
	Turn            int
	FinalResponder  int
	Winner          int
	Tie             bool
	RematchRequests [2]bool
	Version         uint64
	CreatedAt       time.Time
	LastActivityAt  time.Time
}

var (
	ErrWrongState     = errors.New("that action is not available now")
	ErrNotYourTurn    = errors.New("it is not your turn")
	ErrQuestion       = errors.New("that question is not available")
	ErrQuestionChoice = errors.New("choose one of the numbers on the card")
	ErrGuess          = errors.New("enter five valid tiles")
)

func NewWaiting(name, token string, now time.Time) (*Game, error) {
	id, err := randomID(12)
	if err != nil {
		return nil, err
	}
	playerID, err := randomID(12)
	if err != nil {
		return nil, err
	}
	return &Game{
		ID: id, Status: Waiting, Winner: -1, FinalResponder: -1,
		Players: [2]*Player{{ID: playerID, Token: token, Name: cleanName(name, "Player 1")}, nil},
		Version: 1, CreatedAt: now, LastActivityAt: now,
	}, nil
}

func (g *Game) Join(name, token string, rng *mathrand.Rand, now time.Time) error {
	if g.Status != Waiting || g.Players[1] != nil {
		return ErrWrongState
	}
	playerID, err := randomID(12)
	if err != nil {
		return err
	}
	g.Players[1] = &Player{ID: playerID, Token: token, Name: cleanName(name, "Player 2")}
	g.deal(rng)
	g.Status = Active
	g.History = []Event{{Turn: 0, Kind: "start", Message: fmt.Sprintf("%s goes first.", g.Players[g.StartingPlayer].Name)}}
	g.bump(now)
	return nil
}

func (g *Game) deal(rng *mathrand.Rand) {
	tiles := TileSet()
	rng.Shuffle(len(tiles), func(i, j int) { tiles[i], tiles[j] = tiles[j], tiles[i] })
	for seat := range 2 {
		g.Players[seat].Tiles = append([]Tile(nil), tiles[seat*5:seat*5+5]...)
		sortTiles(g.Players[seat].Tiles)
		g.Players[seat].Notebook = [5]NoteSlot{}
	}
	questions := append([]Question(nil), AllQuestions()...)
	rng.Shuffle(len(questions), func(i, j int) { questions[i], questions[j] = questions[j], questions[i] })
	g.Available = append([]Question(nil), questions[:4]...)
	g.Questions = append([]Question(nil), questions[4:]...)
	g.StartingPlayer = rng.Intn(2)
	g.ActivePlayer = g.StartingPlayer
	g.FinalResponder = -1
	g.Winner = -1
	g.Tie = false
	g.Turn = 1
	g.RematchRequests = [2]bool{}
}

func TileSet() []Tile {
	tiles := make([]Tile, 0, 20)
	for number := 0; number <= 9; number++ {
		if number == 5 {
			tiles = append(tiles, Tile{number, Green}, Tile{number, Green})
			continue
		}
		tiles = append(tiles, Tile{number, Black}, Tile{number, White})
	}
	return tiles
}

func sortTiles(tiles []Tile) {
	sort.Slice(tiles, func(i, j int) bool {
		if tiles[i].Number != tiles[j].Number {
			return tiles[i].Number < tiles[j].Number
		}
		return colorOrder(tiles[i].Color) < colorOrder(tiles[j].Color)
	})
}

func colorOrder(color Color) int {
	switch color {
	case Black:
		return 0
	case Green:
		return 1
	default:
		return 2
	}
}

func (g *Game) Ask(seat int, questionID string, choice *int, now time.Time) error {
	if g.Status != Active {
		return ErrWrongState
	}
	if seat != g.ActivePlayer {
		return ErrNotYourTurn
	}
	index := -1
	for i := range g.Available {
		if g.Available[i].ID == questionID {
			index = i
			break
		}
	}
	if index < 0 {
		return ErrQuestion
	}
	question := g.Available[index]
	if !question.Accepts(choice) {
		return ErrQuestionChoice
	}
	answer := question.Answer(g.Players[1-seat].Tiles, choice)
	prompt := question.Prompt
	if choice != nil {
		prompt = strings.ReplaceAll(prompt, "{number}", fmt.Sprint(*choice))
	}
	g.History = append(g.History, Event{Turn: g.Turn, Kind: "question", Message: fmt.Sprintf("%s asked: %s — %s", g.Players[seat].Name, prompt, answer)})
	g.Available = append(g.Available[:index], g.Available[index+1:]...)
	if len(g.Questions) > 0 {
		g.Available = append(g.Available, g.Questions[0])
		g.Questions = g.Questions[1:]
	}
	if len(g.Available) == 0 {
		g.Status = Finished
		g.History = append(g.History, Event{Turn: g.Turn, Kind: "finish", Message: "All questions have been used. The game ends without a winner."})
	} else {
		g.nextTurn()
	}
	g.bump(now)
	return nil
}

func (g *Game) Guess(seat int, guess []Tile, now time.Time) error {
	if g.Status == FinalResponse {
		if seat != g.FinalResponder {
			return ErrNotYourTurn
		}
	} else if g.Status != Active {
		return ErrWrongState
	} else if seat != g.ActivePlayer {
		return ErrNotYourTurn
	}
	if !validGuess(guess) {
		return ErrGuess
	}
	correct := equalTiles(guess, g.Players[1-seat].Tiles)
	if g.Status == FinalResponse {
		g.Status = Finished
		if correct {
			g.Tie = true
			g.Winner = -1
			g.History = append(g.History, Event{Turn: g.Turn, Kind: "tie", Message: fmt.Sprintf("%s also broke the code. The game is a tie.", g.Players[seat].Name)})
		} else {
			g.History = append(g.History, Event{Turn: g.Turn, Kind: "finish", Message: fmt.Sprintf("%s's final response was incorrect. %s wins.", g.Players[seat].Name, g.Players[g.Winner].Name)})
		}
		g.bump(now)
		return nil
	}
	if !correct {
		g.History = append(g.History, Event{Turn: g.Turn, Kind: "guess", Message: fmt.Sprintf("%s made an incorrect guess.", g.Players[seat].Name)})
		g.nextTurn()
		g.bump(now)
		return nil
	}
	if seat == g.StartingPlayer {
		g.Status = FinalResponse
		g.Winner = seat
		g.FinalResponder = 1 - seat
		g.ActivePlayer = 1 - seat
		g.History = append(g.History, Event{Turn: g.Turn, Kind: "final_response", Message: fmt.Sprintf("%s broke the code. %s gets one final response.", g.Players[seat].Name, g.Players[1-seat].Name)})
	} else {
		g.Status = Finished
		g.Winner = seat
		g.History = append(g.History, Event{Turn: g.Turn, Kind: "finish", Message: fmt.Sprintf("%s broke the code and wins.", g.Players[seat].Name)})
	}
	g.bump(now)
	return nil
}

func (g *Game) Resign(seat int, now time.Time) error {
	if g.Status != Active && g.Status != FinalResponse {
		return ErrWrongState
	}
	g.Status = Finished
	g.Winner = 1 - seat
	g.Tie = false
	g.History = append(g.History, Event{Turn: g.Turn, Kind: "finish", Message: fmt.Sprintf("%s resigned. %s wins.", g.Players[seat].Name, g.Players[1-seat].Name)})
	g.bump(now)
	return nil
}

func (g *Game) RequestRematch(seat int, rng *mathrand.Rand, now time.Time) (bool, error) {
	if g.Status != Finished {
		return false, ErrWrongState
	}
	g.RematchRequests[seat] = true
	if !g.RematchRequests[0] || !g.RematchRequests[1] {
		g.bump(now)
		return false, nil
	}
	g.deal(rng)
	g.Status = Active
	g.History = []Event{{Turn: 0, Kind: "start", Message: fmt.Sprintf("Rematch started. %s goes first.", g.Players[g.StartingPlayer].Name)}}
	g.bump(now)
	return true, nil
}

func (g *Game) UpdateNotebook(seat int, notebook [5]NoteSlot, now time.Time) error {
	if seat < 0 || seat > 1 || g.Players[seat] == nil {
		return ErrWrongState
	}
	for _, slot := range notebook {
		if len(slot.Text) > 120 || len(slot.Numbers) > 10 || len(slot.Colors) > 3 {
			return errors.New("notebook entry is too large")
		}
		for _, number := range slot.Numbers {
			if number < 0 || number > 9 {
				return errors.New("notebook contains an invalid number")
			}
		}
		for _, color := range slot.Colors {
			if color != Black && color != White && color != Green {
				return errors.New("notebook contains an invalid color")
			}
		}
	}
	g.Players[seat].Notebook = notebook
	g.bump(now)
	return nil
}

func (g *Game) SeatForToken(token string) int {
	for seat, player := range g.Players {
		if player != nil && player.Token == token {
			return seat
		}
	}
	return -1
}

func (g *Game) nextTurn()          { g.ActivePlayer = 1 - g.ActivePlayer; g.Turn++ }
func (g *Game) bump(now time.Time) { g.Version++; g.LastActivityAt = now }

func validGuess(guess []Tile) bool {
	if len(guess) != 5 {
		return false
	}
	for _, tile := range guess {
		if tile.Number < 0 || tile.Number > 9 {
			return false
		}
		if tile.Number == 5 && tile.Color != Green {
			return false
		}
		if tile.Number != 5 && tile.Color != Black && tile.Color != White {
			return false
		}
	}
	return true
}

func equalTiles(a, b []Tile) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func cleanName(name, fallback string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return fallback
	}
	runes := []rune(name)
	if len(runes) > 24 {
		runes = runes[:24]
	}
	return string(runes)
}

func randomID(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func NewToken() (string, error) { return randomID(32) }
