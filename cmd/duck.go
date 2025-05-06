package cmd

import (
	"fmt"
	"strings"

	"roderik/duckduck"
	"github.com/spf13/cobra"
)

var (
	numResults int
	// DuckCmd queries DuckDuckGo for keyword search and prints formatted results.
	DuckCmd = &cobra.Command{
		Use:     "duck [flags] <search terms>",
		Short:   "Search DuckDuckGo for keyword results",
		Long: `Duck runs a keyword search on DuckDuckGo.com and prints
the top N results in a simple markdown-style snippet:

  ## RESULT 1
  url:     https://duckduckgo.com/
  title:   DuckDuckGo — Privacy, simplified.
  snippet: The search engine that doesn’t track you.

You can override how many results to return with --num (default 20).`,
		Example: `  # Search for “golang cobra”
  roderik duck cobra golang

  # Limit to 5 results
  roderik duck -m 5 privacy`,
		Args:    cobra.MinimumNArgs(1),
		RunE:    runDuck,
	}
)

func init() {
	RootCmd.AddCommand(DuckCmd)
	flags := DuckCmd.Flags()
	flags.IntVarP(&numResults, "num", "m", 20, "number of results to return")
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
		var code int
		if _, scanErr := fmt.Sscanf(err.Error(), "%*[^0-9]%d", &code); scanErr == nil && code >= 200 && code < 300 {
			// treat any 2xx status as success; proceed with empty results
			res = nil
		} else {
			return "", err
		}
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

