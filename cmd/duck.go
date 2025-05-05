package cmd

import (
	"fmt"
	"strings"

	"github.com/sap-nocops/duckduckgogo/client"
	"github.com/spf13/cobra"
)

var numResults int
var duckCmd = &cobra.Command{
	Use:   "duck [flags] <search terms>",
	Short: "search for something on DuckDuckGo",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runDuck,
}

func init() {
	RootCmd.AddCommand(duckCmd)
	flags := duckCmd.Flags()
	flags.IntVarP(&numResults, "num", "n", 20, "number of results to return")
}
func runDuck(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")
	result, err := searchDuck(query, numResults)
	if err != nil {
		return fmt.Errorf("duck search failed: %w", err)
	}
	cmd.Println(result)
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

func AnswerQuestion(ctx context.Context, question, contextStr string) (string, error) {
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
	promptTemplate, err := template.New("prompt").Parse(tmpl)
	if err != nil {
		return "", err
	}
	data := struct {
		Question string
		Context  string
	}{
		Question: question,
		Context:  contextStr,
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
