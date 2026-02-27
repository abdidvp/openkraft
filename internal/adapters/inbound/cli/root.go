package cli

import "github.com/spf13/cobra"

var (
	version = "dev"
	commit  = "none"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "openkraft",
		Short: "Stop shipping 80% code",
		Long:  "OpenKraft scores your codebase's AI-readiness and enforces that every module meets the quality of your best module.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newScoreCmd())
	cmd.AddCommand(newCheckCmd())
	cmd.AddCommand(newMCPCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newOnboardCmd())
	cmd.AddCommand(newFixCmd())
	cmd.AddCommand(newValidateCmd())
	return cmd
}

// NewRootCmdForTest returns the root command for testing.
func NewRootCmdForTest() *cobra.Command {
	return newRootCmd()
}

func Execute() error {
	return newRootCmd().Execute()
}
