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
	"github.com/palm-arcade/votigo/static"
	"github.com/palm-arcade/votigo/templates"
)

type UIMode string

const (
	UIModeModern UIMode = "modern"
	UIModeLegacy UIMode = "legacy"
)

type Server struct {
	db            *sql.DB
	queries       *db.Queries
	templates     map[string]*template.Template
	partials      map[string]*template.Template
	adminPassword string
	uiMode        UIMode
}

func NewServer(database *sql.DB, adminPassword string, uiMode UIMode) (*Server, error) {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	templateDir := string(uiMode)

	tmpls := make(map[string]*template.Template)
	partials := make(map[string]*template.Template)

	// List of page templates to load with layout
	pages := []string{
		"home.html",
		"vote.html",
		"results.html",
		"error.html",
		"admin/dashboard.html",
		"admin/category.html",
	}

	layoutContent, err := templates.FS.ReadFile(templateDir + "/layout.html")
	if err != nil {
		return nil, fmt.Errorf("failed to read layout: %w", err)
	}

	for _, page := range pages {
		pageContent, err := templates.FS.ReadFile(templateDir + "/" + page)
		if err != nil {
			continue
		}

		t, err := template.New(page).Funcs(funcMap).Parse(string(layoutContent) + string(pageContent))
		if err != nil {
			return nil, err
		}
		tmpls[page] = t
	}

	// Load partials for modern UI (htmx responses)
	if uiMode == UIModeModern {
		partialFiles := []string{
			"partials/vote-form.html",
			"partials/vote-success.html",
			"partials/option-row.html",
			"partials/results-table.html",
			"partials/status-badge.html",
		}
		for _, partial := range partialFiles {
			content, err := templates.FS.ReadFile("modern/" + partial)
			if err != nil {
				continue
			}
			t, err := template.New(partial).Funcs(funcMap).Parse(string(content))
			if err != nil {
				return nil, err
			}
			partials[partial] = t
		}
	}

	return &Server{
		db:            database,
		queries:       db.New(database),
		templates:     tmpls,
		partials:      partials,
		adminPassword: adminPassword,
		uiMode:        uiMode,
	}, nil
}

func (s *Server) Start(port int) error {
	mux := http.NewServeMux()

	// Static files (for modern UI)
	if s.uiMode == UIModeModern {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(static.FS))))
	}

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

func (s *Server) isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
	t, ok := s.partials[name]
	if !ok {
		log.Printf("Partial not found: %s", name)
		http.Error(w, "Partial not found", http.StatusInternalServerError)
		return
	}
	err := t.Execute(w, data)
	if err != nil {
		log.Printf("Partial error: %v", err)
	}
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

	// Helper to build vote form data with error
	maxRank := int64(3)
	if cat.MaxRank.Valid {
		maxRank = cat.MaxRank.Int64
	}
	var ranks []int
	if cat.VoteType == "ranked" {
		ranks = make([]int, maxRank)
	}

	renderVoteError := func(nickname, errMsg string) {
		data := map[string]any{
			"Category": cat,
			"Options":  options,
			"Nickname": nickname,
			"Ranks":    ranks,
			"MaxRank":  maxRank,
			"Error":    errMsg,
		}
		if s.isHTMX(r) {
			s.renderPartial(w, "partials/vote-form.html", data)
		} else {
			s.render(w, "vote.html", data)
		}
	}

	nickname := strings.TrimSpace(r.FormValue("nickname"))
	if nickname == "" {
		renderVoteError("", "Please enter a nickname")
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
			renderVoteError(nickname, "Please make a selection")
			return
		}
		optID, _ := strconv.ParseInt(choiceStr, 10, 64)
		selections = append(selections, selection{OptionID: optID})

	case "approval":
		choices := r.Form["choice"]
		if len(choices) == 0 {
			renderVoteError(nickname, "Please make at least one selection")
			return
		}
		for _, c := range choices {
			optID, _ := strconv.ParseInt(c, 10, 64)
			selections = append(selections, selection{OptionID: optID})
		}

	case "ranked":
		seen := make(map[int64]bool)
		for i := int64(1); i <= maxRank; i++ {
			val := r.FormValue(fmt.Sprintf("rank%d", i))
			if val == "" {
				continue
			}
			optID, _ := strconv.ParseInt(val, 10, 64)
			if seen[optID] {
				renderVoteError(nickname, "Each choice must be different")
				return
			}
			seen[optID] = true
			selections = append(selections, selection{
				OptionID: optID,
				Rank:     sql.NullInt64{Int64: i, Valid: true},
			})
		}
		if len(selections) == 0 {
			renderVoteError(nickname, "Please make at least one selection")
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

	data := map[string]any{
		"Category": cat,
		"Success":  "Vote recorded! Thank you, " + nickname,
	}

	if s.isHTMX(r) {
		s.renderPartial(w, "partials/vote-form.html", data)
	} else {
		s.render(w, "vote.html", data)
	}
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

	path := r.URL.Path

	// Route admin requests
	switch {
	case path == "/admin" || path == "/admin/":
		s.handleAdminDashboard(w, r)
	case strings.HasPrefix(path, "/admin/category/"):
		s.handleAdminCategory(w, r)
	case strings.HasPrefix(path, "/admin/option/"):
		s.handleAdminDeleteOption(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	categories, err := s.queries.ListCategories(r.Context())
	if err != nil {
		s.renderError(w, "Failed to load categories", err)
		return
	}

	s.render(w, "admin/dashboard.html", map[string]any{
		"Categories": categories,
	})
}

func (s *Server) handleAdminCategory(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /admin/category/new
	if strings.HasSuffix(path, "/new") {
		s.handleAdminCategoryNew(w, r)
		return
	}

	// Extract category ID and action
	parts := strings.Split(strings.TrimPrefix(path, "/admin/category/"), "/")
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "open":
		s.handleAdminOpen(w, r, id)
	case "close":
		s.handleAdminClose(w, r, id)
	case "option":
		s.handleAdminAddOption(w, r, id)
	default:
		s.handleAdminCategoryEdit(w, r, id)
	}
}

func (s *Server) handleAdminCategoryNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		name := r.FormValue("name")
		voteType := r.FormValue("vote_type")
		showResults := r.FormValue("show_results")
		maxRankStr := r.FormValue("max_rank")

		var maxRank sql.NullInt64
		if voteType == "ranked" {
			mr, _ := strconv.ParseInt(maxRankStr, 10, 64)
			if mr <= 0 {
				mr = 3
			}
			maxRank = sql.NullInt64{Int64: mr, Valid: true}
		}

		_, err := s.queries.CreateCategory(r.Context(), db.CreateCategoryParams{
			Name:        name,
			VoteType:    voteType,
			Status:      "draft",
			ShowResults: showResults,
			MaxRank:     maxRank,
		})
		if err != nil {
			s.render(w, "admin/category.html", map[string]any{
				"Error": "Failed to create category",
			})
			return
		}
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	s.render(w, "admin/category.html", nil)
}

