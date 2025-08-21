package main

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func wrapTextHandler[TArgs any, TResult any](h mcp.StructuredToolHandlerFunc[TArgs, TResult], format func(TResult) string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args TArgs
		if err := req.BindArguments(&args); err != nil {
			errResp := toErrorResponse(err)
			out := mcp.NewToolResultStructured(errResp, errResp.Error)
			out.IsError = true
			return out, nil
		}
		res, err := h(ctx, req, args)
		if err != nil {
			errResp := toErrorResponse(err)
			out := mcp.NewToolResultStructured(errResp, errResp.Error)
			out.IsError = true
			return out, nil
		}
		return mcp.NewToolResultText(format(res)), nil
	}
}

func wrapStructuredHandler[TArgs any, TResult any](h mcp.StructuredToolHandlerFunc[TArgs, TResult]) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args TArgs
		if err := req.BindArguments(&args); err != nil {
			errResp := toErrorResponse(err)
			out := mcp.NewToolResultStructured(errResp, errResp.Error)
			out.IsError = true
			return out, nil
		}
		res, err := h(ctx, req, args)
		if err != nil {
			errResp := toErrorResponse(err)
			out := mcp.NewToolResultStructured(errResp, errResp.Error)
			out.IsError = true
			return out, nil
		}
		return &mcp.CallToolResult{StructuredContent: res}, nil
	}
}

