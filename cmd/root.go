package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/sethpollack/ecr-cleaner/ecr"
	"github.com/spf13/cobra"
)

var (
	opts ecr.Opts

	runExample = `
		ecr-cleaner \
			-r "(^[0-9a-f]+$|-[0-9a-f]+$)" \
			-n foobar
	`
	rootCmd = &cobra.Command{
		Use:   "ecr-cleaner",
		Short: "Clean untagged images from ECR",
		Long:  runExample,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !opts.RunAll && opts.RepoName == "" {
				return errors.New("You must specify either --repo-name or --all.")
			} else {
				return nil
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return ecr.CleanRepos(opts)
		},
	}
)

func init() {
	rootCmd.Flags().StringVarP(&opts.TagRegex, "tag-regex", "r", "(^[0-9a-f]+$|-[0-9a-f]+$)", "Image tag regex")
	rootCmd.Flags().StringVarP(&opts.RepoName, "repo-name", "n", "", "Name of repo to clean.")
	rootCmd.Flags().BoolVarP(&opts.RunAll, "all", "a", false, "Clean all repos.")
	rootCmd.Flags().BoolVarP(&opts.DryRun, "dry-run", "d", true, "dry run to see tags that will be deleted.")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
