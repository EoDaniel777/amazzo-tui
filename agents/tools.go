package agents

// Os modelos locais (qwen2.5-coder, granite4.1, lfm2.5) declaram capability
// "tools" e foram treinados no formato padrão de function calling. Mandar os
// schemas junto é o caminho que eles conhecem: sem isso eles caem nesse
// formato sozinhos e o texto vira uma chamada que ninguém executa.

func propTexto(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": desc}
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

		declara("read_file", "Lê o conteúdo de um arquivo do projeto.",
			map[string]interface{}{"path": propTexto("Caminho do arquivo")}, "path"),

		declara("write_file",
			"Cria ou substitui um arquivo. Use só quando o usuário pedir para criar, salvar ou gravar — para apenas mostrar código, escreva na resposta.",
			map[string]interface{}{
				"path":    propTexto("Caminho do arquivo"),
				"content": propTexto("Conteúdo completo do arquivo"),
			}, "path", "content"),

		declara("edit_file", "Substitui um trecho de um arquivo existente.",
			map[string]interface{}{
				"path":    propTexto("Caminho do arquivo"),
				"search":  propTexto("Trecho exato a procurar"),
				"replace": propTexto("Texto que entra no lugar"),
			}, "path", "search", "replace"),

		declara("make_dir", "Cria um diretório.",
			map[string]interface{}{"path": propTexto("Caminho do diretório")}, "path"),

		declara("search_files",
			"Procura no projeto. Padrão com curinga (*.go) procura ARQUIVO por nome; padrão sem curinga procura TEXTO dentro dos arquivos.",
			map[string]interface{}{
				"pattern": propTexto("Texto a procurar, ou glob de nome de arquivo como *.go"),
				"path":    propTexto(`Onde procurar. Use "." para o projeto inteiro`),
			}, "pattern"),

		declara("execute_command",
			"Roda um comando de shell na pasta de trabalho. Pede confirmação do usuário.",
			map[string]interface{}{"command": propTexto("O comando, ex.: go test ./...")}, "command"),

		declara("web_fetch", "Busca o conteúdo de uma URL.",
			map[string]interface{}{"url": propTexto("Endereço completo")}, "url"),
	}
}