func setupServer(root string) *server.MCPServer {
	s := server.NewMCPServer("fs-mcp-go", "0.1.0")
	server.WithToolHandlerMiddleware(sessionMiddleware())(s)

	readOpts := []mcp.ToolOption{
		mcp.WithDescription("Read a file up to a byte limit."),
		mcp.WithString("path", mcp.Required(), mcp.Description("File path or file:// URI within base folder")),
		mcp.WithNumber("max_bytes", mcp.Min(1), mcp.Description("Maximum bytes to return")),
	}
	if !*compatFlag {
		readOpts = append(readOpts, mcp.WithOutputSchema[ReadResult]())
	}
	readTool := mcp.NewTool("fs_read", readOpts...)
	if *compatFlag {
		s.AddTool(readTool, wrapTextHandler(handleRead(root), formatReadResult))
	} else {
		s.AddTool(readTool, wrapStructuredHandler(handleRead(root)))
	}

	peekOpts := []mcp.ToolOption{
		mcp.WithDescription("Read a file window without loading the whole file"),
		mcp.WithString("path", mcp.Required(), mcp.Description("File path")),
		mcp.WithNumber("offset", mcp.Min(0), mcp.Description("Byte offset to start at")),
		mcp.WithNumber("max_bytes", mcp.Min(1), mcp.Description("Window size in bytes")),
	}
	if !*compatFlag {
		peekOpts = append(peekOpts, mcp.WithOutputSchema[PeekResult]())
	}
	peekTool := mcp.NewTool("fs_peek", peekOpts...)
	if *compatFlag {
		s.AddTool(peekTool, wrapTextHandler(handlePeek(root), formatPeekResult))
	} else {
		s.AddTool(peekTool, wrapStructuredHandler(handlePeek(root)))
	}

	writeOpts := []mcp.ToolOption{
		mcp.WithDescription("Create or modify a file with a strategy"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Target file path")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Data to write")),
		mcp.WithString("strategy", mcp.Enum(string(strategyOverwrite), string(strategyNoClobber), string(strategyAppend), string(strategyPrepend), string(strategyReplaceRange)), mcp.Description("Write strategy: overwrite, no_clobber, append, prepend, replace_range")),
		mcp.WithString("mode", mcp.Pattern("^0?[0-7]{3,4}$"), mcp.Description("File mode in octal, keep existing if omitted")),
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
		s.AddTool(writeTool, wrapStructuredHandler(handleWrite(root)))
	}

	editOpts := []mcp.ToolOption{
		mcp.WithDescription("Search and replace text in a file"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Target text file")),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("Substring or regex to match")),
		mcp.WithString("replace", mcp.Required(), mcp.Description("Replacement text; $1 etc. works in regex mode")),
		mcp.WithBoolean("regex", mcp.Description("Treat pattern as a regular expression")),
		mcp.WithNumber("count", mcp.Min(0), mcp.Description("Maximum replacements; 0 means all")),
	}
	if !*compatFlag {
		editOpts = append(editOpts, mcp.WithOutputSchema[EditResult]())
	}
	editTool := mcp.NewTool("fs_edit", editOpts...)
	if *compatFlag {
		s.AddTool(editTool, wrapTextHandler(handleEdit(root), formatEditResult))
	} else {
		s.AddTool(editTool, wrapStructuredHandler(handleEdit(root)))
	}

	listOpts := []mcp.ToolOption{
		mcp.WithDescription("List directory contents"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Directory to list")),
		mcp.WithBoolean("recursive", mcp.Description("Recurse into subdirectories")),
		mcp.WithNumber("max_entries", mcp.Min(1), mcp.Description("Maximum entries to return")),
	}
	if !*compatFlag {
		listOpts = append(listOpts, mcp.WithOutputSchema[ListResult]())
	}
	listTool := mcp.NewTool("fs_list", listOpts...)
	if *compatFlag {
		s.AddTool(listTool, wrapTextHandler(handleList(root), formatListResult))
	} else {
		s.AddTool(listTool, wrapStructuredHandler(handleList(root)))
	}

	searchOpts := []mcp.ToolOption{
		mcp.WithDescription("Search files recursively for text"),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("Substring or regex to find")),
		mcp.WithString("path", mcp.Description("Start directory relative to base folder")),
		mcp.WithBoolean("regex", mcp.Description("Interpret pattern as regular expression")),
		mcp.WithNumber("max_results", mcp.Min(1), mcp.Description("Maximum matches to return")),
	}
	if !*compatFlag {
		searchOpts = append(searchOpts, mcp.WithOutputSchema[SearchResult]())
	}
	searchTool := mcp.NewTool("fs_search", searchOpts...)
	if *compatFlag {
		s.AddTool(searchTool, wrapTextHandler(handleSearch(root), formatSearchResult))
	} else {
		s.AddTool(searchTool, wrapStructuredHandler(handleSearch(root)))
	}

	globOpts := []mcp.ToolOption{
		mcp.WithDescription("Match paths using shell-style globbing; ** enables recursion"),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("Glob pattern relative to base folder")),
		mcp.WithNumber("max_results", mcp.Min(1), mcp.Description("Maximum matches to return")),
	}
	if !*compatFlag {
		globOpts = append(globOpts, mcp.WithOutputSchema[GlobResult]())
	}
	globTool := mcp.NewTool("fs_glob", globOpts...)
	if *compatFlag {
		s.AddTool(globTool, wrapTextHandler(handleGlob(root), formatGlobResult))
	} else {
		s.AddTool(globTool, wrapStructuredHandler(handleGlob(root)))
	}

	mkdirOpts := []mcp.ToolOption{
		mcp.WithDescription("Create a directory"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Directory path to create")),
		mcp.WithString("mode", mcp.Pattern("^0?[0-7]{3,4}$"), mcp.Description("Directory mode in octal")),
	}
	if !*compatFlag {
		mkdirOpts = append(mkdirOpts, mcp.WithOutputSchema[MkdirResult]())
	}
	mkdirTool := mcp.NewTool("fs_mkdir", mkdirOpts...)
	if *compatFlag {
		s.AddTool(mkdirTool, wrapTextHandler(handleMkdir(root), formatMkdirResult))
	} else {
		s.AddTool(mkdirTool, wrapStructuredHandler(handleMkdir(root)))
	}

	rmdirOpts := []mcp.ToolOption{
		mcp.WithDescription("Remove a directory"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Directory to remove")),
		mcp.WithBoolean("recursive", mcp.Description("Remove directory contents recursively")),
	}
	if !*compatFlag {
		rmdirOpts = append(rmdirOpts, mcp.WithOutputSchema[RmdirResult]())
	}
	rmdirTool := mcp.NewTool("fs_rmdir", rmdirOpts...)
	if *compatFlag {
		s.AddTool(rmdirTool, wrapTextHandler(handleRmdir(root), formatRmdirResult))
	} else {
		s.AddTool(rmdirTool, wrapStructuredHandler(handleRmdir(root)))
	}

	dbgApproachOpts := []mcp.ToolOption{
		mcp.WithDescription("Record debugging approach and resolution"),
		mcp.WithString("session", mcp.Description("Existing session identifier")),
		mcp.WithString("approach", mcp.Required(), mcp.Description("Debugging steps taken")),
		mcp.WithString("resolution", mcp.Description("Outcome or fix applied")),
	}
	if !*compatFlag {
		dbgApproachOpts = append(dbgApproachOpts, mcp.WithOutputSchema[DebuggingApproachResult]())
	}
	dbgApproachTool := mcp.NewTool("debuggingapproach", dbgApproachOpts...)
	s.AddTool(dbgApproachTool, wrapStructuredHandler(handleDebuggingApproach()))

	pendingOpts := []mcp.ToolOption{
		mcp.WithDescription("List unresolved debugging sessions"),
	}
	if !*compatFlag {
		pendingOpts = append(pendingOpts, mcp.WithOutputSchema[PendingDebugResult]())
	}
	pendingTool := mcp.NewTool("pendingdebug", pendingOpts...)
	s.AddTool(pendingTool, wrapStructuredHandler(handlePendingDebug()))

	return s
}
