package cmd

import (
	"github.com/jsiebens/sa-key-rotator/pkg/sakeyrotator"
	"github.com/muesli/coral"
)

func checkCommand() *coral.Command {
	command := &coral.Command{
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

	command.Run = func(cmd *coral.Command, args []string) {
		ctx := cmd.Context()

		logger := sakeyrotator.NewLogger(logLevel, stdout, stderr)

		if expiryInDays < 2 {
			logger.Fatal("days cannot be smaller than 2")
		}
		if renewalWindowInDays < 1 {
			logger.Fatal("window cannot be smaller than 1")
		}
		if renewalWindowInDays >= expiryInDays {
			logger.Fatal("window should be smaller than days")
		}

		rotator, err := sakeyrotator.NewRotator(ctx, logger)
		if err != nil {
			logger.Fatal("error creating the rotator", "service_account", serviceAccountEmail, "err", err)
		}

		if err := rotator.Rotate(ctx, serviceAccountEmail, name, bucket, expiryInDays, renewalWindowInDays, forceCreate, forceDelete); err != nil {
			logger.Fatal("error rotating keys", "service_account", serviceAccountEmail, "err", err)
		}
	}

	return command
}
