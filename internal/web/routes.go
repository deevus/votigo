// internal/web/routes.go
package web

import "fmt"

// Route pattern constants
const (
	PathHome        = "/"
	PathVote        = "/vote/%d"
	PathResults     = "/results/%d"
	PathResultsList = "/results"
	PathResultsTable = "/results/%d/table"

	PathAdmin            = "/admin"
	PathAdminCategory    = "/admin/category/%d"
	PathAdminCategoryNew = "/admin/category/new"
	PathAdminCategoryOpen = "/admin/category/%d/open"
	PathAdminCategoryClose = "/admin/category/%d/close"
	PathAdminCategoryArchive = "/admin/category/%d/archive"
	PathAdminAddOption   = "/admin/category/%d/option/add"
	PathAdminRemoveOption = "/admin/category/%d/option/%d/remove"
	PathAdminOption      = "/admin/option/%d"
)

// Type-safe URL builders
func HomeURL() string {
	return PathHome
}

func VoteURL(categoryID int64) string {
	return fmt.Sprintf(PathVote, categoryID)
}

func ResultsURL(categoryID int64) string {
	return fmt.Sprintf(PathResults, categoryID)
}

func ResultsListURL() string {
	return PathResultsList
}

func ResultsTableURL(categoryID int64) string {
	return fmt.Sprintf(PathResultsTable, categoryID)
}

func AdminURL() string {
	return PathAdmin
}

func AdminCategoryURL(categoryID int64) string {
	return fmt.Sprintf(PathAdminCategory, categoryID)
}

func AdminCategoryNewURL() string {
	return PathAdminCategoryNew
}

func AdminCategoryOpenURL(categoryID int64) string {
	return fmt.Sprintf(PathAdminCategoryOpen, categoryID)
}

func AdminCategoryCloseURL(categoryID int64) string {
	return fmt.Sprintf(PathAdminCategoryClose, categoryID)
}

func AdminCategoryArchiveURL(categoryID int64) string {
	return fmt.Sprintf(PathAdminCategoryArchive, categoryID)
}

func AdminAddOptionURL(categoryID int64) string {
	return fmt.Sprintf(PathAdminAddOption, categoryID)
}

func AdminRemoveOptionURL(categoryID int64, optionID int64) string {
	return fmt.Sprintf(PathAdminRemoveOption, categoryID, optionID)
}

func AdminOptionURL(optionID int64) string {
	return fmt.Sprintf(PathAdminOption, optionID)
}
