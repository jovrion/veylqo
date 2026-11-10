# Veylqo — repeatable targets.
# Board rules in work/README.md. No third-party require; drivers arrive with adapters.

SHELL := /bin/bash

.PHONY: help fmt vet test build work-validate work-board work-view work-next work-start work-done work-new

help: ## List targets
	@grep -E '^[a-zA-Z0-9_%-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-24s %s\n", $$1, $$2}'

# ---- Go (stdlib only until adapter tasks) ----
# vet/test exit 1 when the module has no packages. list still exits 0.
# Any other list failure is printed and fails the target.

define with_packages
	@out=$$(go list ./... 2>&1); status=$$?; \
	if [ $$status -ne 0 ]; then printf '%s\n' "$$out" >&2; exit $$status; fi; \
	if printf '%s\n' "$$out" | grep -q 'matched no packages'; then \
	  echo '$(1): no Go packages yet'; \
	else \
	  $(2); \
	fi
endef

fmt: ## Format Go sources with gofmt -s
	gofmt -s -w .

vet: ## Run go vet
	$(call with_packages,vet,go vet ./...)

test: ## Run tests
	$(call with_packages,test,go test ./...)

build: ## Build every package in the module
	$(call with_packages,build,go build ./...)

# ---- Work tracking (work/README.md) ----

work-validate: ## Check work/ schema, links, cycles, WIP limit and board freshness
	@python3 scripts/work.py validate

work-board: ## Regenerate work/BOARD.md
	@python3 scripts/work.py board

work-view: ## Build work/board.html and open it (NOOPEN=1 to only build)
	@python3 scripts/work.py view $(if $(NOOPEN),,--open)

work-next: ## Recommend the next task to start
	@python3 scripts/work.py next

work-start: ## Claim a task: make work-start ID=VYQ-10
	@python3 scripts/work.py start --id '$(ID)'

work-done: ## Close a task (Verification must be ticked): make work-done ID=VYQ-10
	@python3 scripts/work.py done --id '$(ID)'

work-new: ## New task/epic: make work-new EPIC=VYQ-2 AREA=go TITLE="..."
	@python3 scripts/work.py new --type '$(or $(TYPE),task)' --epic '$(EPIC)' \
	  --area '$(AREA)' --title '$(TITLE)' --phase '$(PHASE)'
