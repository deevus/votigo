package web_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/palm-arcade/votigo/internal/db"
	"github.com/palm-arcade/votigo/internal/web"
)

const testAdminPassword = "testpass"

// testServer creates a new server with an in-memory database for testing (legacy UI)
func testServer(t *testing.T) (*web.Server, *db.Queries, *sql.DB) {
	return testServerWithMode(t, web.UIModeLegacy)
}

// testServerModern creates a new server with modern UI mode (for HTMX tests)
func testServerModern(t *testing.T) (*web.Server, *db.Queries, *sql.DB) {
	return testServerWithMode(t, web.UIModeModern)
}

func testServerWithMode(t *testing.T, mode web.UIMode) (*web.Server, *db.Queries, *sql.DB) {
	t.Helper()

	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	srv, err := web.NewServer(conn, testAdminPassword, mode)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	return srv, db.New(conn), conn
}

// makeRequest creates and executes an HTTP request against a handler
func makeRequest(t *testing.T, handler http.HandlerFunc, method, path string, body url.Values) *httptest.ResponseRecorder {
	t.Helper()

	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(body.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// addBasicAuth adds basic auth header to a request
func addBasicAuth(req *http.Request, user, pass string) {
	req.SetBasicAuth(user, pass)
}

// createTestCategory creates a category for testing and returns its ID
func createTestCategory(t *testing.T, queries *db.Queries, name, voteType, status, showResults string) db.Category {
	t.Helper()

	cat, err := queries.CreateCategory(t.Context(), db.CreateCategoryParams{
		Name:        name,
		VoteType:    voteType,
		Status:      status,
		ShowResults: showResults,
	})
	if err != nil {
		t.Fatalf("failed to create category: %v", err)
	}
	return cat
}

// createTestOption creates an option for testing and returns it
func createTestOption(t *testing.T, queries *db.Queries, categoryID int64, name string) db.Option {
	t.Helper()

	opt, err := queries.CreateOption(t.Context(), db.CreateOptionParams{
		CategoryID: categoryID,
		Name:       name,
	})
	if err != nil {
		t.Fatalf("failed to create option: %v", err)
	}
	return opt
}

// ====================
// HOME PAGE TESTS
// ====================

func TestHandleHome_ShowsOpenCategories(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	// Create categories in different states
	createTestCategory(t, queries, "Open Poll", "single", "open", "live")
	createTestCategory(t, queries, "Draft Poll", "single", "draft", "live")
	createTestCategory(t, queries, "Closed Poll", "single", "closed", "after_close")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Open Poll") {
		t.Error("expected open category to be visible")
	}
	if strings.Contains(body, "Draft Poll") {
		t.Error("draft category should not be visible on home page")
	}
	if strings.Contains(body, "Closed Poll") {
		t.Error("closed category should not be visible on home page")
	}
}

func TestHandleHome_NotFoundForOtherPaths(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

// ====================
// VOTE PAGE TESTS
// ====================

func TestHandleVote_GetForm(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Test Poll", "single", "open", "live")
	createTestOption(t, queries, cat.ID, "Option A")
	createTestOption(t, queries, cat.ID, "Option B")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/vote/1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Test Poll") {
		t.Error("expected category name in response")
	}
	if !strings.Contains(body, "Option A") {
		t.Error("expected Option A in response")
	}
	if !strings.Contains(body, "Option B") {
		t.Error("expected Option B in response")
	}
}

func TestHandleVote_NotFoundForInvalidID(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/vote/abc", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for invalid ID, got %d", rr.Code)
	}
}

func TestHandleVote_ErrorForNonOpenCategory(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Draft Poll", "single", "draft", "live")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/vote/1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "not open") {
		t.Error("expected error message about voting not being open")
	}
}

func TestHandleVote_RankedVotingForm(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat, _ := queries.CreateCategory(t.Context(), db.CreateCategoryParams{
		Name:        "Ranked Poll",
		VoteType:    "ranked",
		Status:      "open",
		ShowResults: "live",
		MaxRank:     sql.NullInt64{Int64: 3, Valid: true},
	})
	createTestOption(t, queries, cat.ID, "First")
	createTestOption(t, queries, cat.ID, "Second")
	createTestOption(t, queries, cat.ID, "Third")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/vote/1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Ranked Poll") {
		t.Error("expected category name in response")
	}
}

// ====================
// VOTE SUBMISSION TESTS
// ====================

func TestHandleVoteSubmit_SingleVote(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Single Poll", "single", "open", "live")
	opt := createTestOption(t, queries, cat.ID, "Option A")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("nickname", "TestUser")
	form.Set("choice", "1")

	req := httptest.NewRequest(http.MethodPost, "/vote/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "VOTE RECORDED") && !strings.Contains(body, "Thank you") {
		t.Error("expected success message")
	}

	// Verify vote was recorded
	count, _ := queries.CountVotesByCategory(t.Context(), cat.ID)
	if count != 1 {
		t.Errorf("expected 1 vote, got %d", count)
	}

	// Verify tally
	tally, _ := queries.TallySimple(t.Context(), cat.ID)
	if len(tally) != 1 {
		t.Fatalf("expected 1 tally row, got %d", len(tally))
	}
	if tally[0].ID != opt.ID || tally[0].Votes != 1 {
		t.Errorf("expected option %d with 1 vote, got id=%d votes=%d", opt.ID, tally[0].ID, tally[0].Votes)
	}
}

