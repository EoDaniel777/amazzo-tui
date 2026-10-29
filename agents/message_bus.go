package agents

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MessageType representa o tipo de mensagem inter-agentes.
type MessageType string

const (
	MessageTypeRequest      MessageType = "request"
	MessageTypeResponse     MessageType = "response"
	MessageTypeNotification MessageType = "notification"
	MessageTypeBroadcast    MessageType = "broadcast"
)

// AgentMessage representa uma mensagem entre agentes.
type AgentMessage struct {
	ID        string                 `json:"id"`
	From      string                 `json:"from"`
	To        string                 `json:"to"` // "*" para broadcast
	Type      MessageType            `json:"type"`
	Subject   string                 `json:"subject"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp time.Time              `json:"timestamp"`
	ReplyTo   string                 `json:"reply_to,omitempty"`
}

// MessageHandler é uma função que processa mensagens.
type MessageHandler func(msg AgentMessage) error

// MessageBus gerencia comunicação entre agentes.
type MessageBus struct {
	channels map[string]chan AgentMessage // agentID -> canal
	handlers map[string]map[string]MessageHandler // agentID -> subject -> handler
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewMessageBus cria um novo message bus.
func NewMessageBus() *MessageBus {
	ctx, cancel := context.WithCancel(context.Background())
	return &MessageBus{
		channels: make(map[string]chan AgentMessage),
		handlers: make(map[string]map[string]MessageHandler),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// RegisterAgent registra um agente no message bus.
func (mb *MessageBus) RegisterAgent(agentID string, bufferSize int) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if _, exists := mb.channels[agentID]; exists {
		return fmt.Errorf("agente %s já registrado", agentID)
	}

	mb.channels[agentID] = make(chan AgentMessage, bufferSize)
	mb.handlers[agentID] = make(map[string]MessageHandler)

	// Iniciar goroutine para processar mensagens
	go mb.processMessages(agentID)

	return nil
}

// UnregisterAgent remove um agente do message bus.
func (mb *MessageBus) UnregisterAgent(agentID string) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if ch, exists := mb.channels[agentID]; exists {
		close(ch)
		delete(mb.channels, agentID)
		delete(mb.handlers, agentID)
	}
}

// RegisterHandler registra um handler para um subject específico.
func (mb *MessageBus) RegisterHandler(agentID, subject string, handler MessageHandler) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if _, exists := mb.handlers[agentID]; !exists {
		return fmt.Errorf("agente %s não registrado", agentID)
	}

	mb.handlers[agentID][subject] = handler
	return nil
}

// Send envia uma mensagem para um agente específico.
func (mb *MessageBus) Send(msg AgentMessage) error {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	msg.Timestamp = time.Now()
	if msg.ID == "" {
		msg.ID = fmt.Sprintf("%s_%d", msg.From, time.Now().UnixNano())
	}

	// Broadcast
	if msg.To == "*" {
		return mb.broadcast(msg)
	}

	// Enviar para agente específico
	ch, exists := mb.channels[msg.To]
	if !exists {
		return fmt.Errorf("agente destinatário %s não encontrado", msg.To)
	}

	select {
	case ch <- msg:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout ao enviar mensagem para %s", msg.To)
	}
}

// broadcast envia uma mensagem para todos os agentes.
func (mb *MessageBus) broadcast(msg AgentMessage) error {
	for agentID, ch := range mb.channels {
		if agentID == msg.From {
			continue // Não enviar para si mesmo
		}

		select {
		case ch <- msg:
		case <-time.After(1 * time.Second):
			// Timeout em broadcast não é erro crítico
			continue
		}
	}

	return nil
}

// Request envia uma requisição e aguarda resposta.
func (mb *MessageBus) Request(from, to, subject string, payload map[string]interface{}, timeout time.Duration) (AgentMessage, error) {
	// Criar canal para resposta
	responseCh := make(chan AgentMessage, 1)
	requestID := fmt.Sprintf("%s_%d", from, time.Now().UnixNano())

	// Registrar handler temporário para a resposta
	mb.mu.Lock()
	if _, exists := mb.handlers[from]; !exists {
		mb.mu.Unlock()
		return AgentMessage{}, fmt.Errorf("agente remetente %s não registrado", from)
	}

	originalHandler := mb.handlers[from][subject]
	mb.handlers[from][requestID] = func(msg AgentMessage) error {
		responseCh <- msg
		return nil
	}
	mb.mu.Unlock()

	// Restaurar handler original após timeout
	defer func() {
		mb.mu.Lock()
		if originalHandler != nil {
			mb.handlers[from][subject] = originalHandler
		} else {
			delete(mb.handlers[from], requestID)
		}
		mb.mu.Unlock()
	}()

	// Enviar requisição
	msg := AgentMessage{
		ID:      requestID,
		From:    from,
		To:      to,
		Type:    MessageTypeRequest,
		Subject: subject,
		Payload: payload,
	}

	if err := mb.Send(msg); err != nil {
		return AgentMessage{}, err
	}

	// Aguardar resposta
	select {
	case response := <-responseCh:
		return response, nil
	case <-time.After(timeout):
		return AgentMessage{}, fmt.Errorf("timeout aguardando resposta de %s", to)
	}
}

// Reply envia uma resposta para uma mensagem.
func (mb *MessageBus) Reply(originalMsg AgentMessage, payload map[string]interface{}) error {
	response := AgentMessage{
		From:    originalMsg.To,
		To:      originalMsg.From,
		Type:    MessageTypeResponse,
		Subject: originalMsg.Subject,
		Payload: payload,
		ReplyTo: originalMsg.ID,
	}

	return mb.Send(response)
}

// processMessages processa mensagens de um agente.
func (mb *MessageBus) processMessages(agentID string) {
	mb.mu.RLock()
	ch := mb.channels[agentID]
	mb.mu.RUnlock()

	for {
		select {
		case <-mb.ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			// Procurar handler para o subject
			mb.mu.RLock()
			handlers := mb.handlers[agentID]
			handler, exists := handlers[msg.Subject]
			if !exists {
				// Tentar handler para reply
				if msg.ReplyTo != "" {
					handler, exists = handlers[msg.ReplyTo]
				}
			}
			mb.mu.RUnlock()

			if exists && handler != nil {
				if err := handler(msg); err != nil {
					// Log error (em produção, usar logger apropriado)
					fmt.Printf("[MessageBus] Erro ao processar mensagem: %v\n", err)
				}
			}
		}
	}
}

// GetPendingCount retorna o número de mensagens pendentes de um agente.
func (mb *MessageBus) GetPendingCount(agentID string) int {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	if ch, exists := mb.channels[agentID]; exists {
		return len(ch)
	}

	return 0
}

// Shutdown encerra o message bus.
func (mb *MessageBus) Shutdown() {
	mb.cancel()

	mb.mu.Lock()
	defer mb.mu.Unlock()

	for agentID, ch := range mb.channels {
		close(ch)
		delete(mb.channels, agentID)
		delete(mb.handlers, agentID)
	}
}
