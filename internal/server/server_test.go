package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testApp(t *testing.T) (*App, *httptest.Server) {
	t.Helper()
	a := New(Config{Password: "open-sesame", SessionKey: []byte("a-test-key-long-enough-for-hmac"), Now: time.Now})
	s := httptest.NewServer(a.Handler())
	t.Cleanup(func() { s.Close(); a.Close() })
	return a, s
}

func client(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

func post(t *testing.T, c *http.Client, url, path string, body any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	response, err := c.Post(url+path, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func login(t *testing.T, c *http.Client, url string) {
	t.Helper()
	response := post(t, c, url, "/api/auth/login", map[string]string{"password": "open-sesame"})
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login: %s", response.Status)
	}
}

func TestPasswordRequired(t *testing.T) {
	_, s := testApp(t)
	response, err := http.Get(s.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("got %s", response.Status)
	}
}

func TestSingleGameAndPrivateViews(t *testing.T) {
	app, s := testApp(t)
	first, second, third := client(t), client(t), client(t)
	login(t, first, s.URL)
	login(t, second, s.URL)
	login(t, third, s.URL)
	response := post(t, first, s.URL, "/api/game/create", map[string]string{"name": "Ada"})
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create: %s", response.Status)
	}
	response = post(t, second, s.URL, "/api/game/join", map[string]string{"name": "Lin"})
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("join: %s", response.Status)
	}
	response = post(t, third, s.URL, "/api/game/join", map[string]string{"name": "Eve"})
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("third join: %s", response.Status)
	}

	app.mu.RLock()
	secret := app.game.Players[1].Tiles
	app.mu.RUnlock()
	stateResponse, err := first.Get(s.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(stateResponse.Body)
	stateResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	secretJSON, _ := json.Marshal(secret)
	if strings.Contains(string(body), string(secretJSON)) {
		t.Fatal("opponent tiles leaked in player view")
	}
	var view stateView
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatal(err)
	}
	if len(view.OwnTiles) != 5 || len(view.Available) != 4 {
		t.Fatal("incomplete player view")
	}
	if len(view.OpponentTiles) != 0 {
		t.Fatal("opponent tiles were included before the game ended")
	}
	if view.Notebook == nil {
		t.Fatal("missing private notebook")
	}
}