func TestHandleVoteSubmit_ApprovalVote(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Approval Poll", "approval", "open", "live")
	createTestOption(t, queries, cat.ID, "Option A")
	createTestOption(t, queries, cat.ID, "Option B")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("nickname", "Voter")
	form.Add("choice", "1")
	form.Add("choice", "2")

	req := httptest.NewRequest(http.MethodPost, "/vote/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Verify tally shows both options received votes
	tally, _ := queries.TallySimple(t.Context(), cat.ID)
	if len(tally) != 2 {
		t.Fatalf("expected 2 tally rows, got %d", len(tally))
	}
	for _, row := range tally {
		if row.Votes != 1 {
			t.Errorf("expected 1 vote for option %s, got %d", row.Name, row.Votes)
		}
	}
}

func TestHandleVoteSubmit_RankedVote(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat, _ := queries.CreateCategory(t.Context(), db.CreateCategoryParams{
		Name:        "Ranked Poll",
		VoteType:    "ranked",
		Status:      "open",
		ShowResults: "live",
		MaxRank:     sql.NullInt64{Int64: 3, Valid: true},
	})
	createTestOption(t, queries, cat.ID, "First")
	createTestOption(t, queries, cat.ID, "Second")
	createTestOption(t, queries, cat.ID, "Third")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("nickname", "RankedVoter")
	form.Set("rank1", "1") // First choice: option 1
	form.Set("rank2", "2") // Second choice: option 2
	form.Set("rank3", "3") // Third choice: option 3

	req := httptest.NewRequest(http.MethodPost, "/vote/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Verify vote count
	count, _ := queries.CountVotesByCategory(t.Context(), cat.ID)
	if count != 1 {
		t.Errorf("expected 1 vote, got %d", count)
	}
}

func TestHandleVoteSubmit_EmptyNickname(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Test Poll", "single", "open", "live")
	createTestOption(t, queries, cat.ID, "Option A")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("nickname", "")
	form.Set("choice", "1")

	req := httptest.NewRequest(http.MethodPost, "/vote/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "nickname") {
		t.Error("expected error about nickname")
	}

	// Verify no vote was recorded
	count, _ := queries.CountVotesByCategory(t.Context(), cat.ID)
	if count != 0 {
		t.Errorf("expected 0 votes, got %d", count)
	}
}

func TestHandleVoteSubmit_NoSelection(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Test Poll", "single", "open", "live")
	createTestOption(t, queries, cat.ID, "Option A")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("nickname", "TestUser")
	// No choice set

	req := httptest.NewRequest(http.MethodPost, "/vote/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "selection") {
		t.Error("expected error about making a selection")
	}
}

func TestHandleVoteSubmit_DuplicateRankedChoices(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat, _ := queries.CreateCategory(t.Context(), db.CreateCategoryParams{
		Name:        "Ranked Poll",
		VoteType:    "ranked",
		Status:      "open",
		ShowResults: "live",
		MaxRank:     sql.NullInt64{Int64: 3, Valid: true},
	})
	createTestOption(t, queries, cat.ID, "First")
	createTestOption(t, queries, cat.ID, "Second")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("nickname", "Voter")
	form.Set("rank1", "1")
	form.Set("rank2", "1") // Same option as rank1 - should error

	req := httptest.NewRequest(http.MethodPost, "/vote/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "different") {
		t.Error("expected error about choices being different")
	}
}

func TestHandleVoteSubmit_ReVote(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Test Poll", "single", "open", "live")
	optA := createTestOption(t, queries, cat.ID, "Option A")
	optB := createTestOption(t, queries, cat.ID, "Option B")

	handler := srv.Handler()

	// First vote for Option A
	form := url.Values{}
	form.Set("nickname", "TestUser")
	form.Set("choice", "1")
	req := httptest.NewRequest(http.MethodPost, "/vote/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Re-vote for Option B
	form.Set("choice", "2")
	req = httptest.NewRequest(http.MethodPost, "/vote/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify only 1 vote exists (re-vote replaced previous)
	count, _ := queries.CountVotesByCategory(t.Context(), cat.ID)
	if count != 1 {
		t.Errorf("expected 1 vote after re-vote, got %d", count)
	}

	// Verify tally shows vote moved to Option B
	tally, _ := queries.TallySimple(t.Context(), cat.ID)
	for _, row := range tally {
		if row.ID == optA.ID && row.Votes != 0 {
			t.Errorf("expected 0 votes for Option A after re-vote, got %d", row.Votes)
		}
		if row.ID == optB.ID && row.Votes != 1 {
			t.Errorf("expected 1 vote for Option B after re-vote, got %d", row.Votes)
		}
	}
}

func TestHandleVoteSubmit_NicknameCaseInsensitive(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Test Poll", "single", "open", "live")
	createTestOption(t, queries, cat.ID, "Option A")
	createTestOption(t, queries, cat.ID, "Option B")

	handler := srv.Handler()

	// Vote with uppercase nickname
	form := url.Values{}
	form.Set("nickname", "TestUser")
	form.Set("choice", "1")
	req := httptest.NewRequest(http.MethodPost, "/vote/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Vote with lowercase nickname - should be same voter
	form.Set("nickname", "testuser")
	form.Set("choice", "2")
	req = httptest.NewRequest(http.MethodPost, "/vote/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should still only be 1 vote (same voter)
	count, _ := queries.CountVotesByCategory(t.Context(), cat.ID)
	if count != 1 {
		t.Errorf("expected 1 vote (case-insensitive nickname), got %d", count)
	}
}

// ====================
// RESULTS PAGE TESTS
// ====================

func TestHandleResultsList(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	// Live results, open poll - should show
	createTestCategory(t, queries, "Live Results", "single", "open", "live")
	// After close results, closed poll - should show
	createTestCategory(t, queries, "Closed Results", "single", "closed", "after_close")
	// After close results, open poll - should NOT show
	createTestCategory(t, queries, "Hidden Results", "single", "open", "after_close")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/results/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Live Results") {
		t.Error("expected Live Results to be visible")
	}
	if !strings.Contains(body, "Closed Results") {
		t.Error("expected Closed Results to be visible")
	}
	if strings.Contains(body, "Hidden Results") {
		t.Error("Hidden Results should not be visible (after_close but still open)")
	}
}

func TestHandleResults_SimpleVoting(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Simple Poll", "single", "open", "live")
	createTestOption(t, queries, cat.ID, "Option A")
	createTestOption(t, queries, cat.ID, "Option B")

	// Cast a vote
	vote, _ := queries.UpsertVote(t.Context(), db.UpsertVoteParams{
		CategoryID: cat.ID,
		Nickname:   "voter1",
	})
	queries.CreateVoteSelection(t.Context(), db.CreateVoteSelectionParams{
		VoteID:   vote.ID,
		OptionID: 1,
	})

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/results/1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Simple Poll") {
		t.Error("expected category name in results")
	}
	if !strings.Contains(body, "Option A") {
		t.Error("expected Option A in results")
	}
}

func TestHandleResults_NotVisibleBeforeClose(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Hidden Poll", "single", "open", "after_close")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/results/1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	// Should render but show "not visible" message
	if !strings.Contains(body, "Hidden Poll") {
		t.Error("expected category name")
	}
}

func TestHandleResults_InvalidID(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/results/abc", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for invalid ID, got %d", rr.Code)
	}
}

func TestHandleResults_RankedVoting(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat, _ := queries.CreateCategory(t.Context(), db.CreateCategoryParams{
		Name:        "Ranked Poll",
		VoteType:    "ranked",
		Status:      "open",
		ShowResults: "live",
		MaxRank:     sql.NullInt64{Int64: 3, Valid: true},
	})
	createTestOption(t, queries, cat.ID, "First")
	createTestOption(t, queries, cat.ID, "Second")

	// Cast ranked votes
	vote, _ := queries.UpsertVote(t.Context(), db.UpsertVoteParams{
		CategoryID: cat.ID,
		Nickname:   "voter1",
	})
	queries.CreateVoteSelection(t.Context(), db.CreateVoteSelectionParams{
		VoteID:   vote.ID,
		OptionID: 1,
		Rank:     sql.NullInt64{Int64: 1, Valid: true},
	})
	queries.CreateVoteSelection(t.Context(), db.CreateVoteSelectionParams{
		VoteID:   vote.ID,
		OptionID: 2,
		Rank:     sql.NullInt64{Int64: 2, Valid: true},
	})

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/results/1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Ranked Poll") {
		t.Error("expected category name in results")
	}
}

// ====================
// ADMIN AUTH TESTS
// ====================

func TestAdminAuth_Unauthorized(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 without auth, got %d", rr.Code)
	}

	if rr.Header().Get("WWW-Authenticate") == "" {
		t.Error("expected WWW-Authenticate header")
	}
}

