module github.com/shopware/shopware-lsp

go 1.24

replace github.com/tree-sitter-grammars/tree-sitter-xml => github.com/justinMBullard/tree-sitter-xml v0.0.0-20250305015746-03d1af911bbd

require (
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/sourcegraph/jsonrpc2 v0.2.0
	github.com/stretchr/testify v1.10.0
	github.com/tree-sitter-grammars/tree-sitter-xml v0.7.0
	github.com/tree-sitter/go-tree-sitter v0.25.0
	github.com/tree-sitter/tree-sitter-php v0.23.12
	go.etcd.io/bbolt v1.4.0
	golang.org/x/sync v0.13.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
