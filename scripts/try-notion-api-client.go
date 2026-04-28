//go:build ignore

// Manually exercises internal/adapters/notionapi against the real Notion API.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kong"
	"github.com/strengthinnumbers-business/client-reminder/internal/adapters/notionapi"
)

type cli struct {
	NotionAPIKey string `name:"notion-api-key" help:"Notion API key. Falls back to NOTION_API_KEY via NewFromEnv when absent."`

	FindDataSourceIDByTitle findDataSourceIDByTitleCmd `cmd:"" name:"find-data-source-id-by-title" help:"Find a Notion data source ID by exact title."`
	QueryDataSource         queryDataSourceCmd         `cmd:"" name:"query-data-source" help:"Query a Notion data source with an empty query."`
}

type findDataSourceIDByTitleCmd struct {
	Title string `arg:"" help:"Exact Notion data source title to find."`
}

type queryDataSourceCmd struct {
	DataSourceID string `arg:"" name:"data-source-id" help:"Notion data source ID to query."`
}

func main() {
	if err := run(os.Stdout, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "try-notion-api-client: %v\n", err)
		os.Exit(2)
	}
}

func run(out io.Writer, args []string) error {
	var commandLine cli
	parser, err := kong.New(&commandLine,
		kong.Name("try-notion-api-client"),
		kong.Description("Manual test helper for the sparse Notion API client."),
		kong.UsageOnError(),
		kong.BindTo(out, (*io.Writer)(nil)),
	)
	if err != nil {
		return err
	}

	context, err := parser.Parse(args)
	if err != nil {
		return err
	}

	client, err := newNotionClient(commandLine.NotionAPIKey)
	if err != nil {
		return err
	}

	return context.Run(client)
}

func newNotionClient(apiKey string) (*notionapi.Client, error) {
	if apiKey != "" {
		return notionapi.New(apiKey), nil
	}
	return notionapi.NewFromEnv()
}

func (cmd *findDataSourceIDByTitleCmd) Run(client *notionapi.Client, out io.Writer) error {
	id, err := client.FindDataSourceIDByTitle(context.Background(), cmd.Title)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, id)
	return err
}

func (cmd *queryDataSourceCmd) Run(client *notionapi.Client, out io.Writer) error {
	pages, err := client.QueryDataSource(context.Background(), cmd.DataSourceID, notionapi.QueryDataSourceRequest{})
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(pages)
}