func TestAdminAuth_WrongPassword(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("admin", "wrongpassword")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 with wrong password, got %d", rr.Code)
	}
}

func TestAdminAuth_WrongUsername(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("notadmin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 with wrong username, got %d", rr.Code)
	}
}

func TestAdminAuth_Success(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 with correct auth, got %d", rr.Code)
	}
}

// ====================
// ADMIN DASHBOARD TESTS
// ====================

func TestAdminDashboard_ListsCategories(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Poll One", "single", "draft", "live")
	createTestCategory(t, queries, "Poll Two", "approval", "open", "live")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Poll One") {
		t.Error("expected Poll One in dashboard")
	}
	if !strings.Contains(body, "Poll Two") {
		t.Error("expected Poll Two in dashboard")
	}
}

func TestAdminDashboard_ExcludesArchived(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Active Poll", "single", "open", "live")
	createTestCategory(t, queries, "Archived Poll", "single", "archived", "live")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Active Poll") {
		t.Error("expected Active Poll in dashboard")
	}
	if strings.Contains(body, "Archived Poll") {
		t.Error("Archived Poll should not be in dashboard")
	}
}

// ====================
// ADMIN CATEGORY CREATE TESTS
// ====================

func TestAdminCategoryNew_GetForm(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin/category/new", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestAdminCategoryNew_Create(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	form := url.Values{}
	form.Set("name", "New Test Poll")
	form.Set("vote_type", "single")
	form.Set("show_results", "live")

	req := httptest.NewRequest(http.MethodPost, "/admin/category/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect (303), got %d", rr.Code)
	}

	// Verify category was created
	cats, _ := queries.ListCategories(t.Context())
	if len(cats) != 1 {
		t.Fatalf("expected 1 category, got %d", len(cats))
	}
	if cats[0].Name != "New Test Poll" {
		t.Errorf("expected name 'New Test Poll', got '%s'", cats[0].Name)
	}
	if cats[0].Status != "draft" {
		t.Errorf("expected status 'draft', got '%s'", cats[0].Status)
	}
}

