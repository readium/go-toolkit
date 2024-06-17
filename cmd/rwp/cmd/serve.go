package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"log/slog"

	"github.com/readium/go-toolkit/cmd/rwp/cmd/serve"
	"github.com/readium/go-toolkit/pkg/streamer"
	"github.com/spf13/cobra"
)

var debugFlag bool

var bindAddressFlag string

var bindPortFlag uint16

var serveCmd = &cobra.Command{
	Use:   "serve <directory>",
	Short: "Start a local HTTP server, serving a specified directory of publications",
	Long: `Start a local HTTP server, serving a specified directory of publications.

This command will start an HTTP serve listening by default on 'localhost:15080',
serving all compatible files (EPUB, PDF, CBZ, etc.) found in the directory
as Readium Web Publications. To get started, the manifest can be accessed from
'http://localhost:15080/<filename in base64url encoding without padding>/manifest.json'.
This file serves as the entry point and contains metadata and links to the rest
of the files that can be accessed for the publication.

For debugging purposes, the server also exposes a '/list.json' endpoint that
returns a list of all the publications found in the directory along with their
encoded paths. This will be replaced by an OPDS 2 feed in a future release.

Note: This server is not meant for production usage, and should not be exposed
to the internet except for testing/debugging purposes.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("expects a directory path to serve publications from")
		} else if len(args) > 1 {
			return errors.New("accepts a directory path")
		}
		return nil
	},

	SuggestFor: []string{"server"},
	RunE: func(cmd *cobra.Command, args []string) error {
		// By the time we reach this point, we know that the arguments were
		// properly parsed, and we don't want to show the usage if an API error
		// occurs.
		cmd.SilenceUsage = true

		path := filepath.Clean(args[0])
		fi, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("given directory %s does not exist", path)
			}
			return fmt.Errorf("failed to stat %s: %w", path, err)
		}
		if !fi.IsDir() {
			return fmt.Errorf("given path %s is not a directory", path)
		}

		// Log level
		if debugFlag {
			slog.SetLogLoggerLevel(slog.LevelDebug)
		} else {
			slog.SetLogLoggerLevel(slog.LevelInfo)
		}

		pubServer := serve.NewServer(serve.ServerConfig{
			Debug:             debugFlag,
			BaseDirectory:     path,
			JSONIndent:        indentFlag,
			InferA11yMetadata: streamer.InferA11yMetadata(inferA11yFlag),
		})

		bind := fmt.Sprintf("%s:%d", bindAddressFlag, bindPortFlag)
		httpServer := &http.Server{
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
			Addr:           bind,
			Handler:        pubServer.Routes(),
		}
		slog.Info("Starting HTTP server", "address", "http://"+httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("Server stopped", "error", err)
		} else {
			slog.Info("Goodbye!")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVarP(&bindAddressFlag, "address", "a", "localhost", "Address to bind the HTTP server to")
	serveCmd.Flags().Uint16VarP(&bindPortFlag, "port", "p", 15080, "Port to bind the HTTP server to")
	serveCmd.Flags().StringVarP(&indentFlag, "indent", "i", "", "Indentation used to pretty-print JSON files")
	serveCmd.Flags().Var(&inferA11yFlag, "infer-a11y", "Infer accessibility metadata: no, merged, split")
	serveCmd.Flags().BoolVarP(&debugFlag, "debug", "d", false, "Enable debug mode")

}
