# amazzo-tui

Agente de terminal em Go, com interface TUI (Bubble Tea), para conversar com
modelos de IA e trabalhar nos arquivos do diretório atual.

> Este repositório contém **apenas o agente de terminal**. Existe outro projeto
> do mesmo autor chamado Amazzo, com outro propósito e outro código. O sufixo
> `-tui` serve para diferenciar os dois.

- Modelos **locais** via [Ollama](https://ollama.com), sem token nem conta.
- Modelos **na nuvem** pelo endpoint compatível com OpenAI do amazzo.tech.
- Modelos do Hugging Face, quando há um `HF_TOKEN` configurado (opcional).
- Layout estilo editor: explorador de arquivos, chat, painel de saída e barra de status.
- Ferramentas de arquivo e de shell, com permissão para o que é arriscado.
- Cliente MCP: usa os servidores configurados no `opencode.json`, se houver.
- 23 temas de cores (`/temas`), agentes especializados (`/agentes`).

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

## Modelos

`/ajuda` → **Trocar modelo…** (ou `ctrl+k` e digite "modelo") abre a lista,
dividida em seções:

| Seção | De onde vem |
|---|---|
| **Local · roda na sua GPU** | os modelos instalados no Ollama (`/ollama` relê a lista) |
| **Nuvem · amazzo.tech** | `amazzo`, `autom-tico` (Automático), Claude Sonnet/Opus/Fable, GPT-6 Astra, Gemini 4 Argon/Carbon, Muse Glimmer, Kimi K3, MiniMax M3, Gemma 4 31B, Dots 3, Tencent Hy4, DeepSeek V4.1 Flash, GLM 5.3, Qwen3 Coder, Cohere North Mini e DeepSeek V4 Pro |
| **Nuvem · Hugging Face** | só aparece com `HF_TOKEN` |

Cada modelo mostra uma nota (por exemplo "só texto" ou "texto e imagem").
Também funciona `/modelo <número>`.

### Chave do amazzo.tech

A chave e o endereço são lidos, nesta ordem de prioridade, de:

1. variáveis de ambiente `AMAZZO_API_KEY` e `AMAZZO_BASE_URL`
2. provider `amazzo` em `~/.config/opencode/opencode.json`
   (`options.apiKey` e `options.baseURL`; o padrão do endereço é `https://amazzo.tech/v1`)

A chave nunca é impressa nem copiada para outro arquivo. Sem chave, a TUI
avisa o que fazer ao escolher um modelo de nuvem.

### Token do Hugging Face (opcional)

Lido, nesta ordem, de:

1. variável de ambiente `HF_TOKEN`
2. arquivo `.env` no diretório atual, `~/.config/amazzo/.env` ou `~/.env`
3. credenciais salvas pelo comando `/key` (armazenadas criptografadas)

Nunca versione o seu `.env`. O `.gitignore` do projeto já o ignora.

### Modelos locais

O Ollama usa 4096 de contexto por padrão, pouco para um agente. A TUI envia
`num_ctx` = 12288 (ajustável com `AMAZZO_OLLAMA_NUM_CTX`). O raciocínio do
modelo vem desligado, por ser bem mais rápido; `/pensar` liga e desliga
(`AMAZZO_THINK=1` liga por padrão).

## Trabalhando na TUI

- **Executar comandos:** o modelo roda `execute_command` em segundo plano, sem
  travar a tela; `esc` interrompe. Comandos que terminam (build, teste) esperam
  o fim. Servidores e bots passam para segundo plano e devolvem o pid e o
  arquivo de log (parar com `kill -- -PID`).
- **`!comando`** roda direto na pasta do projeto, sem passar pelo modelo.
  `/rodar` lista como o projeto costuma ser iniciado.
- **Ferramentas de arquivo:** ler (por trecho), escrever, editar, buscar, listar,
  mover, apagar. Escrever segredos em arquivos é bloqueado e comandos
  destrutivos pedem confirmação.
- **Barra de status** sempre visível: branch, pasta, modelo, relógios da
  conversa e da tarefa, uso de contexto, tok/s e ferramentas usadas.
- **Enquanto espera**, a TUI mostra um spinner com verbos que giram
  ("pensando", "assimilando", "carregando"…).

## Servidores MCP

A TUI é cliente MCP (stdio e HTTP). Ela lê os servidores dos arquivos abaixo,
do mais geral para o mais específico:

1. `~/.config/opencode/opencode.json` (chave `mcp`)
2. `~/.config/amazzo/mcp.json`
3. na pasta do projeto: `opencode.json`, `.mcp.json` ou `.amazzo/mcp.json`

Aceita o formato do opencode (`"mcp": {nome: {type, command[], url, environment}}`)
e o do `.mcp.json` (`"mcpServers": {nome: {command, args, env}}`).

As ferramentas aparecem para o modelo como `servidor_ferramenta`. Para não
estourar o contexto, só entram no prompt até ~1500 tokens de schemas; o
restante fica disponível por `mcp_search` e `mcp_call`.

| Comando | O que faz |
|---|---|
| `/mcp` | lista os servidores e o estado de cada um |
| `/mcp recarregar` | relê a configuração e reinicia os servidores, sem fechar a sessão |
| `/mcp ligar\|desligar <nome>` | liga ou desliga um servidor (persistente) |
| `/mcp confiar\|desconfiar <nome>` | para de pedir (ou volta a pedir) permissão a cada chamada |
| `/mcp ferramentas [nome]` | lista as ferramentas |

A configuração é recarregada sozinha quando um dos arquivos muda, e o modelo
tem a ferramenta `mcp_reload` para se recuperar de um servidor que caiu.
Ferramentas que não se declaram somente leitura pedem permissão, a menos que o
servidor seja marcado como confiável.

## Comandos principais

| Comando | O que faz |
|---|---|
| `/ajuda` ou `ctrl+k` | abre o menu de comandos, com busca |
| `/modelo` | troca o modelo (local ou nuvem) |
| `/agentes` | gerencia agentes especializados |
| `/temas` | troca o tema, com prévia ao vivo |
| `/mcp` | servidores MCP |
| `/rodar`, `!comando` | executa programas do projeto |
| `/pasta <caminho>` | troca a pasta de trabalho |
| `/pensar` | liga ou desliga o raciocínio do modelo |
| `/explorador`, `/painel`, `/foco` | mostram ou ocultam os painéis (`ctrl+b`, `ctrl+g`) |
| `/key` | configura o token do Hugging Face |
| `/limpar` | limpa o histórico da sessão |
| `/sair` | encerra |

## Estrutura

```
cmd/          entrada da CLI (cobra) e inicialização da TUI
app/          modelo Bubble Tea, layout, chat, streaming e integração com Ollama e nuvem
agents/       agentes, orquestração e schemas das ferramentas
commands/     comandos de barra
mcp/          ferramentas de arquivo e shell, e verificações de segurança
mcpclient/    cliente MCP (stdio e HTTP) para servidores externos
themes/       temas
credentials/  armazenamento de token
```

## Licença

[MIT](LICENSE)
