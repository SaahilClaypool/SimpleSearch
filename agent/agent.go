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
	fn := func(prompt string, sys2 string, json bool) (string, error) {
		config := openai.DefaultConfig(key)
		config.BaseURL = endpoint
		sys2 = StripLinePadding(sysMsg + "\n" + sys2)
		prompt = StripLinePadding(prompt)
		client := openai.NewClientWithConfig(config)
		log.Printf("[System]: %s\n[User]: %s\n", sys2, prompt)
		responseFormat := &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeText}
		if json {
			responseFormat.Type = openai.ChatCompletionResponseFormatTypeJSONObject
		}
		resp, err := RetryR(3, func() (openai.ChatCompletionResponse, error) {
			return client.CreateChatCompletion(
				context.Background(),
				openai.ChatCompletionRequest{
					Model:          chatModel,
					ResponseFormat: responseFormat,
					Messages: []openai.ChatCompletionMessage{
						{
							Role:    openai.ChatMessageRoleSystem,
							Content: sys2,
						},
						{
							Role:    openai.ChatMessageRoleUser,
							Content: prompt,
						},
					},
				})
		})

		if err != nil {
			return "", fmt.Errorf("Chat completion error: %w", err)
		}

		content := resp.Choices[0].Message.Content
		log.Printf("[Assistant]: %s\n", content)
		return content, nil
	}

	return fn
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

func StripLinePadding(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimLeft(line, " \t")
	}
	return strings.Join(lines, "\n")
}

func Retry[Tfunc func() error](times int, fun Tfunc) error {
	var err error
	for i := 0; i < times; i++ {
		err = fun()
		if err == nil {
			return nil
		}
	}
	return err
}

func RetryR[K any, Tfunc func() (K, error)](times int, fun Tfunc) (K, error) {
	var result K
	var err error

	err = Retry(times, func() error {
		var tempErr error
		result, tempErr = fun()
		return tempErr
	})

	return result, err
}