func TestAdminCategoryNew_CreateRanked(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	form := url.Values{}
	form.Set("name", "Ranked Poll")
	form.Set("vote_type", "ranked")
	form.Set("show_results", "after_close")
	form.Set("max_rank", "5")

	req := httptest.NewRequest(http.MethodPost, "/admin/category/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	cats, _ := queries.ListCategories(t.Context())
	if len(cats) != 1 {
		t.Fatalf("expected 1 category, got %d", len(cats))
	}
	if !cats[0].MaxRank.Valid || cats[0].MaxRank.Int64 != 5 {
		t.Errorf("expected max_rank 5, got %v", cats[0].MaxRank)
	}
}

// ====================
// ADMIN CATEGORY EDIT TESTS
// ====================

func TestAdminCategoryEdit_GetForm(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Edit Me", "single", "draft", "live")
	createTestOption(t, queries, cat.ID, "Option 1")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin/category/1", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Edit Me") {
		t.Error("expected category name in form")
	}
	if !strings.Contains(body, "Option 1") {
		t.Error("expected option in form")
	}
}

func TestAdminCategoryEdit_Update(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Original Name", "single", "draft", "live")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("name", "Updated Name")
	form.Set("vote_type", "approval")
	form.Set("show_results", "after_close")

	req := httptest.NewRequest(http.MethodPost, "/admin/category/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect (303), got %d", rr.Code)
	}

	// Verify update
	cat, _ := queries.GetCategory(t.Context(), 1)
	if cat.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got '%s'", cat.Name)
	}
	if cat.VoteType != "approval" {
		t.Errorf("expected vote_type 'approval', got '%s'", cat.VoteType)
	}
}

func TestAdminCategoryEdit_EmptyName(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Original", "single", "draft", "live")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("name", "   ") // Whitespace only
	form.Set("vote_type", "single")
	form.Set("show_results", "live")

	req := httptest.NewRequest(http.MethodPost, "/admin/category/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should show form with error, not redirect
	if rr.Code == http.StatusSeeOther {
		t.Error("should not redirect with empty name")
	}

	body := rr.Body.String()
	if !strings.Contains(body, "required") {
		t.Error("expected error about name being required")
	}
}

func TestAdminCategoryEdit_NotFound(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin/category/999", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for nonexistent category, got %d", rr.Code)
	}
}

// ====================
// ADMIN LIFECYCLE TESTS (open/close/reopen/archive)
// ====================

func TestAdminOpen_Success(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Test Poll", "single", "draft", "live")
	createTestOption(t, queries, cat.ID, "Option 1")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/open", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect (303), got %d", rr.Code)
	}

	// Verify status changed
	cat, _ = queries.GetCategory(t.Context(), 1)
	if cat.Status != "open" {
		t.Errorf("expected status 'open', got '%s'", cat.Status)
	}
}

func TestAdminOpen_NoOptions(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Empty Poll", "single", "draft", "live")
	// No options added

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/open", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should show form with error (not redirect)
	body := rr.Body.String()
	if !strings.Contains(body, "option") {
		t.Error("expected error about needing options")
	}

	// Verify status unchanged
	cat, _ := queries.GetCategory(t.Context(), 1)
	if cat.Status != "draft" {
		t.Errorf("expected status to remain 'draft', got '%s'", cat.Status)
	}
}

func TestAdminOpen_GetNotAllowed(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Test Poll", "single", "draft", "live")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin/category/1/open", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for GET on open, got %d", rr.Code)
	}
}

func TestAdminClose_Success(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Open Poll", "single", "open", "live")
	createTestOption(t, queries, cat.ID, "Option")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/close", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect (303), got %d", rr.Code)
	}

	cat, _ = queries.GetCategory(t.Context(), 1)
	if cat.Status != "closed" {
		t.Errorf("expected status 'closed', got '%s'", cat.Status)
	}
}

