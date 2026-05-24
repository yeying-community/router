package message

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yeying-community/router/common/config"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type request struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Content     string `json:"content"`
	URL         string `json:"url"`
	Channel     string `json:"channel"`
	Token       string `json:"token"`
}

type response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func SendMessage(title string, description string, content string) error {
	provider := strings.ToLower(strings.TrimSpace(config.NotifyProvider))
	if provider == "" {
		return errors.New("notify provider is not set")
	}
	switch provider {
	case "message_pusher":
		return sendMessagePusher(title, description, content)
	case "dingtalk":
		return sendDingTalkMessage(title, description, content)
	case "lark":
		return sendLarkMessage(title, description, content)
	default:
		return fmt.Errorf("unsupported notify provider: %s", provider)
	}
}

func sendMessagePusher(title string, description string, content string) error {
	if config.NotifyWebhookURL == "" {
		return errors.New("notify webhook url is not set")
	}
	req := request{
		Title:       title,
		Description: description,
		Content:     content,
		Token:       config.NotifyToken,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	resp, err := http.Post(config.NotifyWebhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var res response
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return err
	}
	if !res.Success {
		return errors.New(res.Message)
	}
	return nil
}

func sendDingTalkMessage(title string, description string, content string) error {
	if config.NotifyWebhookURL == "" {
		return errors.New("notify webhook url is not set")
	}
	requestURL := strings.TrimSpace(config.NotifyWebhookURL)
	if secret := strings.TrimSpace(config.NotifySecret); secret != "" {
		timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
		stringToSign := timestamp + "\n" + secret
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(stringToSign))
		sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		separator := "&"
		if !strings.Contains(requestURL, "?") {
			separator = "?"
		}
		requestURL = fmt.Sprintf("%s%stimestamp=%s&sign=%s", requestURL, separator, timestamp, sign)
	}
	body := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  formatRobotMarkdown(title, description, content),
		},
	}
	return postJSONWithRobotResponse(requestURL, body)
}

func sendLarkMessage(title string, description string, content string) error {
	if config.NotifyWebhookURL == "" {
		return errors.New("notify webhook url is not set")
	}
	body := map[string]any{
		"msg_type": "text",
		"content": map[string]string{
			"text": formatRobotText(title, description, content),
		},
	}
	if secret := strings.TrimSpace(config.NotifySecret); secret != "" {
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		stringToSign := timestamp + "\n" + secret
		mac := hmac.New(sha256.New, []byte(stringToSign))
		_, _ = mac.Write([]byte{})
		body["timestamp"] = timestamp
		body["sign"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	return postJSONWithRobotResponse(strings.TrimSpace(config.NotifyWebhookURL), body)
}

func postJSONWithRobotResponse(requestURL string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(requestURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notify request failed: http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var envelope struct {
		ErrCode       int    `json:"errcode"`
		ErrMsg        string `json:"errmsg"`
		Code          int    `json:"code"`
		Msg           string `json:"msg"`
		StatusCode    int    `json:"StatusCode"`
		StatusMessage string `json:"StatusMessage"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		if envelope.ErrCode != 0 {
			return fmt.Errorf("notify request failed: %s", strings.TrimSpace(envelope.ErrMsg))
		}
		if envelope.Code != 0 {
			return fmt.Errorf("notify request failed: %s", strings.TrimSpace(envelope.Msg))
		}
		if envelope.StatusCode != 0 {
			return fmt.Errorf("notify request failed: %s", strings.TrimSpace(envelope.StatusMessage))
		}
	}
	return nil
}

func formatRobotMarkdown(title string, description string, content string) string {
	text := normalizeTextContent(description, content)
	if text == "" {
		return "### " + strings.TrimSpace(title)
	}
	return "### " + strings.TrimSpace(title) + "\n\n" + strings.ReplaceAll(text, "\n", "\n\n")
}

func formatRobotText(title string, description string, content string) string {
	text := normalizeTextContent(description, content)
	if text == "" {
		return strings.TrimSpace(title)
	}
	return strings.TrimSpace(title) + "\n" + text
}

var htmlTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)

func normalizeTextContent(description string, content string) string {
	plain := strings.TrimSpace(description)
	if plain == "" {
		plain = strings.TrimSpace(content)
	}
	plain = html.UnescapeString(htmlTagPattern.ReplaceAllString(plain, "\n"))
	lines := strings.Split(plain, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		normalized = append(normalized, line)
	}
	return strings.Join(normalized, "\n")
}
