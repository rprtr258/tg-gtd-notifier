.PHONY: run
run:
	@env $(shell cat .env) go run main.go
