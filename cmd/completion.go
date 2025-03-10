/*
Copyright © 2025 Will Atlas <will@atls.dev>
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// completionCmd represents the completion command
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `To load completions:

Bash:
  $ source <(qq completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ qq completion bash > /etc/bash_completion.d/qq
  # macOS:
  $ qq completion bash > $(brew --prefix)/etc/bash_completion.d/qq

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ qq completion zsh > "${fpath[1]}/_qq"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ qq completion fish | source

  # To load completions for each session, execute once:
  $ qq completion fish > ~/.config/fish/completions/qq.fish

PowerShell:
  PS> qq completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> qq completion powershell > qq.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}