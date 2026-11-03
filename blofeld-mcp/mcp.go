package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	_ "embed"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func runMCP(blo *Blofeld) {

	s := server.NewMCPServer(
		"Blofeld MCP",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	docTool := mcp.NewTool("blofeld_describe-sysex",
		mcp.WithDescription("Returns the SysEx implementation description for the Blofeld synthesizer."),
	)

	s.AddTool(docTool, docToolHandler)

	getPatchTool := mcp.NewTool("blofeld_get-patch",
		mcp.WithDescription("Retrieves a patch from the Blofeld synthesizer."),
		mcp.WithString("bank", mcp.Required(), mcp.Description("The bank of the patch (e.g., A, B, ..., H).")),
		mcp.WithNumber("program", mcp.Required(), mcp.Description("The program number of the patch (1-128).")),
	)
	s.AddTool(getPatchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var bank string
		var program int

		log.Println("[mcp]Handling get patch request.")

		bank, err := request.RequireString("bank")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		program, err = request.RequireInt("program")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		patch, _, err := blo.RequestPatchDump(bank, program)
		if err != nil {
			return nil, fmt.Errorf("failed to read patch: %v", err)
		}

		asJson, err := json.MarshalIndent(&patch, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal patch to JSON: %v", err)
		}

		return mcp.NewToolResultText(string(asJson)), nil
	})

	sendPatchTool := mcp.NewTool("blofeld_send-patch",
		mcp.WithDescription("Sends a patch to the Blofeld synthesizer."),
		mcp.WithString("bank", mcp.Required(), mcp.Description("The bank of the patch (e.g., A, B, ..., H).")),
		mcp.WithNumber("program", mcp.Required(), mcp.Description("The program number of the patch (1-128).")),
		mcp.WithString("patch-json", mcp.Required(), mcp.Description("The patch data in JSON format. The JSON must conform to the Patch structure.")),
	)
	s.AddTool(sendPatchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var bank string
		var program int
		var patchJson string

		log.Println("[mcp]Handling send patch request.")

		bank, err := request.RequireString("bank")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		program, err = request.RequireInt("program")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		patchJson, err = request.RequireString("patch-json")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		log.Println("[mcp] Sending patch to Blofeld. Bank:", bank, "Program:", program, "JSON:", patchJson)

		var patch Patch
		if err := json.Unmarshal([]byte(patchJson), &patch); err != nil {
			return nil, fmt.Errorf("failed to unmarshal patch JSON: %v", err)
		}

		if err := blo.SendPatch(bank, program, &patch); err != nil {
			return nil, fmt.Errorf("failed to send patch: %v", err)
		}

		return mcp.NewToolResultText("Patch sent successfully."), nil
	})

	playNotesTool := mcp.NewTool("blofeld_play-test-notes",
		mcp.WithDescription("Plays test notes on the Blofeld synthesizer."),
	)
	s.AddTool(playNotesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := playTestNotes(blo); err != nil {
			return nil, fmt.Errorf("failed to play test notes: %v", err)
		}
		return mcp.NewToolResultText("Test notes played successfully."), nil
	})

	minor7Tool := mcp.NewTool("blofeld_play-minor7",
		mcp.WithDescription("Plays a C minor 7 chord on the Blofeld."),
	)
	s.AddTool(minor7Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := playMinor7Chord(blo); err != nil {
			return nil, fmt.Errorf("failed to play minor 7 chord: %v", err)
		}
		return mcp.NewToolResultText("C minor 7 chord played successfully."), nil
	})

	playTextNotesTool := mcp.NewTool("blofeld_play-notes-text",
		mcp.WithDescription("Plays a melody from a text list of notes (e.g., \"C4 Eb4 G4 Bb4\"), supports rests."),
		mcp.WithString("notes", mcp.Required(), mcp.Description("Space/comma/semicolon-separated notes in scientific pitch (e.g., C4, D#5, Bb3) and rests (R or rest).")),
	)
	s.AddTool(playTextNotesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		notesText, err := request.RequireString("notes")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := playNotesFromText(blo, notesText); err != nil {
			return nil, fmt.Errorf("failed to play notes: %v", err)
		}
		return mcp.NewToolResultText(fmt.Sprintf("Played notes: %s", notesText)), nil
	})

	log.Println("Starting Blofeld MCP server...")

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}

}

//go:embed waldorf_blofeld_sysex_documentation_v.1.04.txt
var sysexDoc string

func docToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Println("[mcp]Handling SysEx documentation request.")

	return mcp.NewToolResultText(string(sysexDoc)), nil
}
