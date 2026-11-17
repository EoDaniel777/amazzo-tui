package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"amazzo/agents"
)

// Provedores dos modelos remotos.
const (
	providerAmazzo = "amazzo"
	providerHF     = "hf"
)

// cloudModels são os modelos do amazzo.tech (API compatível com OpenAI).
// O id é o que o endpoint espera; "só texto" avisa que não lê imagem.
var cloudModels = []agents.ModelConfig{
	{Name: "amazzo", DisplayName: "Amazzo", Endpoint: "amazzo"},
	{Name: "autom-tico", DisplayName: "Automático", Endpoint: "autom-tico"},
	{Name: "claude-sonnet-5.5", DisplayName: "Claude Sonnet 5.5", Endpoint: "claude-sonnet-5.5"},
	{Name: "claude-opus-5.5", DisplayName: "Claude Opus 5.5", Endpoint: "claude-opus-5.5"},
	{Name: "claude-fable-5.2", DisplayName: "Claude Fable 5.2", Endpoint: "claude-fable-5.2"},
	{Name: "gpt-6-astra", DisplayName: "GPT-6 Astra", Endpoint: "gpt-6-astra"},
	{Name: "gemini-4-argon", DisplayName: "Gemini 4 Argon", Endpoint: "gemini-4-argon"},
	{Name: "gemini-4-carbon", DisplayName: "Gemini 4 Carbon", Endpoint: "gemini-4-carbon"},
	{Name: "muse-glimmer", DisplayName: "Muse Glimmer", Endpoint: "muse-glimmer"},
	{Name: "kimi-k3", DisplayName: "Kimi K3", Endpoint: "kimi-k3"},
	{Name: "minimax-m3", DisplayName: "MiniMax M3", Endpoint: "minimax-m3"},
	{Name: "gemma-4-31b", DisplayName: "Gemma 4 31B", Endpoint: "gemma-4-31b"},
	{Name: "dots-3", DisplayName: "Dots 3", Endpoint: "dots-3"},
	{Name: "tencent-hy4", DisplayName: "Tencent Hy4", Endpoint: "tencent-hy4"},
	{Name: "deepseek-v4.1-flash", DisplayName: "DeepSeek V4.1 Flash", Endpoint: "deepseek-v4.1-flash"},
	{Name: "glm-5.3", DisplayName: "GLM 5.3 · só texto", Endpoint: "glm-5.3"},
	{Name: "qwen3-coder", DisplayName: "Qwen3 Coder · só texto", Endpoint: "qwen3-coder"},
	{Name: "cohere-north-mini", DisplayName: "Cohere North Mini · só texto", Endpoint: "cohere-north-mini"},
	{Name: "deepseek-v4-pro", DisplayName: "DeepSeek V4 Pro · só texto", Endpoint: "deepseek-v4-pro"},
}

// cloudAccess é o endereço e a chave do amazzo.tech.
type cloudAccess struct {
	BaseURL string
	Key     string
}

// loadCloudAccess lê a configuração do mesmo lugar do opencode (provider
// "amazzo"). AMAZZO_API_KEY e AMAZZO_BASE_URL têm prioridade. A chave nunca é
// impressa nem gravada em outro arquivo.
func loadCloudAccess() cloudAccess {
	acc := cloudAccess{BaseURL: "https://amazzo.tech/v1"}

	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, ".config", "opencode", "opencode.json"),
	} {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var doc struct {
			Provider map[string]struct {
				Options struct {
					BaseURL string `json:"baseURL"`
					APIKey  string `json:"apiKey"`
				} `json:"options"`
			} `json:"provider"`
		}
		if json.Unmarshal(raw, &doc) != nil {
			continue
		}
		if prov, ok := doc.Provider["amazzo"]; ok {
			if prov.Options.BaseURL != "" {
				acc.BaseURL = prov.Options.BaseURL
			}
			if prov.Options.APIKey != "" {
				acc.Key = prov.Options.APIKey
			}
		}
	}
	if v := os.Getenv("AMAZZO_BASE_URL"); v != "" {
		acc.BaseURL = v
	}
	if v := os.Getenv("AMAZZO_API_KEY"); v != "" {
		acc.Key = v
	}
	acc.BaseURL = strings.TrimRight(acc.BaseURL, "/")
	return acc
}

// composeModels monta a lista: locais (Ollama), depois amazzo.tech, depois
// Hugging Face (só se houver token).
func composeModels(local []agents.ModelConfig, hf bool) []agents.ModelConfig {
	out := make([]agents.ModelConfig, 0, len(local)+len(cloudModels)+len(remoteModels))
	for _, c := range local {
		c.Provider = "ollama"
		out = append(out, c)
	}
	for _, c := range cloudModels {
		c.Provider = providerAmazzo
		out = append(out, c)
	}
	if hf {
		for _, c := range remoteModels {
			c.Provider = providerHF
			out = append(out, c)
		}
	}
	return out
}

// modelGroup diz em qual seção do menu o modelo aparece.
func modelGroup(cfg agents.ModelConfig) string {
	if isLocalModel(cfg) {
		return "Local · roda na sua GPU"
	}
	if cfg.Provider == providerHF {
		return "Nuvem · Hugging Face"
	}
	return "Nuvem · amazzo.tech"
}

// remoteTarget devolve a URL e o token do modelo remoto atual.
func (m *Model) remoteTarget(cfg agents.ModelConfig) (url, token string, err error) {
	if cfg.Provider == providerAmazzo {
		if m.cloud.Key == "" {
			return "", "", fmt.Errorf("%s precisa da chave do amazzo.tech.\n"+
				"Defina AMAZZO_API_KEY, ou configure o provider \"amazzo\" em ~/.config/opencode/opencode.json.\n"+
				"Use /modelo para escolher um modelo local", cfg.DisplayName)
		}
		return m.cloud.BaseURL + "/chat/completions", m.cloud.Key, nil
	}
	if strings.TrimSpace(m.hfToken) == "" {
		return "", "", fmt.Errorf("%s é um modelo remoto e precisa de um token do Hugging Face.\n"+
			"Use /key para configurar, ou /modelo para escolher outro modelo", cfg.DisplayName)
	}
	return "https://router.huggingface.co/v1/chat/completions", m.hfToken, nil
}
