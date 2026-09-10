package indexer

import (
	"context"
	"path/filepath"
)

// ResourcePaths queries the scanner's persisted path index. No source files or
// directories are read, and range bounds use the primary-key index.
func (fs *FileScanner) ResourcePaths(ctx context.Context, directory string, limit int) ([]string, error) {
	if fs == nil || limit <= 0 {
		return nil, nil
	}
	prefix := filepath.Clean(directory) + string(filepath.Separator)
	upper := prefix[:len(prefix)-1] + string(filepath.Separator+1)
	rows, err := fs.db.QueryContext(ctx, "SELECT path FROM file_hashes WHERE path >= ? AND path < ? ORDER BY path LIMIT ?", prefix, upper, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}
