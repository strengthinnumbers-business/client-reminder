ENV := .env

ifneq ("$(wildcard $(ENV))","")
include $(ENV)
export
endif

.PHONY: test
test:
	go test ./internal/...


##############################################################################
# The following targets are for testing the "lower-level" Notion API client. #
##############################################################################

.PHONY: find-clients-db-id
find-clients-db-id:
	go run scripts/try-notion-api-client.go \
	--notion-api-key "$(NOTION_API_KEY)" \
	find-data-source-id-by-title "$(NOTION_CLIENTS_DATA_SOURCE_NAME)"

.PHONY: find-tasks-db-id
find-tasks-db-id:
	go run scripts/try-notion-api-client.go \
	--notion-api-key "$(NOTION_API_KEY)" \
	find-data-source-id-by-title "$(NOTION_TASKS_DATA_SOURCE_NAME)"

.PHONY: query-clients
query-clients:
	go run scripts/try-notion-api-client.go \
	--notion-api-key "$(NOTION_API_KEY)" \
	query-data-source "$(NOTION_CLIENTS_DATA_SOURCE_ID)"

.PHONY: query-tasks
query-tasks:
	go run scripts/try-notion-api-client.go \
	--notion-api-key "$(NOTION_API_KEY)" \
	query-data-source "$(NOTION_TASKS_DATA_SOURCE_ID)"

.PHONY: retrieve-page
retrieve-page:
	# Add a `PAGE_ID=[ACTUAL_ID]` in front of the `make retrieve-page` command.
	go run scripts/try-notion-api-client.go \
	--notion-api-key "$(NOTION_API_KEY)" \
	retrieve-page "$(PAGE_ID)"

.PHONY: create-task
create-task:
	go run scripts/try-notion-api-client.go \
	--notion-api-key "$(NOTION_API_KEY)" \
	create-task-page \
	--data-source-id="$(NOTION_TASKS_DATA_SOURCE_ID)" \
	--title-property="Title" \
	--title="2026-04: Test Reminder Client A" \
	--rich-text-property="Period Key" \
	--rich-text="2026-04" \
	--relation-property="Reminder Client" \
	--related-page-id="$(NOTION_TEST_CLIENT_ID_A)" \
	--select-property="Status" \
	--select="unset"

.PHONY: update-task
update-task:
	go run scripts/try-notion-api-client.go \
	--notion-api-key "$(NOTION_API_KEY)" \
	update-task-page \
	--page-id="$(PAGE_ID)" \
	--select-property="Status" \
	--select="upload_complete"


#############################################################################
# The following targets are for testing the "integration" between the       #
# "lower-level" Notion API client and the "higher-level" client repository. #
#############################################################################

.PHONY: try-notion-client-repository
try-notion-client-repository:
	go run scripts/try-notion-client-repository.go \
	--notion-api-key "$(NOTION_API_KEY)" \
	--data-source-id "$(NOTION_CLIENTS_DATA_SOURCE_ID)"
