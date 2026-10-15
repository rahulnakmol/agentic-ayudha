package cli

import (
	"context"
	"slices"

	"github.com/spf13/cobra"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func init() { register("completion", runCompletion) }

// runCompletion prints the shell completion script (WP1.1b). The command tree
// is the registry's (ground rule 4), and completeInputs teaches it each enum.
func runCompletion(_ context.Context, e *Env, in Values) error {
	root := newRoot(e)
	switch in.String("shell") {
	case "bash":
		return root.GenBashCompletionV2(e.Out, true)
	case "zsh":
		return root.GenZshCompletion(e.Out)
	case "fish":
		return root.GenFishCompletion(e.Out, true)
	default:
		return root.GenPowerShellCompletionWithDesc(e.Out)
	}
}

// completeInputs registers completions for a registry command's enum inputs:
// positional enums in order, and enum flags by name. Other inputs complete
// nothing (an issue number is not guessable offline), so the shell never
// suggests file names for them.
func completeInputs(cmd *cobra.Command, c verbs.Command) {
	var positional []verbs.Input
	for _, in := range c.Inputs {
		if in.Positional {
			positional = append(positional, in)
			continue
		}
		if in.Type == verbs.TypeEnum && !slices.Contains(globalFlags, in.Name) {
			values := in.Enum
			_ = cmd.RegisterFlagCompletionFunc(in.Name, func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
				return values, cobra.ShellCompDirectiveNoFileComp
			})
		}
	}
	cmd.ValidArgsFunction = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) < len(positional) {
			in := positional[len(args)]
			switch in.Type {
			case verbs.TypeEnum:
				return in.Enum, cobra.ShellCompDirectiveNoFileComp
			case verbs.TypePath:
				return nil, cobra.ShellCompDirectiveDefault
			}
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}