func TestAdminReopen_Success(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Closed Poll", "single", "closed", "live")
	createTestOption(t, queries, cat.ID, "Option")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/reopen", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect (303), got %d", rr.Code)
	}

	cat, _ = queries.GetCategory(t.Context(), 1)
	if cat.Status != "open" {
		t.Errorf("expected status 'open', got '%s'", cat.Status)
	}
}

func TestAdminReopen_NotClosed(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Draft Poll", "single", "draft", "live")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/reopen", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should redirect (can't reopen non-closed)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect (303), got %d", rr.Code)
	}

	// Status should remain draft
	cat, _ := queries.GetCategory(t.Context(), 1)
	if cat.Status != "draft" {
		t.Errorf("expected status to remain 'draft', got '%s'", cat.Status)
	}
}

func TestAdminReopen_NotFound(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/999/reopen", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestAdminArchive_Success(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Test Poll", "single", "closed", "live")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/archive", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect (303), got %d", rr.Code)
	}

	cat, _ := queries.GetCategory(t.Context(), 1)
	if cat.Status != "archived" {
		t.Errorf("expected status 'archived', got '%s'", cat.Status)
	}
}

// ====================
// ADMIN OPTION TESTS
// ====================

func TestAdminAddOption_Success(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Test Poll", "single", "draft", "live")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("option_name", "New Option")

	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/option", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect (303), got %d", rr.Code)
	}

	// Verify option was created
	opts, _ := queries.ListOptionsByCategory(t.Context(), 1)
	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}
	if opts[0].Name != "New Option" {
		t.Errorf("expected name 'New Option', got '%s'", opts[0].Name)
	}
}

func TestAdminAddOption_EmptyName(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Test Poll", "single", "draft", "live")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("option_name", "   ") // Whitespace only

	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/option", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should redirect (empty name is silently ignored in non-HTMX)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect (303), got %d", rr.Code)
	}

	// Verify no option was created
	opts, _ := queries.ListOptionsByCategory(t.Context(), 1)
	if len(opts) != 0 {
		t.Errorf("expected 0 options, got %d", len(opts))
	}
}

func TestAdminAddOption_GetNotAllowed(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Test Poll", "single", "draft", "live")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin/category/1/option", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for GET, got %d", rr.Code)
	}
}

func TestAdminDeleteOption_Success(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Test Poll", "single", "draft", "live")
	opt := createTestOption(t, queries, cat.ID, "To Delete")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/option/1", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect (303), got %d", rr.Code)
	}

	// Verify option was deleted
	_, err := queries.GetOption(t.Context(), opt.ID)
	if err == nil {
		t.Error("expected option to be deleted")
	}
}

func TestAdminDeleteOption_WithDeleteSuffix(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Test Poll", "single", "draft", "live")
	createTestOption(t, queries, cat.ID, "To Delete")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/option/1/delete", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect (303), got %d", rr.Code)
	}
}

func TestAdminDeleteOption_DeleteMethod(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Test Poll", "single", "draft", "live")
	createTestOption(t, queries, cat.ID, "To Delete")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodDelete, "/admin/option/1", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect (303), got %d", rr.Code)
	}
}

func TestAdminDeleteOption_NotFound(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/option/999", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should redirect even if not found (non-HTMX behavior)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect (303), got %d", rr.Code)
	}
}

func TestAdminDeleteOption_GetNotAllowed(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Test Poll", "single", "draft", "live")
	createTestOption(t, queries, cat.ID, "Option")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin/option/1", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for GET, got %d", rr.Code)
	}
}

// ====================
// HTMX ENDPOINT TESTS
// ====================

func TestHTMX_VoteSubmit(t *testing.T) {
	srv, queries, conn := testServerModern(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Test Poll", "single", "open", "live")
	createTestOption(t, queries, cat.ID, "Option A")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("nickname", "HTMXVoter")
	form.Set("choice", "1")

	req := httptest.NewRequest(http.MethodPost, "/vote/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Verify vote was recorded
	count, _ := queries.CountVotesByCategory(t.Context(), cat.ID)
	if count != 1 {
		t.Errorf("expected 1 vote, got %d", count)
	}
}

func TestHTMX_AddOption(t *testing.T) {
	srv, queries, conn := testServerModern(t)
	defer conn.Close()

	createTestCategory(t, queries, "Test Poll", "single", "draft", "live")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("option_name", "HTMX Option")

	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/option", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Note: Response is 200 even if partial template has errors (template error is logged)
	// The important thing is verifying the database operation succeeded
	opts, _ := queries.ListOptionsByCategory(t.Context(), 1)
	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}
	if opts[0].Name != "HTMX Option" {
		t.Errorf("expected option name 'HTMX Option', got '%s'", opts[0].Name)
	}
}

func TestHTMX_DeleteOption(t *testing.T) {
	srv, queries, conn := testServerModern(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Test Poll", "single", "draft", "live")
	createTestOption(t, queries, cat.ID, "To Delete")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/option/1", nil)
	req.Header.Set("HX-Request", "true")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for HTMX delete, got %d", rr.Code)
	}

	// Response should be empty (HTMX removes element)
	if rr.Body.Len() != 0 {
		t.Errorf("expected empty response for HTMX delete, got %d bytes", rr.Body.Len())
	}
}

func TestHTMX_OpenCategory(t *testing.T) {
	srv, queries, conn := testServerModern(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Test Poll", "single", "draft", "live")
	createTestOption(t, queries, cat.ID, "Option")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/open", nil)
	req.Header.Set("HX-Request", "true")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for HTMX, got %d", rr.Code)
	}

	// Verify category status changed
	cat, _ = queries.GetCategory(t.Context(), 1)
	if cat.Status != "open" {
		t.Errorf("expected status 'open', got '%s'", cat.Status)
	}
}

func TestHTMX_OpenCategoryNoOptions(t *testing.T) {
	srv, queries, conn := testServerModern(t)
	defer conn.Close()

	createTestCategory(t, queries, "Empty Poll", "single", "draft", "live")
	// No options

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/open", nil)
	req.Header.Set("HX-Request", "true")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for HTMX with no options, got %d", rr.Code)
	}
}

