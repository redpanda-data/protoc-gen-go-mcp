.PHONY: mod
mod:
	go mod tidy

.PHONY: vendor
vendor:
	go mod vendor
