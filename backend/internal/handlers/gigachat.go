package handlers

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"invoicekeeper/internal/apperr"
)

type gigaChatHandler struct {
	AccessToken string
	ExpiresAt   time.Time
}

type gigaTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   int64  `json:"expires_at"`
}

type gigaCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (s *Server) chatWithGigaChat(c *gin.Context) {
	if !s.Cfg.GigaChatConfigured() {
		writeAppError(c, apperr.New(http.StatusServiceUnavailable, "GigaChat не настроен", "GIGACHAT_NOT_CONFIGURED"))
		return
	}
	var body struct {
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Message) == "" {
		writeAppError(c, apperr.ValidationError)
		return
	}
	if len([]rune(body.Message)) > 4000 {
		writeAppError(c, apperr.New(http.StatusBadRequest, "Сообщение слишком длинное", "MESSAGE_TOO_LONG"))
		return
	}

	token, err := s.gigaChatToken()
	if err != nil {
		writeAppError(c, apperr.New(http.StatusBadGateway, "Не удалось получить токен GigaChat", "GIGACHAT_AUTH_FAILED"))
		return
	}
	payload := map[string]interface{}{
		"model": s.Cfg.GigaChatModel,
		"messages": []map[string]string{
			{"role": "system", "content": "Ты полезный ассистент приложения Invoice Keeper. Отвечай по-русски кратко и понятно. Не выдумывай данные о платежах пользователя, если они не переданы в сообщении."},
			{"role": "user", "content": body.Message},
		},
		"temperature": 0.3,
	}
	encoded, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, strings.TrimRight(s.Cfg.GigaChatAPIURL, "/")+"/chat/completions", bytes.NewReader(encoded))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.gigaHTTPClient().Do(req)
	if err != nil {
		writeAppError(c, apperr.New(http.StatusBadGateway, "GigaChat недоступен", "GIGACHAT_UNAVAILABLE"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeAppError(c, apperr.New(http.StatusBadGateway, fmt.Sprintf("GigaChat вернул HTTP %d", resp.StatusCode), "GIGACHAT_ERROR"))
		return
	}
	var result gigaCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		writeAppError(c, apperr.New(http.StatusBadGateway, "GigaChat вернул пустой ответ", "GIGACHAT_INVALID_RESPONSE"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": result.Choices[0].Message.Content})
}

func (s *Server) gigaChatToken() (string, error) {
	form := url.Values{}
	form.Set("scope", s.Cfg.GigaChatScope)
	req, err := http.NewRequest(http.MethodPost, s.Cfg.GigaChatAuthURL, strings.NewReader(form.Encode()))
	if err != nil { return "", err }
	req.Header.Set("Authorization", "Basic "+s.Cfg.GigaChatCredentials)
	req.Header.Set("RqUID", uuid.NewString())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.gigaHTTPClient().Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return "", fmt.Errorf("oauth status %d", resp.StatusCode) }
	var token gigaTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil || token.AccessToken == "" { return "", fmt.Errorf("invalid oauth response") }
	return token.AccessToken, nil
}

func (s *Server) gigaHTTPClient() *http.Client {
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: !s.Cfg.GigaChatVerifyTLS}}
	return &http.Client{Timeout: 30 * time.Second, Transport: transport}
}
