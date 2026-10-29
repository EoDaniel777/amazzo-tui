package app

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"
)

// RetryConfig configura o comportamento de retry.
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
}

// DefaultRetryConfig retorna a configuração padrão de retry.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2.0,
	}
}

// RetryableHTTPClient é um cliente HTTP com suporte a retry.
type RetryableHTTPClient struct {
	client *http.Client
	config RetryConfig
}

// NewRetryableHTTPClient cria um novo cliente com retry.
func NewRetryableHTTPClient(timeout time.Duration, config RetryConfig) *RetryableHTTPClient {
	return &RetryableHTTPClient{
		client: &http.Client{Timeout: timeout},
		config: config,
	}
}

// Do executa uma requisição HTTP com retry.
func (c *RetryableHTTPClient) Do(req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calcular backoff exponencial
			backoff := c.calculateBackoff(attempt)

			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(backoff):
				// Continua após o backoff
			}
		}

		// Fazer a requisição
		resp, err := c.client.Do(req)

		// Se não houve erro e status é OK, retornar
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// Armazenar erro para última tentativa
		if err != nil {
			lastErr = fmt.Errorf("tentativa %d falhou: %w", attempt+1, err)
		} else {
			lastErr = fmt.Errorf("tentativa %d falhou: status %d", attempt+1, resp.StatusCode)
			resp.Body.Close()
		}
	}

	return nil, fmt.Errorf("falhou após %d tentativas: %w", c.config.MaxRetries+1, lastErr)
}

// calculateBackoff calcula o tempo de espera usando backoff exponencial.
func (c *RetryableHTTPClient) calculateBackoff(attempt int) time.Duration {
	backoff := float64(c.config.InitialBackoff) * math.Pow(c.config.Multiplier, float64(attempt-1))

	if backoff > float64(c.config.MaxBackoff) {
		return c.config.MaxBackoff
	}

	return time.Duration(backoff)
}

// DoWithContext executa uma requisição com contexto e retry.
func (c *RetryableHTTPClient) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	return c.Do(req)
}
