
# Default target executed when no arguments are given to make.
all: help

lint:
	uv run ruff check --output-format concise .

format:
	uv run ruff check --output-format concise --fix .

test:
	uv run pytest tests


######################
# HELP
######################

help:
	@echo '----'
	@echo 'lint                         - run linters'
	@echo 'format                       - run code formatters'
	@echo 'test                         - run unit tests'
