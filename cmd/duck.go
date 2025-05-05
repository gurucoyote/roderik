package cmd

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/sap-nocops/duckduckgogo/client"
	"github.com/spf13/cobra"
	"strings"
)

var (
	numResults      int
	naturalQuestion bool
	answerQuestion  bool
	silent          bool
)
var duckCmd = &cobra.Command{
	Use:   "duck [flags] <search terms>",
	Run:   duckCmdHandler,
	Short: "search for something on duckduckgo",
}

func init() {
	// Add the duck sub-command to the root command
	rootCmd.AddCommand(duckCmd)
	duckCmd.PersistentFlags().IntVarP(&numResults, "num", "n", 20, "return n number of results")
	duckCmd.PersistentFlags().BoolVarP(&naturalQuestion, "natural-question", "q", false, "indicates that the query string is a normal question")
	duckCmd.PersistentFlags().BoolVarP(&answerQuestion, "answer", "a", false, "answer the original user question from the context of the search result")
	duckCmd.PersistentFlags().BoolVarP(&silent, "silent", "s", false, "suppress all output except the final result")
}
func duckCmdHandler(cmd *cobra.Command, args []string) {
	query := strings.Join(args, " ")
	searchTerms := query
	if naturalQuestion {
		// If the naturalQuestion flag is set, generate search terms from the user's question
		searchTerms = GenerateSearchTerms(query)
		if !silent {
			cmd.Println("User Question: ", query)
		}
	}
	if !silent {
		cmd.Println("Search Terms: ", searchTerms)
	}
	searchResult := Duck(query, numResults)
	if answerQuestion {
		answer := AnswerQuestion(query, searchResult)
		cmd.Println("Answer: ", answer)
	} else {
		cmd.Println(searchResult)
	}
}
func Duck(query string, num int) string {

	ddg := client.NewDuckDuckGoSearchClient()
	res, err := ddg.SearchLimited(query, num)
	if err != nil {
		fmt.Printf("error: %v", err)
		return ""
	}
	result := ""
	for i, r := range res {
		result += fmt.Sprintf("## RESULT %d\n", i+1)
		result += fmt.Sprintf("url:     %s\n", r.FormattedUrl)
		result += fmt.Sprintf("title:   %s\n", r.Title)
		result += fmt.Sprintf("snippet: %s\n", r.Snippet)
	}
	return result
}

func AnswerQuestion(question string, context string) string {
	// Define the template
	tmpl := `Given the user's question: '{{.Question}}'

 and these search results:

 {{.Context}}

First generate and output an answer that satisfies the user's question. 
Then also supply the 3 most relevant URLs, prioritizing factual, authoritative sources over travel, shopping, or other SEO-optimized sites. 
Use the following format for the URLs:
1. <normalized URL including https schema>| <a short title for the URL>
2. ...
3. ...

 `

	// Create a new template and parse the prompt into it
	promptTemplate, err := template.New("prompt").Parse(tmpl)
	if err != nil {
		panic(err)
	}

	// Create a new PromptData struct to hold the data for the template
	data := struct {
		Question string
		Context  string
	}{
		Question: question,
		Context:  context,
	}

	// Create a bytes.Buffer to hold the generated prompt
	var prompt bytes.Buffer

	// Execute the template, inserting the data and writing the output to the prompt buffer
	err = promptTemplate.Execute(&prompt, data)
	if err != nil {
		panic(err)
	}

	// Call the TldrPromptSend function with the created prompt
	answer, _ := TldrPromptSend(prompt.String())

	// Return the answer
	return answer
}

func GenerateSearchTerms(question string) string {
	// Define the template
	tmpl := `Given the user's question: '{{.Question}}', generate a space-separated list of search terms to feed to a search engine.`

	// Create a new template and parse the prompt into it
	promptTemplate, err := template.New("prompt").Parse(tmpl)
	if err != nil {
		panic(err)
	}

	// Create a new PromptData struct to hold the data for the template
	data := struct {
		Question string
	}{
		Question: question,
	}

	// Create a bytes.Buffer to hold the generated prompt
	var prompt bytes.Buffer

	// Execute the template, inserting the data and writing the output to the prompt buffer
	err = promptTemplate.Execute(&prompt, data)
	if err != nil {
		panic(err)
	}

	// Call the TldrPromptSend function with the created prompt
	answer, _ := TldrPromptSend(prompt.String())

	// Return the list of search terms
	return answer
}
