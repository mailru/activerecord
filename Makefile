PWD = $(CURDIR)
# Название сервиса
SERVICE_NAME = argen
# 8 символов последнего коммита
LAST_COMMIT_HASH = $(shell git rev-parse HEAD | cut -c -8)
# Таймаут для тестов
TEST_TIMEOUT?=30s
# Тег golang-ci
GOLANGCI_TAG:=1.52.2
# Путь до бинарников
LOCAL_BIN:=$(CURDIR)/bin
# Путь до бинарника golang-ci
GOLANGCI_BIN:=$(LOCAL_BIN)/golangci-lint
# Минимальная верси гошки
MIN_GO_VERSION = 1.19.0
# Версии для сборки
RELEASE = $(shell git describe --tags --always)
# Время сборки
BUILD_DATE = $(shell TZ=UTC-3 date +%Y%m%d-%H%M)
# Операционка
OSNAME = $(shell uname)
# ld флаги
LD_FLAGS = "-X 'main.BuildCommit=$(LAST_COMMIT_HASH)' -X 'main.Version=$(RELEASE)' -X 'main.BuildTime=$(BUILD_DATE)' -X 'main.BuildOS=$(OSNAME)'"
# по дефолту просто make соберёт argen
default: build

# Добавляет флаг для тестирования на наличие гонок
ifdef GO_RACE_DETECTOR
    FLAGS += -race
endif

##################### Проверки для запуска golang-ci #####################
# Проверка локальной версии бинаря
ifneq ($(wildcard $(GOLANGCI_BIN)),)
GOLANGCI_BIN_VERSION:=$(shell $(GOLANGCI_BIN) --version)
ifneq ($(GOLANGCI_BIN_VERSION),)
GOLANGCI_BIN_VERSION_SHORT:=$(shell echo "$(GOLANGCI_BIN_VERSION)" | sed -E 's/.* version (.*) built from .* on .*/\1/g')
else
GOLANGCI_BIN_VERSION_SHORT:=0
endif
ifneq "$(GOLANGCI_TAG)" "$(word 1, $(sort $(GOLANGCI_TAG) $(GOLANGCI_BIN_VERSION_SHORT)))"
GOLANGCI_BIN:=
endif
endif

# Проверка глобальной версии бинаря
ifneq (, $(shell which golangci-lint))
GOLANGCI_VERSION:=$(shell golangci-lint --version 2> /dev/null )
ifneq ($(GOLANGCI_VERSION),)
GOLANGCI_VERSION_SHORT:=$(shell echo "$(GOLANGCI_VERSION)"|sed -E 's/.* version (.*) built from .* on .*/\1/g')
else
GOLANGCI_VERSION_SHORT:=0
endif
ifeq "$(GOLANGCI_TAG)" "$(word 1, $(sort $(GOLANGCI_TAG) $(GOLANGCI_VERSION_SHORT)))"
GOLANGCI_BIN:=$(shell which golangci-lint)
endif
endif
##################### Конец проверок golang-ci #####################

# Устанавливает линтер
.PHONY: install-lint
install-lint:
ifeq ($(wildcard $(GOLANGCI_BIN)),)
	$(info #Downloading golangci-lint v$(GOLANGCI_TAG))
	tmp=$$(mktemp -d) && cd $$tmp && pwd && go mod init temp && go get -d github.com/golangci/golangci-lint/cmd/golangci-lint@v$(GOLANGCI_TAG) && \
		go build -ldflags "-X 'main.version=$(GOLANGCI_TAG)' -X 'main.commit=test' -X 'main.date=test'" -o $(LOCAL_BIN)/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint && \
		rm -rf $$tmp
GOLANGCI_BIN:=$(LOCAL_BIN)/golangci-lint
endif

# Линтер проверяет лишь отличия от мастера
.PHONY: lint
lint: install-lint
	$(GOLANGCI_BIN) run --config=.golangci.yml ./... --new-from-rev=origin/main --build-tags=activerecord

# Линтер проходится по всему коду
.PHONY: full-lint
full-lint: install-lint
	$(GOLANGCI_BIN) run --config=.golangci.yml ./... --build-tags=activerecord

# создание отчета о покрытии тестами
.PHONY: cover
cover:
	go test -timeout=$(TEST_TIMEOUT) -v -coverprofile=coverage.out ./...  && go tool cover -html=coverage.out

# Запустить unit тесты
.PHONY: test
test:
	echo "Start testing activerecord \n"
	go test -parallel=10 $(PWD)/... -coverprofile=cover.out -timeout=$(TEST_TIMEOUT)

.PHONY: install
install:
	go install -ldflags=$(LD_FLAGS) ./...

# Сборка сервиса
.PHONY: build
build:
	./scripts/goversioncheck.sh $(MIN_GO_VERSION) && go build -o bin/$(SERVICE_NAME) -ldflags=$(LD_FLAGS) $(PWD)/cmd/$(SERVICE_NAME)

# Устанавливает в локальный проект хук, который проверяет запускает линтеры
.PHONY: pre-commit-hook
pre-commit-hook:
	touch ./.git/hooks/pre-commit
	echo '#!/bin/sh' > ./.git/hooks/pre-commit
	echo 'make generate' >> ./.git/hooks/pre-commit
	echo 'make lint' >> ./.git/hooks/pre-commit
	chmod +x ./.git/hooks/pre-commit

# Устанавливает в локальный проект хук, который проверяет запускает линтеры
.PHONY: pre-push-hook
pre-push-hook:
	touch ./.git/hooks/pre-push
	echo '#!/bin/sh' > ./.git/hooks/pre-push
	echo 'make cover' >> ./.git/hooks/pre-push
	chmod +x ./.git/hooks/pre-push

