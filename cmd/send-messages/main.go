package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Message struct {
	Ts             string `json:"ts"`
	User           string `json:"user"`
	Text           string `json:"text"`
	ConversationID string `json:"conversation_id"`
}

type WebhookRequest struct {
	MsgType string                 `json:"msg_type"`
	Content map[string]interface{} `json:"content"`
}

type WebhookResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func loadWebhooks(filename string) (map[string]string, error) {
	webhooks := make(map[string]string)
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[0]
			url := parts[1]
			webhooks[name] = url
		}
	}

	return webhooks, scanner.Err()
}

func loadMessages(filename string) ([]Message, error) {
	var messages []Message
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &messages)
	return messages, err
}

func sendWebhook(url, text string) error {
	req := WebhookRequest{
		MsgType: "text",
		Content: map[string]interface{}{
			"text": text,
		},
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var webhookResp WebhookResponse
	if err := json.Unmarshal(body, &webhookResp); err != nil {
		return nil
	}

	if webhookResp.Code != 0 {
		return fmt.Errorf("webhook error: %s", webhookResp.Msg)
	}

	return nil
}

func main() {
	jsonFile := flag.String("json", "", "Path to test-messages.json")
	webhookFile := flag.String("webhooks", "", "Path to webhooks.md")
	startIndex := flag.Int("start", 0, "Start index (default 0)")
	count := flag.Int("count", 0, "Number of messages to send (0 = all)")
	delay := flag.Duration("delay", 1*time.Second, "Delay between messages")

	flag.Parse()

	if *jsonFile == "" || *webhookFile == "" {
		fmt.Println("Usage:")
		flag.PrintDefaults()
		fmt.Println("\nExample:")
		fmt.Println("  go run main.go -json ../outputs/test-messages.json -webhooks ../webhooks.md -count 100")
		os.Exit(1)
	}

	// Load webhooks
	webhooks, err := loadWebhooks(*webhookFile)
	if err != nil {
		fmt.Printf("Error loading webhooks: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Loaded %d webhooks\n", len(webhooks))
	if len(webhooks) > 0 {
		names := make([]string, 0, 5)
		for name := range webhooks {
			if len(names) >= 5 {
				break
			}
			names = append(names, name)
		}
		fmt.Printf("Users: %v...\n", names)
	}
	fmt.Println()

	// Load messages
	messages, err := loadMessages(*jsonFile)
	if err != nil {
		fmt.Printf("Error loading messages: %v\n", err)
		os.Exit(1)
	}

	// Determine range
	endIndex := len(messages)
	if *count > 0 {
		endIndex = *startIndex + *count
		if endIndex > len(messages) {
			endIndex = len(messages)
		}
	}

	messagesToSend := messages[*startIndex:endIndex]
	fmt.Printf("Sending %d messages (from %d to %d)\n", len(messagesToSend), *startIndex, endIndex-1)
	fmt.Println("------------------------------------------------------------")

	successCount := 0
	for i, msg := range messagesToSend {
		idx := *startIndex + i
		user := msg.User
		text := msg.Text

		webhookURL, ok := webhooks[user]
		if !ok {
			fmt.Printf("[%3d/%3d] Skipping %-15s - no webhook found\n", idx+1, len(messages), user)
			continue
		}

		fmt.Printf("[%3d/%3d] %-15s: ", idx+1, len(messages), user)

		if err := sendWebhook(webhookURL, text); err != nil {
			fmt.Printf("Failed! %v\n", err)
		} else {
			if len(text) > 45 {
				fmt.Printf("%s...\n", text[:45])
			} else {
				fmt.Printf("%s\n", text)
			}
			successCount++
		}

		// Delay between messages
		if i < len(messagesToSend)-1 {
			time.Sleep(*delay)
		}
	}

	fmt.Println("------------------------------------------------------------")
	fmt.Printf("Done! Sent %d/%d messages successfully\n", successCount, len(messagesToSend))
}
