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
	Short: "search for something on DuckDuckGo",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runDuck,
}

func init() {
	// Add the duck sub-command to the root command
	RootCmd.AddCommand(duckCmd)
	flags := duckCmd.Flags()
	flags.IntVarP(&numResults, "num", "n", 20, "number of results to return")
	flags.BoolVarP(&naturalQuestion, "natural-question", "q", false, "treat query as a natural question")
	flags.BoolVarP(&answerQuestion, "answer", "a", false, "answer the question from search results")
	flags.BoolVarP(&silent, "silent", "s", false, "suppress intermediate output")
}
func runDuck(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")
	terms := query
	if naturalQuestion {
		var err error
		terms, err = GenerateSearchTerms(cmd.Context(), query)
		if err != nil {
			return fmt.Errorf("generate search terms: %w", err)
		}
		if !silent {
			cmd.Println("User question:", query)
		}
	}
	if !silent {
		cmd.Println("Search terms:", terms)
	}
	result, err := searchDuck(terms, numResults)
	if err != nil {
		return fmt.Errorf("duck search failed: %w", err)
	}
	if answerQuestion {
		answer, err := AnswerQuestion(cmd.Context(), query, result)
		if err != nil {
			return fmt.Errorf("answer question failed: %w", err)
		}
		cmd.Println("Answer:", answer)
	} else {
		cmd.Println(result)
	}
	return nil
}
func searchDuck(query string, num int) (string, error) {
	ddg := client.NewDuckDuckGoSearchClient()
	res, err := ddg.SearchLimited(query, num)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for i, r := range res {
		sb.WriteString(fmt.Sprintf("## RESULT %d\n", i+1))
		sb.WriteString(fmt.Sprintf("url:     %s\n", r.FormattedUrl))
		sb.WriteString(fmt.Sprintf("title:   %s\n", r.Title))
		sb.WriteString(fmt.Sprintf("snippet: %s\n", r.Snippet))
	}
	return sb.String(), nil
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

func GenerateSearchTerms(ctx context.Context, question string) (string, error) {
	// Define the template
	tmpl := `Given the user's question: '{{.Question}}', generate a space-separated list of search terms to feed to a search engine.`
	promptTemplate, err := template.New("prompt").Parse(tmpl)
	if err != nil {
		return "", err
	}
	data := struct {
		Question string
	}{
		Question: question,
	}
	var prompt bytes.Buffer
	if err := promptTemplate.Execute(&prompt, data); err != nil {
		return "", err
	}
	answer, err := TldrPromptSend(ctx, prompt.String())
	if err != nil {
		return "", err
	}
	return answer, nil
}
