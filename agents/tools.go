package agents

// Os modelos locais (qwen2.5-coder, granite4.1, lfm2.5) declaram capability
// "tools" e foram treinados no formato padrão de function calling. Mandar os
// schemas junto é o caminho que eles conhecem: sem isso eles caem nesse
// formato sozinhos e o texto vira uma chamada que ninguém executa.

func propTexto(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": desc}
}

func propInt(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "integer", "description": desc}
}

func declara(nome, desc string, props map[string]interface{}, obrigatorios ...string) Tool {
	return Tool{
		Type: ToolTypeFunction,
		Function: Function{
			Name:        nome,
			Description: desc,
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": props,
				"required":   obrigatorios,
			},
		},
	}
}

// DefaultTools devolve os schemas das ferramentas do Amazzo.
//
// As descrições estão em português (é a língua da conversa) e dizem QUANDO
// usar cada uma: modelos pequenos seguem isso melhor do que uma lista de
// regras separada no prompt.
func DefaultTools() []Tool {
	return []Tool{
		declara("set_workspace",
			"Abre uma pasta de projeto para trabalhar nela. Use ANTES de qualquer outra ferramenta quando o usuário citar um caminho fora da pasta atual.",
			map[string]interface{}{"path": propTexto("Caminho absoluto da pasta do projeto")}, "path"),

		declara("list_dir", "Lista arquivos e pastas de um diretório do projeto.",
			map[string]interface{}{"path": propTexto(`Caminho relativo à pasta de trabalho. Use "." para a raiz`)}, "path"),

		declara("read_file", "Lê um arquivo do projeto. Em arquivo grande, leia por trechos com start_line e end_line.",
			map[string]interface{}{
				"path":       propTexto("Caminho do arquivo"),
				"start_line": propInt("Primeira linha a ler (opcional, começa em 1)"),
				"end_line":   propInt("Última linha a ler (opcional)"),
			}, "path"),

		declara("write_file",
			"Cria ou substitui um arquivo (cria as pastas que faltarem). Chame esta ferramenta SEMPRE que o usuário pedir para criar, salvar, gravar ou escrever um arquivo — descrever o conteúdo na resposta não cria nada. Para apenas mostrar código, escreva na resposta.",
			map[string]interface{}{
				"path":    propTexto("Caminho do arquivo"),
				"content": propTexto("Conteúdo completo do arquivo"),
			}, "path", "content"),

		declara("edit_file", "Substitui um trecho de um arquivo existente. Leia o arquivo antes e copie o trecho EXATO em search; ele precisa ser único (inclua linhas ao redor se preciso). Você recebe de volta o trecho já alterado.",
			map[string]interface{}{
				"path":    propTexto("Caminho do arquivo"),
				"search":  propTexto("Trecho exato a procurar"),
				"replace": propTexto("Texto que entra no lugar"),
			}, "path", "search", "replace"),

		declara("append_file", "Acrescenta texto ao fim de um arquivo (cria se não existir). Use para adicionar seções sem reescrever o arquivo inteiro.",
			map[string]interface{}{
				"path":    propTexto("Caminho do arquivo"),
				"content": propTexto("Texto a acrescentar"),
			}, "path", "content"),

		declara("delete_file", "Apaga um arquivo. O usuário confirma antes.",
			map[string]interface{}{"path": propTexto("Caminho do arquivo")}, "path"),

		declara("move_file", "Move ou renomeia um arquivo ou pasta. O usuário confirma antes.",
			map[string]interface{}{
				"path":      propTexto("Origem"),
				"dest_path": propTexto("Destino"),
			}, "path", "dest_path"),

		declara("file_info", "Diz se um caminho existe, se é arquivo ou pasta, e o tamanho. Barato: use antes de criar ou sobrescrever.",
			map[string]interface{}{"path": propTexto("Caminho")}, "path"),

		declara("make_dir", "Cria um diretório (e os pais).",
			map[string]interface{}{"path": propTexto("Caminho do diretório")}, "path"),

		declara("search_files",
			"Procura no projeto. Padrão com curinga (*.go) procura ARQUIVO por nome; padrão sem curinga procura TEXTO dentro dos arquivos.",
			map[string]interface{}{
				"pattern": propTexto("Texto a procurar, ou glob de nome de arquivo como *.go"),
				"path":    propTexto(`Onde procurar. Use "." para o projeto inteiro`),
			}, "pattern"),

		declara("execute_command",
			"Roda um comando de shell na pasta de trabalho. Para build, teste e scripts que terminam, espera o fim. "+
				"Para servidor, bot ou programa que fica rodando, passe background=true: volta em 3s com o pid e o log, e a conversa continua.",
			map[string]interface{}{
				"command":    propTexto("O comando, ex.: go test ./... ou ./meuapp"),
				"background": map[string]interface{}{"type": "boolean", "description": "true para programa que não termina (servidor, bot, watcher)"},
			}, "command"),

		declara("subagent",
			"Delega uma tarefa de PESQUISA a um subagente que lê arquivos e busca no projeto por conta própria e devolve só a conclusão. "+
				"Use para investigar algo amplo (\"como funciona o módulo X\", \"onde Y é usado\") sem encher a sua conversa de arquivos. "+
				"Vários subagent no mesmo lote rodam em paralelo. O subagente só lê: quem edita e roda comandos é você.",
			map[string]interface{}{
				"task": propTexto("O que descobrir, com contexto suficiente para ele trabalhar sozinho e o formato esperado da resposta"),
			}, "task"),

		declara("job_list", "Lista os comandos que ficaram rodando em segundo plano (id, estado, comando).",
			map[string]interface{}{}),

		declara("job_output", "Lê a saída mais recente de um comando em segundo plano, sem esperar ele terminar.",
			map[string]interface{}{
				"job_id": map[string]interface{}{"type": "integer", "description": "Número do job (vem no resultado do execute_command)"},
				"lines":  map[string]interface{}{"type": "integer", "description": "Quantas linhas finais mostrar (padrão 80)"},
			}, "job_id"),

		declara("job_stop", "Encerra um comando em segundo plano.",
			map[string]interface{}{
				"job_id": map[string]interface{}{"type": "integer", "description": "Número do job"},
			}, "job_id"),

		declara("web_search", "Pesquisa na internet (DuckDuckGo) e devolve título, link e trecho de cada resultado. Depois abra o melhor com web_fetch.",
			map[string]interface{}{"query": propTexto("O que pesquisar")}, "query"),

		declara("web_fetch", "Abre uma URL e devolve a página em markdown (títulos, listas, código, tabelas e links absolutos). "+
			"Serve para sites, documentação, JSON e arquivos de repositório; links github.com/.../blob/... viram o arquivo bruto. "+
			"Páginas longas vêm em fatias de ~15 mil caracteres: o fim do resultado diz o offset para continuar.",
			map[string]interface{}{
				"url":    propTexto("Endereço completo"),
				"offset": map[string]interface{}{"type": "integer", "description": "Caractere em que continuar a leitura (vem no rodapé da fatia anterior)"},
			}, "url"),
	}
}
