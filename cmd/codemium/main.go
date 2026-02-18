package main

import (
	"fmt"
	"os"

	"github.com/labtiva/codemium/internal/auth"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "codemium",
		Short: "Generate code statistics across repositories",
	}

	root.AddCommand(newAuthCmd())
	root.AddCommand(newAnalyzeCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a provider",
		RunE:  runAuthLogin,
	}
	loginCmd.Flags().String("provider", "", "Provider to authenticate with (bitbucket, github)")
	loginCmd.MarkFlagRequired("provider")

	cmd.AddCommand(loginCmd)
	return cmd
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	providerName, _ := cmd.Flags().GetString("provider")

	store := auth.NewFileStore(auth.DefaultStorePath())
	ctx := cmd.Context()

	var cred auth.Credentials
	var err error

	switch providerName {
	case "bitbucket":
		clientID := os.Getenv("CODEMIUM_BITBUCKET_CLIENT_ID")
		clientSecret := os.Getenv("CODEMIUM_BITBUCKET_CLIENT_SECRET")
		if clientID == "" || clientSecret == "" {
			return fmt.Errorf("set CODEMIUM_BITBUCKET_CLIENT_ID and CODEMIUM_BITBUCKET_CLIENT_SECRET environment variables")
		}
		bb := &auth.BitbucketOAuth{ClientID: clientID, ClientSecret: clientSecret}
		fmt.Fprintln(os.Stderr, "Opening browser for Bitbucket authorization...")
		cred, err = bb.Login(ctx)

	case "github":
		clientID := os.Getenv("CODEMIUM_GITHUB_CLIENT_ID")
		if clientID == "" {
			return fmt.Errorf("set CODEMIUM_GITHUB_CLIENT_ID environment variable")
		}
		gh := &auth.GitHubOAuth{ClientID: clientID, OpenBrowser: true}
		cred, err = gh.Login(ctx)

	default:
		return fmt.Errorf("unsupported provider: %s (use bitbucket or github)", providerName)
	}

	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if err := store.Save(providerName, cred); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Successfully authenticated with %s!\n", providerName)
	return nil
}

func newAnalyzeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "analyze",
		Short: "Analyze repositories and generate code statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented")
		},
	}
}
