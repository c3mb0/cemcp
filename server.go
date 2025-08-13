package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func wrapTextHandler[TArgs any, TResult any](h mcp.StructuredToolHandlerFunc[TArgs, TResult], format func(TResult) string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args TArgs
		if err := req.BindArguments(&args); err != nil {
			return nil, fmt.Errorf("failed to bind arguments: %w", err)
		}
		res, err := h(ctx, req, args)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(format(res)), nil
	}
}

func setupServer(root string) *server.MCPServer {
	s := server.NewMCPServer("fs-mcp-go", "0.1.0")

	readOpts := []mcp.ToolOption{
		mcp.WithDescription("Read a file up to a byte limit. Detects encoding when unspecified."),
		mcp.WithString("path", mcp.Required(), mcp.Description("File path or file:// URI within root")),
		mcp.WithString("encoding", mcp.Enum(string(encText), string(encBase64)), mcp.Description("Force text or base64; auto-detected if empty")),
		mcp.WithNumber("max_bytes", mcp.Min(1), mcp.Description("Maximum bytes to return (default 64 KiB)")),
	}
	if !*compatFlag {
		readOpts = append(readOpts, mcp.WithOutputSchema[ReadResult]())
	}
	readTool := mcp.NewTool("fs_read", readOpts...)
	if *compatFlag {
		s.AddTool(readTool, wrapTextHandler(handleRead(root), formatReadResult))
	} else {
		s.AddTool(readTool, mcp.NewStructuredToolHandler(handleRead(root)))
	}

	peekOpts := []mcp.ToolOption{
		mcp.WithDescription("Read a file window without loading the whole file"),
		mcp.WithString("path", mcp.Required(), mcp.Description("File path")),
		mcp.WithNumber("offset", mcp.Min(0), mcp.Description("Byte offset to start at (default 0)")),
		mcp.WithNumber("max_bytes", mcp.Min(1), mcp.Description("Window size in bytes (default 4 KiB)")),
	}
	if !*compatFlag {
		peekOpts = append(peekOpts, mcp.WithOutputSchema[PeekResult]())
	}
	peekTool := mcp.NewTool("fs_peek", peekOpts...)
	if *compatFlag {
		s.AddTool(peekTool, wrapTextHandler(handlePeek(root), formatPeekResult))
	} else {
		s.AddTool(peekTool, mcp.NewStructuredToolHandler(handlePeek(root)))
	}

	writeOpts := []mcp.ToolOption{
		mcp.WithDescription("Create or modify a file using a strategy"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Target file path")),
		mcp.WithString("encoding", mcp.Required(), mcp.Enum(string(encText), string(encBase64)), mcp.Description("Content encoding: text or base64")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Data to write")),
		mcp.WithString("strategy", mcp.Enum(string(strategyOverwrite), string(strategyNoClobber), string(strategyAppend), string(strategyPrepend), string(strategyReplaceRange)), mcp.Description("Write behavior (default overwrite)")),
		mcp.WithBoolean("create_dirs", mcp.Description("Create parent directories if needed (default false)")),
		mcp.WithString("mode", mcp.Pattern("^0?[0-7]{3,4}$"), mcp.Description("File mode in octal; omit to keep existing")),
		mcp.WithNumber("start", mcp.Min(0), mcp.Description("Start byte for replace_range")),
		mcp.WithNumber("end", mcp.Min(0), mcp.Description("End byte (exclusive) for replace_range")),
	}
	if !*compatFlag {
		writeOpts = append(writeOpts, mcp.WithOutputSchema[WriteResult]())
	}
	writeTool := mcp.NewTool("fs_write", writeOpts...)
	if *compatFlag {
		s.AddTool(writeTool, wrapTextHandler(handleWrite(root), formatWriteResult))
	} else {
		s.AddTool(writeTool, mcp.NewStructuredToolHandler(handleWrite(root)))
	}

	editOpts := []mcp.ToolOption{
		mcp.WithDescription("Search and replace text in a file"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Target text file")),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("Substring or regex to match")),
		mcp.WithString("replace", mcp.Required(), mcp.Description("Replacement text; supports $1 etc. in regex mode")),
		mcp.WithBoolean("regex", mcp.Description("Treat pattern as a regular expression")),
		mcp.WithNumber("count", mcp.Min(0), mcp.Description("If >0, maximum replacements; 0 replaces all")),
	}
	if !*compatFlag {
		editOpts = append(editOpts, mcp.WithOutputSchema[EditResult]())
	}
	editTool := mcp.NewTool("fs_edit", editOpts...)
	if *compatFlag {
		s.AddTool(editTool, wrapTextHandler(handleEdit(root), formatEditResult))
	} else {
		s.AddTool(editTool, mcp.NewStructuredToolHandler(handleEdit(root)))
	}

	listOpts := []mcp.ToolOption{
		mcp.WithDescription("List directory contents"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Directory to list")),
		mcp.WithBoolean("recursive", mcp.Description("Recurse into subdirectories")),
		mcp.WithNumber("max_entries", mcp.Min(1), mcp.Description("Maximum entries to return (default 1000)")),
	}
	if !*compatFlag {
		listOpts = append(listOpts, mcp.WithOutputSchema[ListResult]())
	}
	listTool := mcp.NewTool("fs_list", listOpts...)
	if *compatFlag {
		s.AddTool(listTool, wrapTextHandler(handleList(root), formatListResult))
	} else {
		s.AddTool(listTool, mcp.NewStructuredToolHandler(handleList(root)))
	}

	searchOpts := []mcp.ToolOption{
		mcp.WithDescription("Search files recursively for text using concurrent workers"),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("Substring or regex to find")),
		mcp.WithString("path", mcp.Description("Start directory relative to root")),
		mcp.WithBoolean("regex", mcp.Description("Interpret pattern as regular expression")),
		mcp.WithNumber("max_results", mcp.Min(1), mcp.Description("Maximum matches to return (default 100)")),
	}
	if !*compatFlag {
		searchOpts = append(searchOpts, mcp.WithOutputSchema[SearchResult]())
	}
	searchTool := mcp.NewTool("fs_search", searchOpts...)
	if *compatFlag {
		s.AddTool(searchTool, wrapTextHandler(handleSearch(root), formatSearchResult))
	} else {
		s.AddTool(searchTool, mcp.NewStructuredToolHandler(handleSearch(root)))
	}

	globOpts := []mcp.ToolOption{
		mcp.WithDescription("Match paths with shell-style globbing and ** for recursion"),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("Glob pattern relative to root")),
		mcp.WithNumber("max_results", mcp.Min(1), mcp.Description("Maximum matches to return (default 1000)")),
	}
	if !*compatFlag {
		globOpts = append(globOpts, mcp.WithOutputSchema[GlobResult]())
	}
	globTool := mcp.NewTool("fs_glob", globOpts...)
	if *compatFlag {
		s.AddTool(globTool, wrapTextHandler(handleGlob(root), formatGlobResult))
	} else {
		s.AddTool(globTool, mcp.NewStructuredToolHandler(handleGlob(root)))
	}

	return s
}
