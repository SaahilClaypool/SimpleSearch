package agent

import (
	"fmt"
	"log"
)

func Research(input string, numSearches int, numRead int) (ResearchResult, error) {
	result := ResearchResult{}
	webSearchAgent := Make(`
		You are an assistant who will help the main agent to a query in various ways.
		The Original Request is the users original request.
		Only use this to determine how best to answer your specific task.
		Do not make up or add information except from the information provided.
		`, false)

	// 1. do you need any information
	searches := &Searches{}
	err := webSearchAgent.Json(fmt.Sprintf(`
		What google search queries would you perform to answer this query?
		Provide up to %d queries.
		Make the queries varied, targetting different parts of the users request if it's multi part.
		Target different domain specific keywords or sites (for example, 'stackoverflow.com how do I invert a linked list?'.
		----
		Original Request: %s`, numSearches, input), searches)
	if err != nil {
		return result, err
	}

	ctx := ""
	for _, q := range searches.Queries {
		result.Queries = append(result.Queries, q)
		webResults, err := Search(q)
		if err != nil {
			log.Printf("Warning: %v\n", err)
			continue
		}
		mdResults := FormatSearchResultsAsMarkdown(webResults)
		clickableUrls := &Urls{}
		err = webSearchAgent.Json(fmt.Sprintf(`
		Which urls in this result set are worth clicking on to answer the request below?
		You can only click on up to %d.
		----
		Original Request: %s
		`, numRead, mdResults), clickableUrls)
		if err != nil {
			log.Printf("Warning: %v\n", err)
			continue
		}
		for _, url := range clickableUrls.Urls {
			content, err := Article(url)
			if err != nil {
				log.Printf("Warning: %v\n", err)
				continue
			}
			relevant, err := webSearchAgent(fmt.Sprintf(`
				What part of the content below is useful to answer the original request?
				Write out the relevant sections verbatim.
				If no parts are relevant, respond with just "Irrelevant"
				Original Request: %s
				==========
				%s
				`, input, content.TextContent), "", false)
			if err != nil {
				log.Printf("Warning, failed to extract context: %v\n", err)
				continue
			}
			if startsWith(relevant, "Irrelvant") {
				log.Printf("Warning, no relevant section found")
				continue
			}
			ctx = fmt.Sprintf("%s\n===============\n[%s](%s)\n%s", ctx, content.Title, url, relevant)
			result.References = append(result.References, Reference{Title: content.Title, Url: url, Context: relevant, Full: content.TextContent})
		}
	}

	ag := Make(fmt.Sprintf("Answer the users request using the following context. Cite your answers with the title & url of the cite you pulled information from:\n\n%s", ctx), true)
	resp, err := ag(input, "", false)
	if err != nil {
		return result, err
	}

	result.Answer = resp

	return result, nil
}

type Searches struct {
	Queries []string
}

type Urls struct {
	Urls []string
}

type ResearchResult struct {
	Answer     string
	References []Reference
	Queries    []string
}
type Reference struct {
	Title   string
	Url     string
	Context string
	Full    string
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
