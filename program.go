package main

import (
	"fmt"
	"log"
	"os"

	"github.com/saahilclaypool/simplesearch/agent"
)

func main() {
	input := os.Args[1]
	webSearchAgent := agent.Make(`
		You are an assistant who will help the main agent to a query in various ways.
		The Original Request is the users original request.
		Only use this to determine how best to answer your specific task.
		Do not make up or add information except from the information provided.
		`, false)

	// 1. do you need any information
	searches := &Searches{}
	err := webSearchAgent.Json(fmt.Sprintf("What google search queries would you perform to answer this query? Provide up to 5 queries. Make the queries varied, targetting different parts of the users request if it's multi part. Target different domain specific keywords or sites (for example, 'stackoverflow.com how do I invert a linked list?'.\n----\nOriginal Request: %s", input), searches)
	if err != nil {
		panic(err)
	}

	ctx := ""
	for _, q := range searches.Queries {
		webResults, err := agent.Search(q)
		if err != nil {
			log.Printf("Warning: %v\n", err)
			continue
		}
		mdResults := agent.FormatSearchResultsAsMarkdown(webResults)
		clickableUrls := &Urls{}
		err = webSearchAgent.Json(fmt.Sprintf("Which urls in this result set are worth clicking on to answer the request below? You can only click on up to 3.\n----\nOriginal Request: %s\n", mdResults), clickableUrls)
		if err != nil {
			log.Printf("Warning: %v\n", err)
			continue
		}
		for _, url := range clickableUrls.Urls {
			content, err := agent.Article(url)
			if err != nil {
				log.Printf("Warning: %v\n", err)
				continue
			}
			relevant, err := webSearchAgent(fmt.Sprintf("What part of the content below is useful to answer the original request? Write out the relevant sections verbatim.\nOriginal Request: %s\nArticle: [%s]( %s)\n==========\n%s\n", input, content.Title, url, content.TextContent), "", false)
			ctx = fmt.Sprintf("%s\n===============\n[%s](%s)\n%s", ctx, content.Title, url, relevant)
		}
	}

	ag := agent.Make(fmt.Sprintf("Answer the users request using the following context. Cite your answers with the title & url of the cite you pulled information from:\n\n%s", ctx), true)
	resp, err := ag(input, "", false)
	if err != nil {
		panic(err)
	}
	fmt.Printf("================================\n\n")
	fmt.Printf("Response:\n%s", resp)
}

type Searches struct {
	Queries []string
}

type Urls struct {
	Urls []string
}
