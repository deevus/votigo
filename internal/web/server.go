// internal/web/server.go
package web

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/palm-arcade/votigo/internal/db"
	"github.com/palm-arcade/votigo/templates"
)

type Server struct {
	db            *sql.DB
	queries       *db.Queries
	templates     map[string]*template.Template
	adminPassword string
}

func NewServer(database *sql.DB, adminPassword string) (*Server, error) {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	tmpls := make(map[string]*template.Template)

	// List of page templates to load with layout
	pages := []string{
		"home.html",
		"vote.html",
		"results.html",
		"error.html",
		"admin/dashboard.html",
		"admin/category.html",
	}

	layoutContent, err := templates.FS.ReadFile("layout.html")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		pageContent, err := templates.FS.ReadFile(page)
		if err != nil {
			// Skip missing templates for now
			continue
		}

		// Create a separate template for each page
		t, err := template.New(page).Funcs(funcMap).Parse(string(layoutContent) + string(pageContent))
		if err != nil {
			return nil, err
		}
		tmpls[page] = t
	}

	return &Server{
		db:            database,
		queries:       db.New(database),
		templates:     tmpls,
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
	t, ok := s.templates[name]
	if !ok {
		log.Printf("Template not found: %s", name)
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	err := t.Execute(w, data)
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

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	categories, err := s.queries.ListOpenCategories(r.Context())
	if err != nil {
		s.renderError(w, "Failed to load categories", err)
		return
	}

	s.render(w, "home.html", map[string]any{
		"Categories": categories,
	})
}

func (s *Server) handleVote(w http.ResponseWriter, r *http.Request) {
	// Extract ID from /vote/{id}
	idStr := r.URL.Path[len("/vote/"):]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	cat, err := s.queries.GetCategory(r.Context(), id)
	if err != nil {
		s.renderError(w, "Category not found", err)
		return
	}

	if cat.Status != "open" {
		s.render(w, "error.html", map[string]any{
			"Message": "Voting is not open for this category",
		})
		return
	}

	options, err := s.queries.ListOptionsByCategory(r.Context(), id)
	if err != nil {
		s.renderError(w, "Failed to load options", err)
		return
	}

	if r.Method == http.MethodPost {
		s.handleVoteSubmit(w, r, cat, options)
		return
	}

	// Build ranks slice for ranked voting
	var ranks []int
	maxRank := int64(3)
	if cat.MaxRank.Valid {
		maxRank = cat.MaxRank.Int64
	}
	if cat.VoteType == "ranked" {
		ranks = make([]int, maxRank)
	}

	s.render(w, "vote.html", map[string]any{
		"Category": cat,
		"Options":  options,
		"Ranks":    ranks,
		"MaxRank":  maxRank,
	})
}

func (s *Server) handleVoteSubmit(w http.ResponseWriter, r *http.Request,
	cat db.Category, options []db.Option) {

	r.ParseForm()

	nickname := strings.TrimSpace(r.FormValue("nickname"))
	if nickname == "" {
		s.render(w, "vote.html", map[string]any{
			"Category": cat,
			"Options":  options,
			"Error":    "Please enter a nickname",
		})
		return
	}
	nickname = strings.ToLower(nickname)

	// Collect selections based on vote type
	type selection struct {
		OptionID int64
		Rank     sql.NullInt64
	}
	var selections []selection

	switch cat.VoteType {
	case "single":
		choiceStr := r.FormValue("choice")
		if choiceStr == "" {
			s.render(w, "vote.html", map[string]any{
				"Category": cat,
				"Options":  options,
				"Nickname": nickname,
				"Error":    "Please make a selection",
			})
			return
		}
		optID, _ := strconv.ParseInt(choiceStr, 10, 64)
		selections = append(selections, selection{OptionID: optID})

	case "approval":
		choices := r.Form["choice"]
		if len(choices) == 0 {
			s.render(w, "vote.html", map[string]any{
				"Category": cat,
				"Options":  options,
				"Nickname": nickname,
				"Error":    "Please make at least one selection",
			})
			return
		}
		for _, c := range choices {
			optID, _ := strconv.ParseInt(c, 10, 64)
			selections = append(selections, selection{OptionID: optID})
		}

	case "ranked":
		maxRank := int64(3)
		if cat.MaxRank.Valid {
			maxRank = cat.MaxRank.Int64
		}
		seen := make(map[int64]bool)
		for i := int64(1); i <= maxRank; i++ {
			val := r.FormValue(fmt.Sprintf("rank%d", i))
			if val == "" {
				continue
			}
			optID, _ := strconv.ParseInt(val, 10, 64)
			if seen[optID] {
				s.render(w, "vote.html", map[string]any{
					"Category": cat,
					"Options":  options,
					"Nickname": nickname,
					"Ranks":    make([]int, maxRank),
					"MaxRank":  maxRank,
					"Error":    "Each choice must be different",
				})
				return
			}
			seen[optID] = true
			selections = append(selections, selection{
				OptionID: optID,
				Rank:     sql.NullInt64{Int64: i, Valid: true},
			})
		}
		if len(selections) == 0 {
			s.render(w, "vote.html", map[string]any{
				"Category": cat,
				"Options":  options,
				"Nickname": nickname,
				"Ranks":    make([]int, maxRank),
				"MaxRank":  maxRank,
				"Error":    "Please make at least one selection",
			})
			return
		}
	}

	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		s.renderError(w, "Database error", err)
		return
	}
	defer tx.Rollback()

	qtx := s.queries.WithTx(tx)

	// Upsert vote
	vote, err := qtx.UpsertVote(r.Context(), db.UpsertVoteParams{
		CategoryID: cat.ID,
		Nickname:   nickname,
	})
	if err != nil {
		s.renderError(w, "Failed to save vote", err)
		return
	}

	// Clear old selections
	err = qtx.DeleteVoteSelections(r.Context(), vote.ID)
	if err != nil {
		s.renderError(w, "Failed to update vote", err)
		return
	}

	// Insert new selections
	for _, sel := range selections {
		err = qtx.CreateVoteSelection(r.Context(), db.CreateVoteSelectionParams{
			VoteID:   vote.ID,
			OptionID: sel.OptionID,
			Rank:     sel.Rank,
		})
		if err != nil {
			s.renderError(w, "Failed to save selection", err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		s.renderError(w, "Failed to save vote", err)
		return
	}

	s.render(w, "vote.html", map[string]any{
		"Category": cat,
		"Success":  "Vote recorded! Thank you, " + nickname,
	})
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/results/"):]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	cat, err := s.queries.GetCategory(r.Context(), id)
	if err != nil {
		s.renderError(w, "Category not found", err)
		return
	}

	// Check visibility
	if cat.ShowResults == "after_close" && cat.Status != "closed" {
		s.render(w, "results.html", map[string]any{
			"Category":   cat,
			"NotVisible": true,
		})
		return
	}

	voteCount, _ := s.queries.CountVotesByCategory(r.Context(), id)

	type Result struct {
		Name       string
		Votes      int64
		Points     int64
		FirstPlace int64
	}
	var results []Result

	if cat.VoteType == "ranked" {
		maxRank := sql.NullInt64{Int64: 3, Valid: true}
		if cat.MaxRank.Valid {
			maxRank = cat.MaxRank
		}
		rows, err := s.queries.TallyRanked(r.Context(), db.TallyRankedParams{
			MaxRank:    maxRank,
			CategoryID: id,
		})
		if err != nil {
			s.renderError(w, "Failed to tally results", err)
			return
		}
		for _, row := range rows {
			// Points is interface{} due to COALESCE
			points := int64(0)
			if row.Points != nil {
				switch v := row.Points.(type) {
				case int64:
					points = v
				case float64:
					points = int64(v)
				}
			}
			results = append(results, Result{
				Name:       row.Name,
				Points:     points,
				FirstPlace: row.FirstPlaceVotes,
			})
		}
	} else {
		rows, err := s.queries.TallySimple(r.Context(), id)
		if err != nil {
			s.renderError(w, "Failed to tally results", err)
			return
		}
		for _, row := range rows {
			results = append(results, Result{
				Name:  row.Name,
				Votes: row.Votes,
			})
		}
	}

	s.render(w, "results.html", map[string]any{
		"Category":  cat,
		"VoteCount": voteCount,
		"Results":   results,
	})
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
