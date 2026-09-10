package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/sinanganiz/commitography/internal/server"
)

func newServeCommand() *cobra.Command {
	var options server.Options

	cmd := &cobra.Command{
		Use:           "serve",
		Short:         "Start the local web dashboard",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return server.Serve(context.Background(), options)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&options.ListenAddress, "listen", "127.0.0.1:8080", "HTTP listen address")
	flags.BoolVar(&options.Open, "open", false, "Open the dashboard in the default browser")
	flags.StringArrayVar(&options.AllowedRoots, "allowed-root", nil, "Allowed repository root; repeatable")
	return cmd
}
