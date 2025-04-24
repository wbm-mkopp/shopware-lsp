# Tree-Sitter Pattern Matcher

This package provides a powerful, composable pattern matching system for Tree-Sitter nodes. Inspired by JetBrains IntelliJ IDEA's [ElementPatterns](https://plugins.jetbrains.com/docs/intellij/element-patterns.html), it allows for declarative pattern matching against syntax trees.

## Basic Usage

```go
import (
    "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

// Create a pattern
methodCallPattern := treesitterhelper.And(
    treesitterhelper.NodeKind("member_call_expression"),
    treesitterhelper.HasChild(treesitterhelper.And(
        treesitterhelper.NodeKind("name"),
        treesitterhelper.NodeText("renderStorefront"),
    )),
)

// Match a node against the pattern
if methodCallPattern.Matches(node, content) {
    // Node matches the pattern
}

// Find all matching nodes in a syntax tree
matches := treesitterhelper.FindAll(rootNode, methodCallPattern, content)
```

## Core Concepts

### Patterns

Patterns are composable predicates that can be applied to Tree-Sitter nodes. All patterns implement the `Pattern` interface:

```go
type Pattern interface {
    Matches(node *tree_sitter.Node, content []byte) bool
}
```

### Pattern Types

#### Basic Patterns

- `NodeKind(kind string)`: Matches nodes with a specific kind
- `AnyNodeKind(kinds ...string)`: Matches nodes with any of the specified kinds
- `NodeText(text string)`: Matches nodes with exact text content
- `NodeTextContains(substring string)`: Matches nodes containing specific text

#### Relationship Patterns

- `ParentOfKind(kind string, level int)`: Matches nodes with a parent of specific kind at a given level
- `HasChildOfKind(kind string)`: Matches nodes that have a child of a specific kind
- `HasChild(pattern Pattern)`: Matches nodes that have a child matching a pattern
- `NamedChild(index uint, pattern Pattern)`: Matches nodes whose child at the given index matches a pattern
- `ChildSequence(patterns ...Pattern)`: Matches nodes whose children match a sequence of patterns
- `Ancestor(pattern Pattern, maxDepth int)`: Matches nodes with an ancestor matching a pattern within a depth limit

#### Logical Operators

- `And(patterns ...Pattern)`: Matches nodes that match all patterns
- `Or(patterns ...Pattern)`: Matches nodes that match any pattern
- `Not(pattern Pattern)`: Matches nodes that don't match the pattern

#### Capture Patterns

- `Capture(name string, pattern Pattern)`: Captures a node for later retrieval if it matches a pattern

## Example Patterns

### PHP Method Call Pattern

The old manual checking approach:

```go
func IsPHPRenderStorefrontCall(node *tree_sitter.Node, content []byte) bool {
    if node.Kind() != "string_content" {
        return false
    }

    methodCall := node.Parent().Parent().Parent().Parent()

    if methodCall.Kind() != "member_call_expression" {
        return false
    }

    nameNode := GetFirstNodeOfKind(methodCall, "name")

    if nameNode == nil {
        return false
    }

    if string(nameNode.Utf8Text(content)) != "renderStorefront" {
        return false
    }

    return true
}
```

Using the pattern matcher:

```go
func IsPHPRenderStorefrontCall(node *tree_sitter.Node, content []byte) bool {
    pattern := treesitterhelper.And(
        treesitterhelper.NodeKind("string_content"),
        treesitterhelper.ParentOfKind("member_call_expression", 4),
        treesitterhelper.Ancestor(
            treesitterhelper.And(
                treesitterhelper.NodeKind("member_call_expression"),
                treesitterhelper.HasChild(treesitterhelper.And(
                    treesitterhelper.NodeKind("name"),
                    treesitterhelper.NodeText("renderStorefront"),
                )),
            ),
            4,
        ),
    )
    
    return pattern.Matches(node, content)
}
```

### Twig Tag Pattern

Matching Twig tags and strings within them:

```go
// Find all extends tags in a template
extendsTags := treesitterhelper.FindAll(rootNode, treesitterhelper.TwigTagPattern("extends"), content)

// Find all template paths in extends and sw_extends tags
extendsStringNodes := treesitterhelper.FindAll(rootNode, 
    treesitterhelper.TwigStringInTagPattern("extends", "sw_extends"), content)

// Check if a string node is inside a specific tag type
if treesitterhelper.IsTwigTag(stringNode, content, "extends", "sw_extends") {
    // Process template path
}
```

The pattern matcher handles both direct tag types (like "extends" and "include") and keyword-based tags (like "sw_extends" and "block"):

```go
// Check if a string is inside a block or include tag
isBlockOrInclude := treesitterhelper.IsTwigTag(node, content, "block", "include")
```

### XML Service Pattern

Finding XML service nodes with a specific ID:

```go
// Create a pattern for services with a specific ID
servicePattern := treesitterhelper.And(
    treesitterhelper.NodeKind("element"),
    treesitterhelper.HasChild(treesitterhelper.And(
        treesitterhelper.NodeKind("tag_name"),
        treesitterhelper.NodeText("service"),
    )),
    treesitterhelper.HasChild(treesitterhelper.And(
        treesitterhelper.NodeKind("attribute"),
        treesitterhelper.HasChild(treesitterhelper.And(
            treesitterhelper.NodeKind("attribute_name"),
            treesitterhelper.NodeText("id"),
        )),
        treesitterhelper.HasChild(treesitterhelper.And(
            treesitterhelper.NodeKind("attribute_value"),
            treesitterhelper.NodeText("my.service.id"),
        )),
    )),
)
```

### Capturing Nodes

Extracting the name of a Twig block:

```go
func GetTwigBlockName(blockNode *tree_sitter.Node, content []byte) string {
    // Create a capture for the string node
    nameCapture := treesitterhelper.Capture("name", treesitterhelper.NodeKind("string"))
    
    // Create a pattern that combines the block pattern with the capture
    blockWithNamePattern := treesitterhelper.And(
        treesitterhelper.TwigBlockPattern,
        treesitterhelper.HasChild(nameCapture),
    )
    
    // Match the pattern and extract the captured node
    if blockWithNamePattern.Matches(blockNode, content) {
        nameNode := nameCapture.GetCapturedNode()
        if nameNode != nil {
            return string(nameNode.Utf8Text(content))
        }
    }
    
    return ""
}
```

## Searching Trees

The library provides two functions for finding nodes in a syntax tree:

- `FindFirst(root *tree_sitter.Node, pattern Pattern, content []byte) *tree_sitter.Node`: Returns the first node matching a pattern
- `FindAll(root *tree_sitter.Node, pattern Pattern, content []byte) []*tree_sitter.Node`: Returns all nodes matching a pattern

## Common Patterns Library

The package includes a set of predefined patterns for common structures:

```go
// PHP patterns
PHPMethodCallPattern("methodName")
PHPStringLiteralPattern
PHPRenderStorefrontCallPattern

// XML patterns
XMLServicePattern
XMLServiceWithIdPattern("serviceId")

// Twig patterns
TwigBlockPattern
TwigExtendsPattern
```

## Helper Functions

The package also includes helper functions that build on these patterns:

```go
// Check if a node is an XML service with a specific ID
IsXMLServiceWithID(node, content, "my.service.id")

// Find all block nodes in a Twig document
blocks := FindAllTwigBlocks(rootNode, content)

// Get the name of a Twig block
blockName := GetTwigBlockName(blockNode, content)
```