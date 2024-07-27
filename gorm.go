package apirouter

import "gorm.io/gorm"

// Paginate applies standard page_no & results_per_page rules on a gorm tx as a scope
func (ctx *Context) Paginate(resultsPerPage int) func(tx *gorm.DB) *gorm.DB {
	// This "return a function" process is required by gorm to have this working as a scope
	//
	// See: https://gorm.io/docs/scopes.html
	return func(tx *gorm.DB) *gorm.DB {
		page, _ := GetParam[int](ctx, "page_no")
		if page < 1 {
			page = 1
		}

		if resultsPerPage <= 0 {
			resultsPerPage = 25
		}
		if rpp, ok := GetParam[int](ctx, "results_per_page"); ok && rpp > 0 && rpp <= 100 {
			resultsPerPage = rpp
		}

		tx = tx.Offset((page - 1) * resultsPerPage).Limit(resultsPerPage)

		return tx
	}
}
