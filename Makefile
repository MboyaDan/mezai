# Convenience targets from the repo root; the real ones live in backend/Makefile.
BACKEND_TARGETS := help up down logs ps test lint run migrate migrate-status migrate-create seed sqlc

.PHONY: $(BACKEND_TARGETS) web
$(BACKEND_TARGETS):
	@$(MAKE) --no-print-directory -C backend $@

web: ## Run the Next.js dev server
	cd frontend && npm run dev
