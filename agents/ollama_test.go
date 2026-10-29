package agents

import (
	"encoding/json"
	"strings"
	"testing"
)

// Alguns modelos embutem <think>...</think> no proprio content; isso nao pode
// vazar para a resposta nem confundir o parser de blocos ```tool.
func TestStripThinkTags(t *testing.T) {
	casos := []struct{ nome, entrada, esperado string }{
		{"bloco simples", "<think>raciocínio aqui</think>\nOK", "OK"},
		{"multilinha", "<think>linha 1\nlinha 2</think>\n\n```tool\n{}\n```", "```tool\n{}\n```"},
		{"sem think", "resposta normal", "resposta normal"},
		{"so think", "<think>pensando</think>", ""},
	}

	for _, c := range casos {
		if got := stripThinkTags(c.entrada); got != c.esperado {
			t.Errorf("%s: quero %q, obtive %q", c.nome, c.esperado, got)
		}
	}
}

// num_predict = -1 significa "sem teto" para o Ollama. Como o campo tem
// omitempty, um valor errado poderia sumir da requisição e o teto voltaria
// silenciosamente ao padrão do Ollama.
func TestNumPredictSemTetoChegaNaRequisicao(t *testing.T) {
	opts := DefaultOllamaOptions()
	if opts.NumPredict != -1 {
		t.Fatalf("esperava -1 (sem teto), obtive %d", opts.NumPredict)
	}

	corpo, err := json.Marshal(OllamaChatRequest{Model: "x", Options: opts})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(corpo), `"num_predict":-1`) {
		t.Errorf("num_predict sumiu da requisição: %s", corpo)
	}
}

// Os modelos declaram "tools" nativo e foram treinados nesse formato. Mandar
// os schemas junto é o caminho que eles conhecem — mais confiável que pedir
// blocos de texto e fazer parsing.
func TestSchemasDeFerramentasVaoNaRequisicao(t *testing.T) {
	corpo, err := json.Marshal(OllamaChatRequest{Model: "x", Tools: DefaultTools()})
	if err != nil {
		t.Fatal(err)
	}

	texto := string(corpo)
	for _, nome := range []string{"read_file", "write_file", "list_dir", "execute_command", "set_workspace"} {
		if !strings.Contains(texto, nome) {
			t.Errorf("ferramenta %q não foi enviada: %s", nome, texto)
		}
	}
	if !strings.Contains(texto, `"type":"function"`) {
		t.Errorf("formato de schema errado: %s", texto)
	}
}

// Quando o Ollama preenche tool_calls, é isso que vale — não o texto.
func TestToolCallsNativosSaoLidos(t *testing.T) {
	linha := `{"message":{"role":"assistant","content":"","tool_calls":[
		{"function":{"name":"list_dir","arguments":{"path":"/projeto"}}}]},"done":true}`

	var chunk OllamaChatResponse
	if err := json.Unmarshal([]byte(linha), &chunk); err != nil {
		t.Fatal(err)
	}
	if len(chunk.Message.ToolCalls) != 1 {
		t.Fatalf("tool_calls não foram lidos: %+v", chunk.Message)
	}
	if chunk.Message.ToolCalls[0].Function.Name != "list_dir" {
		t.Errorf("nome errado: %+v", chunk.Message.ToolCalls[0])
	}
}

// O lfm2.5 escreve <tool_call> e </tool_call> ao redor da chamada quando cai no
// formato de texto. As tags são andaime do template de chat e apareceram
// dentro do balão da resposta na TUI.
func TestTagsDeToolCallSaemDoTexto(t *testing.T) {
	entrada := "<tool_call>\n{\"tool\":\"read_file\",\"path\":\"main.go\"}\n</tool_call>"

	got := stripThinkTags(entrada)

	if strings.Contains(got, "tool_call>") {
		t.Errorf("a tag ficou no texto: %q", got)
	}
	// O miolo é a chamada: o parser precisa dela intacta.
	if !strings.Contains(got, `"tool":"read_file"`) {
		t.Errorf("a chamada foi destruída junto com a tag: %q", got)
	}
}

// <think> continua sendo removido com o conteúdo, ao contrário de <tool_call>.
func TestThinkContinuaSendoRemovidoComOConteudo(t *testing.T) {
	got := stripThinkTags("<think>raciocínio interno</think>resposta")
	if got != "resposta" {
		t.Errorf("esperava %q, obtive %q", "resposta", got)
	}
}
