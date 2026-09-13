package main

import (
	"html/template"
	"log"
	"net/http"
	"time"
)

var pages = template.Must(template.New("pages").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    :root { color-scheme: dark; font-family: Inter, ui-sans-serif, system-ui, sans-serif; background: #10121a; color: #f4f6fb; }
    body { max-width: 900px; margin: 0 auto; padding: 72px 24px; }
    h1 { font-size: clamp(2.4rem, 7vw, 4.8rem); letter-spacing: -0.07em; margin: 0 0 14px; }
    p { color: #b9c1d1; font-size: 1.1rem; line-height: 1.65; }
    .eyebrow { color: #8ca2ff; font-size: .8rem; font-weight: 700; letter-spacing: .14em; text-transform: uppercase; }
    .card { display: block; max-width: 520px; margin-top: 44px; padding: 30px; border: 1px solid #303a58; border-radius: 18px; background: #181c29; color: inherit; text-decoration: none; transition: border-color .2s, transform .2s; }
    .card:hover { border-color: #8ca2ff; transform: translateY(-3px); }
    .card h2 { margin: 0 0 8px; font-size: 1.5rem; }
    .tag { display: inline-block; margin-top: 8px; padding: 5px 9px; border-radius: 999px; background: #263461; color: #bfd0ff; font-size: .8rem; }
    a { color: #aebeff; }
  </style>
</head>
<body>
  {{if .Game}}
    <div class="eyebrow">Project · Coming soon</div>
    <h1>Codebreaker</h1>
    <p>The game is being built. This route is ready for the playable experience.</p>
    <p><a href="/">← View all projects</a></p>
  {{else}}
    <div class="eyebrow">Rishabh Daga</div>
    <h1>Projects</h1>
    <p>A growing collection of things I’m building.</p>
    <a class="card" href="/codebreaker/">
      <h2>Codebreaker</h2>
      <p>A new web game, built with a Go backend.</p>
      <span class="tag">In development →</span>
    </a>
  {{end}}
</body>
</html>`))

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		render(w, "Projects", false)
	})
	mux.HandleFunc("/codebreaker", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/codebreaker/", http.StatusFound)
	})
	mux.HandleFunc("/codebreaker/", func(w http.ResponseWriter, r *http.Request) {
		render(w, "Codebreaker", true)
	})

	server := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func render(w http.ResponseWriter, title string, game bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages.Execute(w, struct {
		Title string
		Game  bool
	}{title, game}); err != nil {
		log.Printf("render page: %v", err)
	}
}
