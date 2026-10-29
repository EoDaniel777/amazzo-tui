# amazzo-tui

Agente de terminal em Go, com interface TUI (Bubble Tea), para conversar com
modelos de IA e trabalhar nos arquivos do diretório atual.

- Modelos **locais** via [Ollama](https://ollama.com), sem token nem conta.
- Modelos **remotos** via Hugging Face, quando há um `HF_TOKEN` configurado.
- Agentes especializados (`/agentes`), temas visuais (`/temas`) e servidor MCP.
- Permissões e verificações de segurança para ações no sistema de arquivos.

## Requisitos

- Go 1.23 ou superior
- Um terminal com suporte a TUI
- Ollama rodando localmente, se for usar modelos locais

## Compilar e executar

```
make build    # gera build/amazzo
make run      # compila e abre a TUI
make install  # instala em ~/.local/bin
make test     # roda os testes
```

Sem argumentos, `amazzo` abre a TUI. Também é possível usar `amazzo tui`.

## Token do Hugging Face (opcional)

O token só é necessário para modelos remotos. Ele é lido, nesta ordem, de:

1. variável de ambiente `HF_TOKEN`
2. arquivo `.env` no diretório atual, `~/.config/amazzo/.env` ou `~/.env`
3. credenciais salvas pelo comando `/key` (armazenadas criptografadas)

Nunca versione o seu `.env`. O `.gitignore` do projeto já o ignora.

## Comandos principais da TUI

| Comando | O que faz |
|---|---|
| `/ajuda` | lista os comandos |
| `/agente` | lista ou troca o modelo |
| `/agentes` | gerencia agentes especializados |
| `/temas` | lista ou troca o tema |
| `/key` | configura o token dos modelos remotos |
| `/limpar` | limpa o histórico da sessão |
| `/sair` | encerra |

## Estrutura

```
cmd/          entrada da CLI (cobra) e inicialização da TUI
app/          modelo Bubble Tea, streaming, cache e integração com Ollama
agents/       agentes, orquestração, balanceamento e ferramentas
commands/     comandos de barra
mcp/          servidor MCP e verificações de segurança
ui/           componentes visuais
themes/       temas
credentials/  armazenamento de token
```

## Licença

[MIT](LICENSE)
