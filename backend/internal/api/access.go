package api

import (
	"database/sql"
	"strings"
)

// libAccess resolves which libraries a user may see.
// unrestricted == true means "no filtering" (admin, or a user that has not been
// limited to specific libraries).
func libAccess(db *sql.DB, uid string) (unrestricted bool, paths []string) {
	if uid == "" {
		return false, nil
	}
	var isAdmin, restricted int
	err := db.QueryRow(
		"SELECT is_admin, COALESCE(restricted_libraries,0) FROM users WHERE id=?", uid,
	).Scan(&isAdmin, &restricted)
	if err != nil {
		return false, nil
	}
	if isAdmin == 1 || restricted == 0 {
		return true, nil
	}
	rows, err := db.Query(`
		SELECT l.path FROM libraries l
		JOIN user_libraries ul ON ul.library_id = l.id
		WHERE ul.user_id = ?
	`, uid)
	if err != nil {
		return false, nil
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			continue
		}
		paths = append(paths, p)
	}
	return false, paths
}

// pathIn reports whether an absolute path sits inside one of the allowed roots.
func pathIn(path string, roots []string) bool {
	for _, r := range roots {
		if r != "" && (path == r || strings.HasPrefix(path, r+"/")) {
			return true
		}
	}
	return false
}

// canAccessPath is the single-path check (used for detail/stream/thumb lookups
// where a folder id is not available).
func canAccessPath(db *sql.DB, uid, path string) bool {
	unrestricted, roots := libAccess(db, uid)
	if unrestricted {
		return true
	}
	return pathIn(path, roots)
}

// pathFilter builds a SQL predicate on col limiting rows to the libraries the
// user may access. An empty expression means no restriction applies.
func pathFilter(db *sql.DB, uid, col string) (string, []any) {
	unrestricted, roots := libAccess(db, uid)
	if unrestricted {
		return "", nil
	}
	if len(roots) == 0 {
		return "0", nil
	}
	parts := make([]string, 0, len(roots))
	args := make([]any, 0, len(roots)*2)
	for _, r := range roots {
		parts = append(parts, col+" = ?", col+" LIKE ?")
		args = append(args, r, r+"/%")
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}
