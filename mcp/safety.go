package mcp

import (
	"regexp"
	"strings"
)

// A política de comandos era uma whitelist de prefixos: só rodava o que
// estivesse listado. Isso barrava mkdir, cp, mv, touch, sed, cd — ou seja, o
// trabalho normal de mexer num projeto — e deixava o agente inútil para o que
// ele existe para fazer.
//
// O portão de verdade é a confirmação do usuário: execute_command sempre pede
// autorização, e a tela mostra o comando inteiro. Aqui só barramos o que é
// irreversível ou está fora do escopo de trabalhar num projeto: apagar a raiz
// do sistema, formatar disco, escrever direto em dispositivo, desligar a
// máquina e baixar script da internet para executar.

// padroesPerigosos casa comandos que não têm uso legítimo aqui.
var padroesPerigosos = []struct {
	re     *regexp.Regexp
	motivo string
}{
	{regexp.MustCompile(`(^|[;&|]\s*)(sudo\s+)?rm\s+(-[a-zA-Z]*\s+)*(/|/\*|~|\$home|/home(/[^/\s]+)?)\s*($|[;&|])`),
		"apagaria a raiz do sistema ou uma pasta de usuário inteira"},
	{regexp.MustCompile(`\bmkfs(\.\w+)?\b`), "formataria um sistema de arquivos"},
	{regexp.MustCompile(`\bdd\b[^|]*\bof=/dev/`), "escreveria direto num dispositivo de bloco"},
	{regexp.MustCompile(`>\s*/dev/(sd|nvme|hd)`), "escreveria direto num disco"},
	{regexp.MustCompile(`:\(\)\s*\{.*\}\s*;\s*:`), "é uma fork bomb"},
	{regexp.MustCompile(`\b(shutdown|reboot|halt|poweroff|init\s+0)\b`), "desligaria ou reiniciaria a máquina"},
	{regexp.MustCompile(`\b(curl|wget)\b[^|]*\|\s*(sudo\s+)?(ba)?sh\b`),
		"executaria um script baixado da internet sem inspeção"},
	{regexp.MustCompile(`\bchmod\s+(-[a-zA-Z]+\s+)*[0-7]{3,4}\s+/\s*($|[;&|])`),
		"mudaria as permissões da raiz do sistema"},
	{regexp.MustCompile(`\b(mkswap|fdisk|parted|wipefs)\b`), "alteraria o particionamento do disco"},
	{regexp.MustCompile(`\bhistory\s+-c\b|>\s*~/\.bash_history`), "apagaria o histórico do shell"},
}

// caminhosProtegidos são diretórios do sistema. Técnica emprestada do
// ghost-framework: ler deles é diagnóstico, escrever neles é estrago — e
// nenhum trabalho de projeto precisa disso.
var caminhosProtegidos = []string{
	"/bin", "/sbin", "/usr", "/lib", "/lib64", "/etc",
	"/boot", "/proc", "/sys", "/dev", "/var/lib",
}

// escritaEmCaminhoProtegido casa comandos que gravam num diretório de sistema.
var escritaEmCaminhoProtegido = regexp.MustCompile(
	`(^|[;&|]\s*)(sudo\s+)?(rm|mv|cp|touch|tee|install|ln|chown|chmod|truncate)\b[^;&|]*\s(/\S+)`)

// redirecionamentoParaArquivo casa "> /caminho" e ">> /caminho".
var redirecionamentoParaArquivo = regexp.MustCompile(`>>?\s*(/\S+)`)

// comandoPerigoso devolve o motivo pelo qual um comando é recusado, ou string
// vazia se ele pode rodar (após a confirmação do usuário).
func comandoPerigoso(command string) string {
	normalizado := strings.ToLower(strings.TrimSpace(command))

	for _, p := range padroesPerigosos {
		if p.re.MatchString(normalizado) {
			return p.motivo
		}
	}

	if alvo := alvoProtegido(normalizado); alvo != "" {
		return "gravaria em " + alvo + ", que é diretório de sistema"
	}

	return ""
}

// alvoProtegido devolve o caminho de sistema que o comando tentaria gravar.
func alvoProtegido(command string) string {
	var candidatos []string

	for _, m := range escritaEmCaminhoProtegido.FindAllStringSubmatch(command, -1) {
		candidatos = append(candidatos, m[len(m)-1])
	}
	for _, m := range redirecionamentoParaArquivo.FindAllStringSubmatch(command, -1) {
		candidatos = append(candidatos, m[1])
	}

	for _, alvo := range candidatos {
		for _, protegido := range caminhosProtegidos {
			if alvo == protegido || strings.HasPrefix(alvo, protegido+"/") {
				return alvo
			}
		}
	}

	return ""
}

