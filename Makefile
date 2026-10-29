# Makefile para Amazzo
# Facilita compilação e instalação

.PHONY: all build clean install run test help

# Variáveis
BINARY_NAME=amazzo
BUILD_DIR=build
MAIN_FILE=main.go
# Instala onde o PATH ja procura (era ~/go/bin, que nao esta em uso).
INSTALL_PATH=$(HOME)/.local/bin

# Target padrão
all: clean build

# Compila o projeto para /build
build:
	@echo "🔨 Compilando Amazzo..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "✅ Compilado com sucesso: $(BUILD_DIR)/$(BINARY_NAME)"

# Limpa binários antigos
clean:
	@echo "🧹 Limpando binários antigos..."
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)*
	@echo "✅ Limpeza concluída"

# Instala globalmente
install: build
	@echo "📦 Instalando Amazzo globalmente..."
	@mkdir -p $(INSTALL_PATH)
	@install -m 0755 $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✅ Instalado em: $(INSTALL_PATH)/$(BINARY_NAME)"
	@echo "💡 Agora você pode usar 'amazzo' de qualquer lugar!"

# Executa o binário compilado
run: build
	@echo "🚀 Executando Amazzo..."
	@$(BUILD_DIR)/$(BINARY_NAME)

# Executa testes
test:
	@echo "🧪 Executando testes..."
	@go test -v ./...

# Mostra ajuda
help:
	@echo "📚 Comandos disponíveis:"
	@echo "  make build    - Compila o projeto para /build"
	@echo "  make clean    - Remove binários antigos"
	@echo "  make install  - Instala globalmente em ~/.local/bin"
	@echo "  make run      - Compila e executa"
	@echo "  make test     - Executa testes"
	@echo "  make all      - Limpa e compila (padrão)"
	@echo "  make help     - Mostra esta mensagem"
