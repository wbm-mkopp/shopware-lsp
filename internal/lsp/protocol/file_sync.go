package protocol

// FileOperationRegistrationOptions represents the options for registering file operations
type FileOperationRegistrationOptions struct {
	Filters []FileOperationFilter `json:"filters"`
}

// FileOperationFilter represents a filter for file operations
type FileOperationFilter struct {
	Scheme string           `json:"scheme,omitempty"`
	Pattern FileOperationPattern `json:"pattern"`
}

// FileOperationPattern represents a pattern for file operations
type FileOperationPattern struct {
	Glob    string `json:"glob"`
	Matches string `json:"matches,omitempty"`
	Options struct {
		IgnoreCase bool `json:"ignoreCase,omitempty"`
	} `json:"options,omitempty"`
}

// FileEvent represents a file event
type FileEvent struct {
	URI  string `json:"uri"`
	Type int    `json:"type"`
}

// FileChangeType represents the type of file change
type FileChangeType int

const (
	// FileCreated represents a file creation event
	FileCreated FileChangeType = 1
	// FileChanged represents a file change event
	FileChanged FileChangeType = 2
	// FileDeleted represents a file deletion event
	FileDeleted FileChangeType = 3
)

// DidChangeWatchedFilesParams represents the parameters for a didChangeWatchedFiles notification
type DidChangeWatchedFilesParams struct {
	Changes []FileEvent `json:"changes"`
}

// DidChangeWatchedFilesRegistrationOptions represents the options for registering file watchers
type DidChangeWatchedFilesRegistrationOptions struct {
	Watchers []FileSystemWatcher `json:"watchers"`
}

// FileSystemWatcher represents a file system watcher
type FileSystemWatcher struct {
	GlobPattern string `json:"globPattern"`
	Kind        int    `json:"kind,omitempty"`
}

// WatchKind represents the kind of file watching
type WatchKind int

const (
	// WatchCreate represents watching for file creation
	WatchCreate WatchKind = 1
	// WatchChange represents watching for file changes
	WatchChange WatchKind = 2
	// WatchDelete represents watching for file deletion
	WatchDelete WatchKind = 4
)

// FileOperationOptions represents the options for file operations
type FileOperationOptions struct {
	DidCreate  *FileOperationRegistrationOptions `json:"didCreate,omitempty"`
	WillCreate *FileOperationRegistrationOptions `json:"willCreate,omitempty"`
	DidRename  *FileOperationRegistrationOptions `json:"didRename,omitempty"`
	WillRename *FileOperationRegistrationOptions `json:"willRename,omitempty"`
	DidDelete  *FileOperationRegistrationOptions `json:"didDelete,omitempty"`
	WillDelete *FileOperationRegistrationOptions `json:"willDelete,omitempty"`
}

// CreateFilesParams represents the parameters for a workspace/willCreateFiles request
type CreateFilesParams struct {
	Files []FileCreate `json:"files"`
}

// FileCreate represents a file creation operation
type FileCreate struct {
	URI string `json:"uri"`
}

// RenameFilesParams represents the parameters for a workspace/willRenameFiles request
type RenameFilesParams struct {
	Files []FileRename `json:"files"`
}

// FileRename represents a file rename operation
type FileRename struct {
	OldURI string `json:"oldUri"`
	NewURI string `json:"newUri"`
}

// DeleteFilesParams represents the parameters for a workspace/willDeleteFiles request
type DeleteFilesParams struct {
	Files []FileDelete `json:"files"`
}

// FileDelete represents a file deletion operation
type FileDelete struct {
	URI string `json:"uri"`
}
