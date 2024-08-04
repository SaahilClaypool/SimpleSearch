package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/go-shiori/go-readability"
)

func getRawHTML(url string) (string, error) {
	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error fetching webpage: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	return string(body), nil
}
func Article(urlStr string) (*readability.Article, error) {
	// Fetch the raw HTML content from the URL
	htmlContent, err := getRawHTML(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch HTML content from %s: %v", urlStr, err)
	}

	// Parse the URL to pass it to readability.FromReader
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %v", err)
	}

	// Convert the HTML content string to a reader
	htmlReader := strings.NewReader(htmlContent)

	// Parse the article using readability
	article, err := readability.FromReader(htmlReader, parsedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse article from %s: %v", urlStr, err)
	}

	return &article, nil
}

func Search(query string) ([]SearchResult, error) {
	url := "https://google.serper.dev/search"
	method := "POST"

	payload := strings.NewReader(fmt.Sprintf(`{"q":"%s"}`, query))

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	req.Header.Add("X-API-KEY", os.Getenv("SERPER_KEY"))
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	var data map[string]any
	err = json.Unmarshal([]byte(body), &data)
	if err != nil {
		return nil, err
	}
	output := make([]SearchResult, 0)
	for _, result := range (data["organic"]).([]any) {
		fmt.Printf("Result: %v\n", result)
		resultMap := result.(map[string]any)
		searchResult := SearchResult{
			Url:     resultMap["link"].(string),
			Title:   resultMap["title"].(string),
			Summary: resultMap["snippet"].(string),
		}
		output = append(output, searchResult)

		// Check for sitelinks and map them as well
		if sitelinks, ok := resultMap["sitelinks"].([]any); ok {
			for _, sitelink := range sitelinks {
				sitelinkMap := sitelink.(map[string]any)
				sitelinkResult := SearchResult{
					Url:     sitelinkMap["link"].(string),
					Title:   sitelinkMap["title"].(string),
					Summary: resultMap["snippet"].(string),
				}
				output = append(output, sitelinkResult)
			}
		}
	}
	return output, nil
}

type SearchResult struct {
	Url     string
	Title   string
	Summary string
}

func FormatSearchResultsAsMarkdown(results []SearchResult) string {
	var markdown strings.Builder
	for _, result := range results {
		markdown.WriteString(fmt.Sprintf("- [%s](%s)\n", result.Title, result.Url))
		markdown.WriteString(fmt.Sprintf("  %s\n", result.Summary))
	}
	return markdown.String()
}
