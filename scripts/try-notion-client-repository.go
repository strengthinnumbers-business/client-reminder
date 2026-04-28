//go:build ignore

// Manually exercises internal/adapters/client/notion against the real Notion API.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kong"
	clientnotion "github.com/strengthinnumbers-business/client-reminder/internal/adapters/client/notion"
	"github.com/strengthinnumbers-business/client-reminder/internal/adapters/notionapi"
)

type cli struct {
	NotionAPIKey string `name:"notion-api-key" help:"Notion API key. Falls back to NOTION_API_KEY env var when absent."`
	DataSourceID string `name:"data-source-id" required:"" help:"Notion data source ID containing client configuration."`
}

func main() {
	if err := run(os.Stdout, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "try-notion-client-repository: %v\n", err)
		os.Exit(2)
	}
}

func run(out io.Writer, args []string) error {
	var commandLine cli
	parser, err := kong.New(&commandLine,
		kong.Name("try-notion-client-repository"),
		kong.Description("Manual test helper for the Notion-backed client repository."),
		kong.UsageOnError(),
	)
	if err != nil {
		return err
	}

	if _, err := parser.Parse(args); err != nil {
		return err
	}

	client, err := newNotionClient(commandLine.NotionAPIKey)
	if err != nil {
		return err
	}

	repo := clientnotion.New(client, commandLine.DataSourceID, clientnotion.FieldMapping{})
	clients, err := repo.GetAllClients()
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(clients)
}

func newNotionClient(apiKey string) (*notionapi.Client, error) {
	if apiKey != "" {
		return notionapi.New(apiKey), nil
	}
	return notionapi.NewFromEnv()
}
