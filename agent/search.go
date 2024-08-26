package agent

import (
	"fmt"
	"log"

	"github.com/samber/lo"
)

func Research(input string, numSearches int, numRead int) (ResearchResult, error) {
	result := ResearchResult{}
	webSearchAgent := Make(`
		You are an assistant who will help the main agent to a query in various ways.
		The Original Request is the users original request.
		Only use this to determine how best to answer your specific task.
		Do not make up or add information except from the information provided.
		
		Before answering, write down your thoughts before answering.
		Write up to 5 thoughts. Each thought should be a single line in a json array.
		`, false)

	// 1. do you need any information
	searches := &Response[Searches]{Thoughts: []string{"{thought}"}, Answer: Searches{Queries: []string{"{query}"}}}
	err := webSearchAgent.Json(fmt.Sprintf(`
		# Original Request:

		%s
		
		# Question

		What google search queries would you perform to answer this query?
		Provide up to %d queries.
		Make the queries varied, targetting different parts of the users request if it's multi part.
		Target different domain specific keywords or sites (for example, 'stackoverflow.com how do I invert a linked list?'.
		`, input, numSearches), searches)
	if err != nil {
		return result, err
	}

	searchContext := ""
	for _, q := range searches.Answer.Queries {
		result.Queries = append(result.Queries, q)
		queryContext, queryReferences, err := processQuery(q, input, webSearchAgent)
		if err != nil {
			log.Printf("Warning: %v\n", err)
			continue
		}
		searchContext += queryContext
		result.References = append(result.References, queryReferences...)
	}

	ag := Make(fmt.Sprintf(`
	# Context

	%s

	# Directions

	First, synthesize your thoughts as a bulleted list. Think about the outline for how you'd provide your answer.
	Answer the users request using the context above.
	Cite your answers with the title & url of the cite you pulled information from.
	Use markdown sections to break up response. Use bulleted lists where possible.
	Be specific and cite key statistics in your answer where possible.
	Only include information from the context above.

	Use this format:

	# Thoughts

	think about how you'll answer the question, what sources you'll cite.
	Write down your notes.

	# Outline

	outline for your answer
	
	# Response

	Write your full markdown response with sections here. This should be a long (2+ page) answer fulling explaining your answer.`, searchContext), true)
	resp, err := ag(input, "", false)
	if err != nil {
		return result, err
	}

	result.Answer = resp

	return result, nil
}

type Response[T any] struct {
	Thoughts []string
	Answer   T
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
func processQuery(query string, input string, webSearchAgent LLM) (string, []Reference, error) {
	var searchContext string
	var references []Reference

	webResults, err := Search(query)
	if err != nil {
		return "", nil, fmt.Errorf("error searching for query: %v", err)
	}

	for _, url := range lo.Slice(webResults, 0, 3) {
		article, err := Article(url.Url)
		if err != nil {
			log.Printf("Warning: %v\n", err)
			continue
		}
		relevant, err := webSearchAgent(fmt.Sprintf(`
			# Search Results

			%s

			# Original Request

			%s

			# Question

			What part of the content below is useful to answer the original request?
			Write out the relevant sections verbatim.
			If no parts are relevant, respond with just "Irrelevant"
			If its relevant, include a short summary of the full article at the bottom.
			`, article.TextContent, input), "", false)
		if err != nil {
			log.Printf("Warning, failed to extract context: %v\n", err)
			continue
		}
		if startsWith(relevant, "Irrelvant") {
			log.Printf("Warning, no relevant section found")
			continue
		}
		searchContext = fmt.Sprintf("%s\n===============\n[%s](%s)\n%s", searchContext, article.Title, url.Url, relevant)
		references = append(references, Reference{Title: article.Title, Url: url.Url, Context: relevant, Full: article.TextContent})
	}

	return searchContext, references, nil
}
