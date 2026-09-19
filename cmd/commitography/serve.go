package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/server"
)

func newServeCommand(env environment) *cobra.Command {
	// The listen address and the allowed roots are operational values
	// (ADR-0026 clause 1): they decide how the server runs and never reach a
	// report.
	operational := config.DefaultOperational()
	var open bool

	cmd := &cobra.Command{
		Use:           "serve",
		Short:         "Start the local web dashboard",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			options := server.Options{
				ListenAddress: operational.ListenAddress,
				AllowedRoots:  operational.AllowedRoots,
				Open:          open,
			}
			app, serveOptions, logger, err := composeServe(env, options)
			if err != nil {
				return err
			}
			for _, root := range app.EmptyAllowedRoots() {
				logger.Warn("allowed root %q is empty; if it is a mount, check that its source path exists", root)
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			err = server.Serve(ctx, serveOptions)
			// The manager owns every analysis goroutine, and none outlives the
			// command (ADR-0044 clause 1).
			app.Jobs.CancelAll()
			app.Jobs.Wait()
			return err
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&operational.ListenAddress, "listen", operational.ListenAddress, "HTTP listen address")
	flags.BoolVar(&open, "open", false, "Open the dashboard in the default browser")
	flags.StringArrayVar(&operational.AllowedRoots, "allowed-root", nil, "Allowed repository root; repeatable")
	return cmd
}
