import (
	"net/http"
	"flag"
	"github.com/spf13/cobra"
)

var port int

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a simple static file server",
	Run: func(cmd *cobra.Command, args []string) {
		fs := http.FileServer(http.Dir("assets/"))
		http.Handle("/", fs)

		addr := fmt.Sprintf(":%d", port)
		fmt.Printf("Starting server on %s\n", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	flag.IntVar(&port, "port", 80, "Port to run the server on")
	rootCmd.AddCommand(serverCmd)
}
