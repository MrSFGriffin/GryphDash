package main

import (
	_ "embed"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"
)

//go:embed web/dashboard.html
var dashboardHTML string

type metric struct {
	Label string
	Value string
	Note  string
}

type provider struct {
	Name    string
	Detail  string
	Metrics []metric
}

func newHandler() http.Handler {
	page := template.Must(template.New("dashboard").Parse(dashboardHTML))
	providers := []provider{
		{
			Name:   "Codex",
			Detail: "Usage & rate limits",
			Metrics: []metric{
				{Label: "Usage this week", Value: "128 requests", Note: "Illustrative activity count"},
				{Label: "Short window remaining", Value: "72%", Note: "Example reset: in 2 hours"},
				{Label: "Weekly limit remaining", Value: "84%", Note: "Example reset: in 4 days"},
			},
		},
		{
			Name:   "OpenRouter",
			Detail: "Spend & budget · USD",
			Metrics: []metric{
				{Label: "Spend this month", Value: "$12.40", Note: "Example monthly budget: $50.00"},
				{Label: "Budget remaining", Value: "$37.60", Note: "75.2% of example monthly budget"},
				{Label: "Credit balance", Value: "$87.60", Note: "Example prepaid credits"},
			},
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodHead {
			return
		}
		if err := page.Execute(w, providers); err != nil {
			log.Printf("render dashboard: %v", err)
		}
	})
}

func main() {
	addr := os.Getenv("GRYPHDASH_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("GryphDash listening on http://%s (demo data)", addr)
	log.Fatal(server.ListenAndServe())
}
