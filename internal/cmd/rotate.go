package cmd

import (
	"fmt"
	"github.com/jsiebens/sa-key-rotator/pkg/sakeyrotator"
	"github.com/spf13/cobra"
)

func rotateCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "rotate",
		SilenceUsage: true,
	}

	var name string
	var serviceAccountEmail string
	var bucket string
	var expiryInDays int
	var renewalWindowInDays int
	var forceCreate bool
	var forceDelete bool

	command.Flags().StringVar(&name, "name", sakeyrotator.DefaultName, "")
	command.Flags().StringVar(&serviceAccountEmail, "service-account", "", "")
	command.Flags().StringVar(&bucket, "bucket", "", "")
	command.Flags().IntVar(&expiryInDays, "days", 90, "number of days a key is valid for")
	command.Flags().IntVar(&renewalWindowInDays, "window", 15, "span of days at the end of the key's validity period in which it should be renewed/rotated")
	command.Flags().BoolVar(&forceCreate, "force-create", false, "")
	command.Flags().BoolVar(&forceDelete, "force-delete", false, "")

	_ = command.MarkFlagRequired("service-account")
	_ = command.MarkFlagRequired("bucket")

	command.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		if expiryInDays < 2 {
			return fmt.Errorf("days cannot be smaller than 2")
		}
		if renewalWindowInDays < 1 {
			return fmt.Errorf("window cannot be smaller than 1")
		}
		if renewalWindowInDays >= expiryInDays {
			return fmt.Errorf("window should be smaller than days")
		}

		rotator, err := sakeyrotator.NewRotator(ctx)
		if err != nil {
			return fmt.Errorf("error creating the rotator: %w", err)
		}

		if err := rotator.Rotate(ctx, serviceAccountEmail, name, bucket, expiryInDays, renewalWindowInDays, forceCreate, forceDelete); err != nil {
			return fmt.Errorf("error rotating keys: %w", err)
		}

		return nil
	}

	return command
}
