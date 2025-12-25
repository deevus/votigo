// internal/web/server.go
package web

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/palm-arcade/votigo/internal/db"
	"github.com/palm-arcade/votigo/templates"
)

type Server struct {
	db            *sql.DB
	queries       *db.Queries
	templates     *template.Template
	adminPassword string
}

func NewServer(database *sql.DB, adminPassword string) (*Server, error) {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templates.FS,
		"*.html", "admin/*.html")
	if err != nil {
		return nil, err
	}

	return &Server{
		db:            database,
		queries:       db.New(database),
		templates:     tmpl,
		adminPassword: adminPassword,
	}, nil
}

func (s *Server) Start(port int) error {
	mux := http.NewServeMux()

	// Voter routes
	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/vote/", s.handleVote)
	mux.HandleFunc("/results/", s.handleResults)

	// Admin routes
	mux.HandleFunc("/admin", s.handleAdmin)
	mux.HandleFunc("/admin/", s.handleAdmin)

	addr := ":" + strconv.Itoa(port)
	log.Printf("Starting server on http://0.0.0.0%s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	// First execute layout with data
	err := s.templates.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		log.Printf("Template error: %v", err)
	}
}

func (s *Server) renderError(w http.ResponseWriter, message string, err error) {
	log.Printf("Error: %s: %v", message, err)
	w.WriteHeader(http.StatusInternalServerError)
	s.render(w, "error.html", map[string]any{
		"Message": message,
	})
}

// Placeholder handlers - will be implemented in Phase 8
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Write([]byte("Home page - coming soon"))
}

func (s *Server) handleVote(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Vote page - coming soon"))
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Results page - coming soon"))
}

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	// Basic auth check
	user, pass, ok := r.BasicAuth()
	if !ok || user != "admin" || pass != s.adminPassword {
		w.Header().Set("WWW-Authenticate", `Basic realm="Admin"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	w.Write([]byte("Admin dashboard - coming soon"))
}
