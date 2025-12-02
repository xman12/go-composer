.PHONY: build install clean test run

# Переменные
BINARY_NAME=go-composer
INSTALL_PATH=/usr/local/bin

# Сборка
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) -ldflags="-s -w" .
	@echo "✅ Build complete: ./$(BINARY_NAME)"

# Установка
install: build
	@echo "📦 Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	@sudo cp $(BINARY_NAME) $(INSTALL_PATH)/
	@echo "✅ Installed successfully!"

# Удаление бинарника
clean:
	@echo "🧹 Cleaning..."
	@rm -f $(BINARY_NAME)
	@echo "✅ Clean complete"

# Удаление установленного бинарника
uninstall:
	@echo "🗑️  Uninstalling $(BINARY_NAME)..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✅ Uninstalled successfully!"

# Тесты (в разработке)
test:
	@echo "🧪 Running tests..."
	@go test -v ./...

# Запуск
run: build
	@./$(BINARY_NAME)

# Скачать зависимости
deps:
	@echo "📥 Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "✅ Dependencies downloaded"

# Сборка для всех платформ
build-all:
	@echo "🔨 Building for all platforms..."
	@GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY_NAME)-linux-amd64 -ldflags="-s -w" .
	@GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY_NAME)-darwin-amd64 -ldflags="-s -w" .
	@GOOS=darwin GOARCH=arm64 go build -o bin/$(BINARY_NAME)-darwin-arm64 -ldflags="-s -w" .
	@GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY_NAME)-windows-amd64.exe -ldflags="-s -w" .
	@echo "✅ Build complete for all platforms in ./bin/"

# Справка
help:
	@echo "Available targets:"
	@echo "  make build      - Build the binary"
	@echo "  make install    - Install to $(INSTALL_PATH)"
	@echo "  make clean      - Remove binary"
	@echo "  make uninstall  - Remove installed binary"
	@echo "  make test       - Run tests"
	@echo "  make run        - Build and run"
	@echo "  make deps       - Download dependencies"
	@echo "  make build-all  - Build for all platforms"