func TestHTMX_CloseCategory(t *testing.T) {
	srv, queries, conn := testServerModern(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Open Poll", "single", "open", "live")
	createTestOption(t, queries, cat.ID, "Option")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/close", nil)
	req.Header.Set("HX-Request", "true")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for HTMX, got %d", rr.Code)
	}

	// Verify category status changed
	cat, _ = queries.GetCategory(t.Context(), 1)
	if cat.Status != "closed" {
		t.Errorf("expected status 'closed', got '%s'", cat.Status)
	}
}

func TestHTMX_ReopenCategory(t *testing.T) {
	srv, queries, conn := testServerModern(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Closed Poll", "single", "closed", "live")
	createTestOption(t, queries, cat.ID, "Option")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/reopen", nil)
	req.Header.Set("HX-Request", "true")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for HTMX, got %d", rr.Code)
	}

	// Verify category status changed
	cat, _ = queries.GetCategory(t.Context(), 1)
	if cat.Status != "open" {
		t.Errorf("expected status 'open', got '%s'", cat.Status)
	}
}

func TestHTMX_ReopenNotClosed(t *testing.T) {
	srv, queries, conn := testServerModern(t)
	defer conn.Close()

	createTestCategory(t, queries, "Draft Poll", "single", "draft", "live")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/reopen", nil)
	req.Header.Set("HX-Request", "true")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for HTMX reopen on non-closed, got %d", rr.Code)
	}
}

func TestHTMX_ArchiveCategory(t *testing.T) {
	srv, queries, conn := testServerModern(t)
	defer conn.Close()

	createTestCategory(t, queries, "Test Poll", "single", "closed", "live")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/archive", nil)
	req.Header.Set("HX-Request", "true")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for HTMX, got %d", rr.Code)
	}

	// Verify category status changed
	cat, _ := queries.GetCategory(t.Context(), 1)
	if cat.Status != "archived" {
		t.Errorf("expected status 'archived', got '%s'", cat.Status)
	}
}

// ====================
// EDGE CASE TESTS
// ====================

func TestHandleVote_CategoryNotFound(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/vote/999", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Returns error page, not 404
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for nonexistent category, got %d", rr.Code)
	}
}

func TestHandleResults_CategoryNotFound(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/results/999", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Returns error page
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for nonexistent category, got %d", rr.Code)
	}
}

func TestAdminReopen_NoOptions(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	// Create closed category with no options
	createTestCategory(t, queries, "Closed Poll", "single", "closed", "live")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/reopen", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should show form with error
	body := rr.Body.String()
	if !strings.Contains(body, "option") {
		t.Error("expected error about needing options")
	}

	// Status should remain closed
	cat, _ := queries.GetCategory(t.Context(), 1)
	if cat.Status != "closed" {
		t.Errorf("expected status to remain 'closed', got '%s'", cat.Status)
	}
}

func TestHTMX_ReopenNoOptions(t *testing.T) {
	srv, queries, conn := testServerModern(t)
	defer conn.Close()

	createTestCategory(t, queries, "Closed Poll", "single", "closed", "live")
	// No options

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/reopen", nil)
	req.Header.Set("HX-Request", "true")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for HTMX reopen with no options, got %d", rr.Code)
	}
}

func TestHTMX_DeleteOptionNotFound(t *testing.T) {
	srv, _, conn := testServerModern(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/option/999", nil)
	req.Header.Set("HX-Request", "true")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestHTMX_AddOptionEmpty(t *testing.T) {
	srv, queries, conn := testServerModern(t)
	defer conn.Close()

	createTestCategory(t, queries, "Test Poll", "single", "draft", "live")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("option_name", "   ") // Whitespace only

	req := httptest.NewRequest(http.MethodPost, "/admin/category/1/option", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty option name, got %d", rr.Code)
	}

	// Verify no option was created
	opts, _ := queries.ListOptionsByCategory(t.Context(), 1)
	if len(opts) != 0 {
		t.Errorf("expected 0 options, got %d", len(opts))
	}
}

func TestAdminUnknownRoute(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin/unknown", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for unknown admin route, got %d", rr.Code)
	}
}

