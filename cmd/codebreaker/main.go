package main

import (
	"html/template"
	"log"
	"net/http"
	"time"
)

var gamePage = template.Must(template.New("game").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Codebreaker</title>
  <style>
    :root { color-scheme: dark; font-family: Inter, ui-sans-serif, system-ui, sans-serif; background: #10121a; color: #f4f6fb; }
    body { display: grid; min-height: 100vh; place-items: center; margin: 0; padding: 24px; text-align: center; }
    main { max-width: 640px; }
    p { color: #b9c1d1; font-size: 1.1rem; line-height: 1.65; }
    .eyebrow { color: #8ca2ff; font-size: .8rem; font-weight: 700; letter-spacing: .14em; text-transform: uppercase; }
    h1 { font-size: clamp(3rem, 12vw, 7rem); letter-spacing: -.08em; margin: 12px 0; }
  </style>
</head>
<body>
  <main>
    <div class="eyebrow">In development</div>
    <h1>Codebreaker</h1>
    <p>The game is being built.</p>
  </main>
</body>
</html>`))

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", game)
	mux.HandleFunc("/healthz", health)

	server := &http.Server{
		Addr:              "127.0.0.1:8081",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func game(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := gamePage.Execute(w, nil); err != nil {
		log.Printf("render game: %v", err)
	}
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}
