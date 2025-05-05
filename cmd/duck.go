package cmd

import (
	"fmt"
	"strings"

	"github.com/sap-nocops/duckduckgogo/client"
	"github.com/spf13/cobra"
)

var (
	numResults int
	// DuckCmd queries DuckDuckGo for keyword search and prints formatted results.
	DuckCmd = &cobra.Command{
		Use:   "duck [flags] <search terms>",
		Short: "keyword search on DuckDuckGo",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runDuck,
	}
)

func init() {
	RootCmd.AddCommand(DuckCmd)
	flags := DuckCmd.Flags()
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