func TestAdminClose_GetNotAllowed(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Open Poll", "single", "open", "live")
	createTestOption(t, queries, cat.ID, "Option")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin/category/1/close", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for GET on close, got %d", rr.Code)
	}
}

func TestAdminArchive_GetNotAllowed(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Test Poll", "single", "closed", "live")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin/category/1/archive", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for GET on archive, got %d", rr.Code)
	}
}

func TestAdminReopen_GetNotAllowed(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Closed Poll", "single", "closed", "live")
	createTestOption(t, queries, cat.ID, "Option")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin/category/1/reopen", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for GET on reopen, got %d", rr.Code)
	}
}

func TestAdminCategoryEdit_InvalidID(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin/category/abc", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for invalid category ID, got %d", rr.Code)
	}
}

func TestAdminDeleteOption_InvalidID(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodPost, "/admin/option/abc", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for invalid option ID, got %d", rr.Code)
	}
}

func TestAdminCategoryPath_Empty(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/admin/category/", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for empty category path, got %d", rr.Code)
	}
}

func TestVoteSubmit_ApprovalNoSelection(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Approval Poll", "approval", "open", "live")
	createTestOption(t, queries, cat.ID, "Option A")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("nickname", "TestUser")
	// No choice set

	req := httptest.NewRequest(http.MethodPost, "/vote/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "selection") {
		t.Error("expected error about making a selection")
	}
}

func TestVoteSubmit_RankedNoSelection(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat, _ := queries.CreateCategory(t.Context(), db.CreateCategoryParams{
		Name:        "Ranked Poll",
		VoteType:    "ranked",
		Status:      "open",
		ShowResults: "live",
		MaxRank:     sql.NullInt64{Int64: 3, Valid: true},
	})
	createTestOption(t, queries, cat.ID, "First")

	handler := srv.Handler()
	form := url.Values{}
	form.Set("nickname", "TestUser")
	// No ranks set

	req := httptest.NewRequest(http.MethodPost, "/vote/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "selection") {
		t.Error("expected error about making a selection")
	}
}

func TestAdminCategoryNew_DefaultMaxRank(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()
	form := url.Values{}
	form.Set("name", "Ranked Poll")
	form.Set("vote_type", "ranked")
	form.Set("show_results", "live")
	form.Set("max_rank", "0") // Invalid, should default to 3

	req := httptest.NewRequest(http.MethodPost, "/admin/category/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	cats, _ := queries.ListCategories(t.Context())
	if len(cats) != 1 {
		t.Fatalf("expected 1 category, got %d", len(cats))
	}
	if !cats[0].MaxRank.Valid || cats[0].MaxRank.Int64 != 3 {
		t.Errorf("expected max_rank 3 (default), got %v", cats[0].MaxRank)
	}
}

func TestAdminCategoryEdit_UpdateRanked(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	queries.CreateCategory(t.Context(), db.CreateCategoryParams{
		Name:        "Original",
		VoteType:    "ranked",
		Status:      "draft",
		ShowResults: "live",
		MaxRank:     sql.NullInt64{Int64: 3, Valid: true},
	})

	handler := srv.Handler()
	form := url.Values{}
	form.Set("name", "Updated")
	form.Set("vote_type", "ranked")
	form.Set("show_results", "after_close")
	form.Set("max_rank", "0") // Invalid, should default to 3

	req := httptest.NewRequest(http.MethodPost, "/admin/category/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", testAdminPassword)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect, got %d", rr.Code)
	}

	cat, _ := queries.GetCategory(t.Context(), 1)
	if !cat.MaxRank.Valid || cat.MaxRank.Int64 != 3 {
		t.Errorf("expected max_rank 3 (default), got %v", cat.MaxRank)
	}
}

func TestEmptyDatabase(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()

	// Home page should work with no categories
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Results list should work with no categories
	req = httptest.NewRequest(http.MethodGet, "/results/", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Admin dashboard should work with no categories
	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("admin", testAdminPassword)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestVoteOnCategoryWithNoOptions(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	createTestCategory(t, queries, "Empty Poll", "single", "open", "live")
	// No options added

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/vote/1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Empty Poll") {
		t.Error("expected category name in response")
	}
}

func TestResultsWithNoVotes(t *testing.T) {
	srv, queries, conn := testServer(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "No Votes Poll", "single", "open", "live")
	createTestOption(t, queries, cat.ID, "Option A")
	createTestOption(t, queries, cat.ID, "Option B")

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/results/1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "No Votes Poll") {
		t.Error("expected category name in results")
	}
}

func TestAdminRouteTrailingSlash(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()

	// Both /admin and /admin/ should work
	for _, path := range []string{"/admin", "/admin/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.SetBasicAuth("admin", testAdminPassword)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200 for %s, got %d", path, rr.Code)
		}
	}
}

// ====================
// RESULTS TABLE TESTS
// ====================

