package agent

import (
	"fmt"
	"log"

	"github.com/samber/lo"
	"github.com/samber/lo/parallel"
)

func Research(input string, numSearches int, numRead int) (ResearchResult, error) {
	result := ResearchResult{}
	webSearchAgent := makeHelperAgent()

	searches, err := createSearches(webSearchAgent, input, numSearches)
	if err != nil {
		return result, err
	}

	searchContext := ""
	queryResults := make(chan struct {
		context    string
		references []Reference
		err        error
	}, len(searches.Answer.Queries))

	for _, q := range searches.Answer.Queries {
		result.Queries = append(result.Queries, q)
		go func(query string) {
			context, references, err := summarizeQuery(query, input, webSearchAgent)
			queryResults <- struct {
				context    string
				references []Reference
				err        error
			}{context, references, err}
		}(q)
	}

	for range searches.Answer.Queries {
		queryResult := <-queryResults
		if queryResult.err != nil {
			log.Printf("Warning: %v\n", queryResult.err)
			continue
		}
		searchContext += queryResult.context
		result.References = append(result.References, queryResult.references...)
	}

	resp, err := summarizeContext(searchContext, input)
	if err != nil {
		return result, err
	}

	result.Answer = resp

	return result, nil
}

func createSearches(webSearchAgent LLM, input string, numSearches int) (Response[Searches], error) {
	searches := Response[Searches]{Thoughts: []string{"{thought}"}, Answer: Searches{Queries: []string{"{query}"}}}
	err := webSearchAgent.Json(fmt.Sprintf(`
		# Original Request:

		%s
		
		# Question

		What google search queries would you perform to answer this query?
		Provide up to %d queries.
		Make the queries varied, targetting different parts of the users request if it's multi part.
		Target different domain specific keywords or sites (for example, 'stackoverflow.com how do I invert a linked list?'.
		`, input, numSearches), &searches)
	return searches, err
}

func makeHelperAgent() LLM {
	webSearchAgent := Make(`
		You are an assistant who will help the main agent to a query in various ways.
		The Original Request is the users original request.
		Only use this to determine how best to answer your specific task.
		Do not make up or add information except from the information provided.
		
		Before answering, write down your thoughts before answering.
		Write up to 5 thoughts. Each thought should be a single line in a json array.
		`, false)
	return webSearchAgent
}

func summarizeContext(searchContext string, input string) (string, error) {
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
	return resp, err
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
func summarizeQuery(query string, input string, webSearchAgent LLM) (string, []Reference, error) {
	var searchContext string
	var references []Reference

	webResults, err := Search(query)
	if err != nil {
		return "", nil, fmt.Errorf("error searching for query: %v", err)
	}

	parallel.ForEach(lo.Slice(webResults, 0, 3), func(result SearchResult, _ int) {
		urlContext, reference, err := summarizeWebPageContent(result.Url, input, webSearchAgent)
		if err != nil {
			log.Printf("Warning: %v\n", err)
			return
		}
		if urlContext != "" {
			searchContext += urlContext
			references = append(references, reference)
		}
	})

	return searchContext, references, nil
}

func summarizeWebPageContent(url string, input string, webSearchAgent LLM) (string, Reference, error) {
	article, err := Article(url)
	if err != nil {
		return "", Reference{}, fmt.Errorf("error fetching article: %v", err)
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
		return "", Reference{}, fmt.Errorf("failed to extract context: %v", err)
	}

	if startsWith(relevant, "Irrelevant") {
		return "", Reference{}, nil
	}

	urlContext := fmt.Sprintf("\n===============\n[%s](%s)\n%s", article.Title, url, relevant)
	reference := Reference{Title: article.Title, Url: url, Context: relevant, Full: article.TextContent}

	return urlContext, reference, nil
}
