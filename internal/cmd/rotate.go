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

	command.Flags().StringVar(&name, "name", sakeyrotator.DefaultName, "")
	command.Flags().StringVar(&serviceAccountEmail, "service-account", "", "")
	command.Flags().StringVar(&bucket, "bucket", "", "")
	command.Flags().IntVar(&expiryInDays, "days", 90, "number of days a key is valid for")
	command.Flags().IntVar(&renewalWindowInDays, "window", 15, "span of days at the end of the key's validity period in which it should be renewed/rotated")

	_ = command.MarkFlagRequired("service-account")
	_ = command.MarkFlagRequired("bucket")

	command.Run = func(cmd *coral.Command, args []string) {
		ctx := cmd.Context()

		logger := sakeyrotator.NewLogger(logLevel, stdout, stderr)
		rotator, err := sakeyrotator.NewRotator(ctx, logger)
		if err != nil {
			logger.Fatal("error creating the rotator", "service_account", serviceAccountEmail, "err", err)
		}

		if err := rotator.Rotate(ctx, serviceAccountEmail, name, bucket, expiryInDays, renewalWindowInDays); err != nil {
			logger.Fatal("error rotating keys", "service_account", serviceAccountEmail, "err", err)
		}
	}

	return command
}
