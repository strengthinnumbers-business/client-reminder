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
	RetrievePage            retrievePageCmd            `cmd:"" name:"retrieve-page" help:"Retrieve a Notion page by ID."`
	CreateTaskPage          createTaskPageCmd          `cmd:"" name:"create-task-page" help:"Create a sparse task page under a Notion data source."`
	UpdateTaskPage          updateTaskPageCmd          `cmd:"" name:"update-task-page" help:"Update one select property on a Notion task page."`
}

type findDataSourceIDByTitleCmd struct {
	Title string `arg:"" help:"Exact Notion data source title to find."`
}

type queryDataSourceCmd struct {
	DataSourceID string `arg:"" name:"data-source-id" help:"Notion data source ID to query."`
}

type retrievePageCmd struct {
	PageID string `arg:"" name:"page-id" help:"Notion page ID to retrieve."`
}

type createTaskPageCmd struct {
	DataSourceID     string `name:"data-source-id" required:"" help:"Notion data source ID that will parent the new task page."`
	TitleProperty    string `name:"title-property" required:"" help:"Name of the title property to set."`
	Title            string `name:"title" required:"" help:"Title value for the new task page."`
	RichTextProperty string `name:"rich-text-property" required:"" help:"Name of the rich_text property to set."`
	RichText         string `name:"rich-text" required:"" help:"Plain text value for the rich_text property."`
	RelationProperty string `name:"relation-property" required:"" help:"Name of the relation property to set."`
	RelatedPageID    string `name:"related-page-id" required:"" help:"Related Notion page ID for the relation property."`
	SelectProperty   string `name:"select-property" required:"" help:"Name of the select property to set."`
	Select           string `name:"select" required:"" help:"Select option name to set."`
}

type updateTaskPageCmd struct {
	PageID         string `name:"page-id" required:"" help:"Notion page ID to update."`
	SelectProperty string `name:"select-property" required:"" help:"Name of the select property to update."`
	Select         string `name:"select" required:"" help:"Select option name to set."`
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
	return writeJSON(out, pages)
}

func (cmd *retrievePageCmd) Run(client *notionapi.Client, out io.Writer) error {
	page, err := client.RetrievePage(context.Background(), cmd.PageID, notionapi.RetrievePageRequest{})
	if err != nil {
		return err
	}
	return writeJSON(out, page)
}

func (cmd *createTaskPageCmd) Run(client *notionapi.Client, out io.Writer) error {
	page, err := client.CreatePage(context.Background(), notionapi.CreatePageRequest{
		DataSourceID: cmd.DataSourceID,
		Properties: notionapi.PagePropertyUpdates{
			cmd.TitleProperty:    notionapi.TitleProperty(cmd.Title),
			cmd.RichTextProperty: notionapi.RichTextProperty(cmd.RichText),
			cmd.RelationProperty: notionapi.RelationProperty(cmd.RelatedPageID),
			cmd.SelectProperty:   notionapi.SelectProperty(cmd.Select),
		},
	})
	if err != nil {
		return err
	}
	return writeJSON(out, page)
}

func (cmd *updateTaskPageCmd) Run(client *notionapi.Client, out io.Writer) error {
	page, err := client.UpdatePageSelect(context.Background(), cmd.PageID, notionapi.UpdatePageSelectRequest{
		PropertyName: cmd.SelectProperty,
		SelectName:   cmd.Select,
	})
	if err != nil {
		return err
	}
	return writeJSON(out, page)
}

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
