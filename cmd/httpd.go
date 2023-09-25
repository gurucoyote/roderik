package cmd

import (
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/spf13/cobra"
	"net/http"
	"strings"
)

var (
	port      int
	basicAuth bool
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a simple static file server",
	Run: func(cmd *cobra.Command, args []string) {
		fs := http.FileServer(http.Dir("assets/"))
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if basicAuth {
				auth := r.Header.Get("Authorization")
				if auth == "" {
					w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
					http.Error(w, "Unauthorized.", http.StatusUnauthorized)
					return
				}

				payload, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
				pair := strings.SplitN(string(payload), ":", 2)

				if len(pair) != 2 || !(pair[0] == "user" && pair[1] == "pass") {
					http.Error(w, "Unauthorized.", http.StatusUnauthorized)
					return
				}
			}

			fs.ServeHTTP(w, r)
		})

		http.Handle("/", handler)

		addr := fmt.Sprintf(":%d", port)
		fmt.Printf("Starting server on %s\n", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			panic(err)
		}
	},
}

func init() {
	serverCmd.Flags().IntVarP(&port, "port", "p", 80, "Port to run the server on")
	serverCmd.Flags().BoolVarP(&basicAuth, "basic-auth", "a", false, "Require basic auth")
	RootCmd.AddCommand(serverCmd)
}
