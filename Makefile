ifneq ("$(wildcard .env)","")
include .env
export
endif

.PHONY: test
test:
	go test ./internal/...

.PHONY: find-clients-db-id
find-clients-db-id:
	go run scripts/try-notion-api-client.go --notion-api-key "$(NOTION_API_KEY)" find-data-source-id-by-title "$(NOTION_CLIENTS_DATA_SOURCE_NAME)"

.PHONY: find-tasks-db-id
find-tasks-db-id:
	go run scripts/try-notion-api-client.go --notion-api-key "$(NOTION_API_KEY)" find-data-source-id-by-title "$(NOTION_TASKS_DATA_SOURCE_NAME)"

.PHONY: try-notion-client-repository
try-notion-client-repository:
	go run scripts/try-notion-client-repository.go --notion-api-key "$(NOTION_API_KEY)" --data-source-id "$(NOTION_CLIENTS_DATA_SOURCE_ID)"
