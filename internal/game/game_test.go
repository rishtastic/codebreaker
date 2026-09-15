package game

import (
	mathrand "math/rand"
	"reflect"
	"testing"
	"time"
)

func activeGame(t *testing.T) *Game {
	t.Helper()
	g, err := NewWaiting("Ada", "token-a", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Join("Lin", "token-b", mathrand.New(mathrand.NewSource(7)), time.Unix(2, 0)); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestTileSetAndDeal(t *testing.T) {
	tiles := TileSet()
	if len(tiles) != 20 {
		t.Fatalf("got %d tiles", len(tiles))
	}
	g := activeGame(t)
	if len(g.Available) != 4 {
		t.Fatalf("got %d available questions", len(g.Available))
	}
	seen := map[Tile]int{}
	for _, player := range g.Players {
		if len(player.Tiles) != 5 {
			t.Fatal("player did not receive five tiles")
		}
		for i, tile := range player.Tiles {
			seen[tile]++
			if i > 0 && tile.Number < player.Tiles[i-1].Number {
				t.Fatal("tiles not sorted")
			}
			if i > 0 && tile.Number == player.Tiles[i-1].Number && tile.Color == Black {
				t.Fatal("black duplicate must be first")
			}
		}
	}
	for tile, count := range seen {
		limit := 1
		if tile == (Tile{5, Green}) {
			limit = 2
		}
		if count > limit {
			t.Fatalf("physical tile duplicated: %+v", tile)
		}
	}
}

func TestQuestionCatalogue(t *testing.T) {
	questions := AllQuestions()
	if len(questions) != 21 {
		t.Fatalf("got %d questions", len(questions))
	}
	seen := make(map[string]bool, len(questions))
	for _, question := range questions {
		if question.ID == "" || seen[question.ID] {
			t.Fatalf("missing or duplicate question id %q", question.ID)
		}
		seen[question.ID] = true
	}
}

func TestQuestionAnswers(t *testing.T) {
	tiles := []Tile{{1, Black}, {2, White}, {3, White}, {6, Black}, {6, White}}
	cases := map[string]string{
		"same-number": "There is 1 pair.", "black-count": "There are 2 tiles.",
		"even-count": "There are 3 tiles.", "consecutive": "A, B and C.",
		"same-color": "B and C.", "middle-sum": "The sum is 11.",
		"difference": "The difference is 5.", "total-sum": "The sum is 18.",
		"white-sum": "The sum is 11.", "middle-over-4": "No.",
	}
	for _, q := range AllQuestions() {
		want, ok := cases[q.ID]
		if ok && q.Answer(tiles, nil) != want {
			t.Errorf("%s: got %q want %q", q.ID, q.Answer(tiles, nil), want)
		}
	}
}

func TestNotebookIsNeverChangedByQuestion(t *testing.T) {
	g := activeGame(t)
	seat := g.ActivePlayer
	beforeA, beforeB := g.Players[0].Notebook, g.Players[1].Notebook
	q := g.Available[0]
	var choice *int
	if len(q.Choices) > 1 {
		choice = &q.Choices[0]
	}
	if err := g.Ask(seat, q.ID, choice, time.Unix(3, 0)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g.Players[0].Notebook, beforeA) || !reflect.DeepEqual(g.Players[1].Notebook, beforeB) {
		t.Fatal("question mutated a notebook")
	}
}

func TestStartingPlayerGetsTieResponse(t *testing.T) {
	g := activeGame(t)
	starter := g.StartingPlayer
	if err := g.Guess(starter, append([]Tile(nil), g.Players[1-starter].Tiles...), time.Unix(3, 0)); err != nil {
		t.Fatal(err)
	}
	if g.Status != FinalResponse || g.FinalResponder != 1-starter {
		t.Fatal("missing final response")
	}
	if err := g.Guess(1-starter, append([]Tile(nil), g.Players[starter].Tiles...), time.Unix(4, 0)); err != nil {
		t.Fatal(err)
	}
	if g.Status != Finished || !g.Tie {
		t.Fatal("expected tie")
	}
}

func TestWrongGuessRevealsNoDetail(t *testing.T) {
	g := activeGame(t)
	seat := g.ActivePlayer
	wrong := []Tile{{0, Black}, {1, Black}, {2, Black}, {3, Black}, {4, Black}}
	if equalTiles(wrong, g.Players[1-seat].Tiles) {
		wrong[0] = Tile{0, White}
	}
	if err := g.Guess(seat, wrong, time.Unix(3, 0)); err != nil {
		t.Fatal(err)
	}
	want := g.Players[seat].Name + " made an incorrect guess."
	if last := g.History[len(g.History)-1].Message; last != want {
		t.Fatalf("unexpected detail: %q", last)
	}
}