func (s *Server) handleAdminCategoryEdit(w http.ResponseWriter, r *http.Request, id int64) {
	cat, err := s.queries.GetCategory(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	options, _ := s.queries.ListOptionsByCategory(r.Context(), id)

	if r.Method == http.MethodPost {
		r.ParseForm()
		name := strings.TrimSpace(r.FormValue("name"))
		voteType := r.FormValue("vote_type")
		showResults := r.FormValue("show_results")
		maxRankStr := r.FormValue("max_rank")

		if name == "" {
			s.render(w, "admin/category.html", map[string]any{
				"Category": cat,
				"Options":  options,
				"Error":    "Name is required",
			})
			return
		}

		var maxRank sql.NullInt64
		if voteType == "ranked" {
			mr, _ := strconv.ParseInt(maxRankStr, 10, 64)
			if mr <= 0 {
				mr = 3
			}
			maxRank = sql.NullInt64{Int64: mr, Valid: true}
		}

		err := s.queries.UpdateCategory(r.Context(), db.UpdateCategoryParams{
			Name:        name,
			VoteType:    voteType,
			ShowResults: showResults,
			MaxRank:     maxRank,
			ID:          id,
		})
		if err != nil {
			s.render(w, "admin/category.html", map[string]any{
				"Category": cat,
				"Options":  options,
				"Error":    "Failed to update category",
			})
			return
		}

		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	s.render(w, "admin/category.html", map[string]any{
		"Category": cat,
		"Options":  options,
	})
}

func (s *Server) handleAdminOpen(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	count, _ := s.queries.CountOptionsByCategory(r.Context(), id)
	if count == 0 {
		cat, _ := s.queries.GetCategory(r.Context(), id)
		options, _ := s.queries.ListOptionsByCategory(r.Context(), id)
		s.render(w, "admin/category.html", map[string]any{
			"Category": cat,
			"Options":  options,
			"Error":    "Cannot open voting: add at least one option first",
		})
		return
	}

	s.queries.UpdateCategoryStatus(r.Context(), db.UpdateCategoryStatusParams{
		Status: "open",
		ID:     id,
	})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) handleAdminClose(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	s.queries.UpdateCategoryStatus(r.Context(), db.UpdateCategoryStatusParams{
		Status: "closed",
		ID:     id,
	})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) handleAdminAddOption(w http.ResponseWriter, r *http.Request, categoryID int64) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("option_name"))
	if name == "" {
		http.Redirect(w, r, fmt.Sprintf("/admin/category/%d", categoryID), http.StatusSeeOther)
		return
	}

	count, _ := s.queries.CountOptionsByCategory(r.Context(), categoryID)
	s.queries.CreateOption(r.Context(), db.CreateOptionParams{
		CategoryID: categoryID,
		Name:       name,
		SortOrder:  sql.NullInt64{Int64: count, Valid: true},
	})

	http.Redirect(w, r, fmt.Sprintf("/admin/category/%d", categoryID), http.StatusSeeOther)
}

func (s *Server) handleAdminDeleteOption(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	// Parse /admin/option/{id}/delete
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/admin/option/"), "/")
	if len(parts) < 2 || parts[1] != "delete" {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	opt, err := s.queries.GetOption(r.Context(), id)
	if err != nil {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	s.queries.DeleteOption(r.Context(), id)
	http.Redirect(w, r, fmt.Sprintf("/admin/category/%d", opt.CategoryID), http.StatusSeeOther)
}