// removeArquivos casa comandos que apagam ou desfazem trabalho.
var removeArquivos = regexp.MustCompile(
	`(^|[;&|]\s*)(sudo\s+)?(rm|rmdir|shred|unlink|truncate|dd|mv)\b` +
		`|\bgit\s+(reset\s+--hard|clean\b|checkout\s+--)`)

// ComandoRemove diz se um comando apaga ou desfaz trabalho, e por isso merece
// uma confirmação.
//
// A regra antiga pedia confirmação para TODO execute_command — inclusive
// "go build" e "ls". Confirmar o tempo todo não protege ninguém: vira um
// reflexo de apertar "permitir". Confirmação só vale quando há algo a perder.
func ComandoRemove(command string) bool {
	return removeArquivos.MatchString(strings.ToLower(strings.TrimSpace(command)))
}

// MotivoDeConfirmacao devolve POR QUE um comando merece uma pergunta, ou ""
// quando ele pode rodar direto.
//
// A politica e lista negra, nao lista branca: por padrao o comando roda. So
// paramos no que o usuario disse que quer decidir pessoalmente --
//
//  1. o que e irreversivel (apaga ou desfaz trabalho);
//  2. o que conecta esta maquina a outra;
//  3. o que abre uma porta / expoe um servico aqui;
//  4. o que precisa de elevacao de privilegio;
//  5. o que instala ou remove software no sistema.
//
// Fora disso ele trabalha sozinho: grep, find, sed, nano, go build, go test,
// git status/add/commit, mkdir, cp, touch. Perguntar a cada busca transforma o
// "permitir" em reflexo, e ai a pergunta que importa passa despercebida.
//
// Devolver o motivo (e nao um booleano) e o que permite a tela dizer o que
// esta em jogo: "apaga arquivos" e "conecta a outra maquina" merecem leituras
// diferentes.
func MotivoDeConfirmacao(command string) string {
	normalizado := strings.ToLower(strings.TrimSpace(command))

	for _, c := range confirmacoes {
		if c.re.MatchString(normalizado) {
			return c.motivo
		}
	}
	return ""
}

var confirmacoes = []struct {
	re     *regexp.Regexp
	motivo string
}{
	// 4. Elevacao vem primeiro: "sudo rm" e melhor descrito como elevacao.
	{regexp.MustCompile(`(^|[;&|]\s*)(sudo|su|doas|pkexec)\b`),
		"pede elevação de privilégio"},

	// 1. Irreversivel.
	{removeArquivos, "apaga ou desfaz trabalho"},

	// 3. Abre porta / expoe servico nesta maquina.
	{regexp.MustCompile(`\b(ngrok|localtunnel)\b` +
		`|\b(nc|ncat|netcat)\b[^|]*\s-[a-z]*l` +
		`|\bsocat\b` +
		`|\bpython[0-9.]*\s+-m\s+http\.server\b` +
		`|\bphp\s+-s\b` +
		`|\bssh\b[^|]*\s-[lrd]\s` +
		`|\bdocker\s+run\b[^|]*\s-p\s`),
		"abre uma porta nesta máquina"},

	// 2. Conecta a outra maquina.
	{regexp.MustCompile(`(^|[;&|]\s*)(ssh|scp|sftp|telnet|ftp|nc|ncat|netcat|curl|wget)\b` +
		`|\brsync\b[^|]*\s[^\s:]+:` +
		`|\bgit\s+(push|clone|pull|fetch)\b` +
		`|\b(docker|podman)\s+(push|pull)\b` +
		`|\bnpm\s+publish\b|\btwine\s+upload\b`),
		"conecta a outra máquina"},

	// 5. Instala ou remove software no sistema.
	{regexp.MustCompile(`\b(apt|apt-get|dnf|yum|pacman|apk|zypper|brew)\b[^|]*\b(install|remove|upgrade|-s|-r|-u)\b` +
		`|\bnpm\s+(i|install)\b[^|]*\s-g\b|\bnpm\s+-g\b` +
		`|\bpip[0-9.]*\s+(install|uninstall)\b`),
		// "go install ./cmd/x" fica de fora: compila um binario do proprio
		// projeto para o GOBIN do usuario. E rotina de Go, igual a "make
		// install" -- nao mexe em software do sistema.
		"instala ou remove software"},
}
