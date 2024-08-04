package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/invopop/yaml"
	"github.com/sashabaranov/go-openai"
)

func Make(sysMsg string, big bool) LLM {
	chatModel := os.Getenv("CHAT_MODEL")
	if big {
		chatModel = os.Getenv("CHAT_BIG_MODEL")
	}
	endpoint := os.Getenv("CHAT_ENDPOINT")
	key := os.Getenv("CHAT_KEY")
	return func(prompt string, sys2 string, json bool) (string, error) {
		config := openai.DefaultConfig(key)
		config.BaseURL = endpoint
		sys2 = sysMsg + "\n" + sys2
		client := openai.NewClientWithConfig(config)
		log.Printf("[System]: %s\n[User]: %s\n", sys2, prompt)
		responseFormat := &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeText}
		if json {
			responseFormat.Type = openai.ChatCompletionResponseFormatTypeJSONObject
		}
		resp, err := client.CreateChatCompletion(
			context.Background(),
			openai.ChatCompletionRequest{
				Model:          chatModel,
				ResponseFormat: responseFormat,
				Messages: []openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleSystem,
						Content: strings.TrimSpace(sys2),
					},
					{
						Role:    openai.ChatMessageRoleUser,
						Content: strings.TrimSpace(prompt),
					},
				},
			},
		)

		if err != nil {
			fmt.Printf("ChatCompletion error: %v\n", err)
			return "", err
		}

		content := resp.Choices[0].Message.Content
		log.Printf("[Assistant]: %s\n", content)
		return content, nil
	}
}

type LLM func(prompt string, system string, jsonMode bool) (string, error)

func (llm LLM) Json(prompt string, output any) error {
	schema := jsonschema.Reflect(output)
	schemaString, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}

	sampleString, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	y, err := yaml.JSONToYAML(schemaString)
	if err != nil {
		return err
	}
	resp, err := llm(prompt, fmt.Sprintf("Respond with a single JSON object in a fenced markdown codeblock with the following schema:\n%s\n\nExample:\n```json\n%s\n```", y, sampleString), true)
	if err != nil {
		return err
	}

	jsonResp := ExtractLastCodeBlock(resp)
	if err != nil {
		return err
	}
	err = json.Unmarshal([]byte(jsonResp), output)
	return err
}

func ExtractLastCodeBlock(markdown string) string {
	re := regexp.MustCompile("(?s)```[^\n]*\n(.*?)```")
	matches := re.FindAllStringSubmatch(markdown, -1)

	if len(matches) == 0 {
		return markdown
	}

	lastMatch := matches[len(matches)-1]
	if len(lastMatch) < 2 {
		return markdown
	}

	return lastMatch[1]
}
