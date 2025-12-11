package apirouter

import "gorm.io/gorm"

// Paginate returns a GORM scope function that applies pagination to database queries.
// It reads "page_no" and "results_per_page" from the request parameters.
//
// Parameters:
//   - page_no: The page number (1-indexed, defaults to 1)
//   - results_per_page: Number of results per page (defaults to resultsPerPage argument, max 100)
//
// The resultsPerPage argument sets the default page size when not specified in the request.
// If resultsPerPage <= 0, it defaults to 25.
//
// Example usage:
//
//	var users []User
//	db.Scopes(ctx.Paginate(25)).Find(&users)
func (ctx *Context) Paginate(resultsPerPage int) func(tx *gorm.DB) *gorm.DB {
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