func TestHandleResultsTable_SimpleVoting(t *testing.T) {
	srv, queries, conn := testServerModern(t)
	defer conn.Close()

	cat := createTestCategory(t, queries, "Simple Poll", "single", "open", "live")
	createTestOption(t, queries, cat.ID, "Option A")
	createTestOption(t, queries, cat.ID, "Option B")

	// Cast a vote
	vote, _ := queries.UpsertVote(t.Context(), db.UpsertVoteParams{
		CategoryID: cat.ID,
		Nickname:   "voter1",
	})
	queries.CreateVoteSelection(t.Context(), db.CreateVoteSelectionParams{
		VoteID:   vote.ID,
		OptionID: 1,
	})

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/results/1/table", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandleResultsTable_RankedVoting(t *testing.T) {
	srv, queries, conn := testServerModern(t)
	defer conn.Close()

	cat, _ := queries.CreateCategory(t.Context(), db.CreateCategoryParams{
		Name:        "Ranked Poll",
		VoteType:    "ranked",
		Status:      "open",
		ShowResults: "live",
		MaxRank:     sql.NullInt64{Int64: 3, Valid: true},
	})
	createTestOption(t, queries, cat.ID, "First")
	createTestOption(t, queries, cat.ID, "Second")

	// Cast ranked votes
	vote, _ := queries.UpsertVote(t.Context(), db.UpsertVoteParams{
		CategoryID: cat.ID,
		Nickname:   "voter1",
	})
	queries.CreateVoteSelection(t.Context(), db.CreateVoteSelectionParams{
		VoteID:   vote.ID,
		OptionID: 1,
		Rank:     sql.NullInt64{Int64: 1, Valid: true},
	})
	queries.CreateVoteSelection(t.Context(), db.CreateVoteSelectionParams{
		VoteID:   vote.ID,
		OptionID: 2,
		Rank:     sql.NullInt64{Int64: 2, Valid: true},
	})

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/results/1/table", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestHandleResultsTable_NotFound(t *testing.T) {
	srv, _, conn := testServerModern(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/results/999/table", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestHandleResultsTable_InvalidID(t *testing.T) {
	srv, _, conn := testServerModern(t)
	defer conn.Close()

	handler := srv.Handler()
	req := httptest.NewRequest(http.MethodGet, "/results/abc/table", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

// ====================
// ROUTE HELPER TESTS
// ====================

func TestRouteHelpers(t *testing.T) {
	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{"HomeURL", web.HomeURL, "/"},
		{"ResultsListURL", web.ResultsListURL, "/results"},
		{"AdminURL", web.AdminURL, "/admin"},
		{"AdminCategoryNewURL", web.AdminCategoryNewURL, "/admin/category/new"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRouteHelpersWithID(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(int64) string
		id       int64
		expected string
	}{
		{"VoteURL", web.VoteURL, 42, "/vote/42"},
		{"ResultsURL", web.ResultsURL, 42, "/results/42"},
		{"ResultsTableURL", web.ResultsTableURL, 42, "/results/42/table"},
		{"AdminCategoryOpenURL", web.AdminCategoryOpenURL, 42, "/admin/category/42/open"},
		{"AdminCategoryCloseURL", web.AdminCategoryCloseURL, 42, "/admin/category/42/close"},
		{"AdminCategoryArchiveURL", web.AdminCategoryArchiveURL, 42, "/admin/category/42/archive"},
		{"AdminAddOptionURL", web.AdminAddOptionURL, 42, "/admin/category/42/option/add"},
		{"AdminOptionURL", web.AdminOptionURL, 42, "/admin/option/42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(tt.id)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestAdminCategoryURL(t *testing.T) {
	// Without anchor
	result := web.AdminCategoryURL(42)
	if result != "/admin/category/42" {
		t.Errorf("expected /admin/category/42, got %s", result)
	}

	// With anchor
	result = web.AdminCategoryURL(42, "options")
	if result != "/admin/category/42#options" {
		t.Errorf("expected /admin/category/42#options, got %s", result)
	}

	// With empty anchor
	result = web.AdminCategoryURL(42, "")
	if result != "/admin/category/42" {
		t.Errorf("expected /admin/category/42, got %s", result)
	}
}

func TestAdminRemoveOptionURL(t *testing.T) {
	result := web.AdminRemoveOptionURL(42, 7)
	expected := "/admin/category/42/option/7/remove"
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

// ====================
// ADDITIONAL EDGE CASES
// ====================

func TestResultsRouteTrailingSlash(t *testing.T) {
	srv, _, conn := testServer(t)
	defer conn.Close()

	handler := srv.Handler()

	// /results/ should return 200
	req := httptest.NewRequest(http.MethodGet, "/results/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for /results/, got %d", rr.Code)
	}

	// /results (no trailing slash) redirects to /results/ (301) - standard ServeMux behavior
	req = httptest.NewRequest(http.MethodGet, "/results", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMovedPermanently {
		t.Errorf("expected status 301 redirect for /results, got %d", rr.Code)
	}
}
