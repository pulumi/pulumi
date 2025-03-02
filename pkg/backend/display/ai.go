package display

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// OpenAIResponse represents the structure of the OpenAI API response
type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// OpenAIRequest represents the structure of the OpenAI API request
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func summarize(lines []string) string {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "Error: OpenAI API key not found in environment variables"
	}

	// Join all lines, but keep relevant formatting
	content := strings.Join(lines, "\n")

	// Create the system message that explains the task
	systemMessage := `You are analyzing Pulumi diagnostic output. Please provide a concise (no more than 80 character) summary of the diagnostic messages, focusing on
what went wrong and how to fix it.`

	// Prepare the request
	requestBody := OpenAIRequest{
		Model: "gpt-4o",
		Messages: []OpenAIMessage{
			{
				Role:    "system",
				Content: systemMessage,
			},
			{
				Role:    "user",
				Content: content,
			},
		},
		Temperature: 0.3, // Lower temperature for more consistent output
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Sprintf("Error preparing request: %v", err)
	}

	// Create the HTTP request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Sprintf("Error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Error making request: %v", err)
	}
	defer resp.Body.Close()

	// Parse the response
	var openAIResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return fmt.Sprintf("Error parsing response: %v", err)
	}

	// Check for API errors
	if openAIResp.Error.Message != "" {
		return fmt.Sprintf("OpenAI API error: %s", openAIResp.Error.Message)
	}

	// Check if we got any choices back
	if len(openAIResp.Choices) == 0 {
		return "Error: No summary generated"
	}

	return openAIResp.Choices[0].Message.Content
}
