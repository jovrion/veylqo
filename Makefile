# Veylqo — repeatable targets.
# Board rules in work/README.md. Build targets arrive with VYQ-11.

SHELL := /bin/bash

.PHONY: help work-validate work-board work-view work-next work-start work-done work-new

help: ## List targets
	@grep -E '^[a-zA-Z0-9_%-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-24s %s\n", $$1, $$2}'

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
