package commands

import (
	"fmt"
	"os"

	"cmm/internal/mod"
	"github.com/spf13/cobra"
)

var (
	pinVersion string
)

var pinCmd = &cobra.Command{
	Use:   "pin <slug>",
	Short: "Pin a mod to its current or a specified version",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		slug := args[0]

		mgr := mod.NewManager(nil, "cmm.toml", "cmm.lock")

		if err := mgr.Pin(slug, pinVersion); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if pinVersion != "" {
			fmt.Printf("Pinned mod '%s' to version %s\n", slug, pinVersion)
		} else {
			fmt.Printf("Pinned mod '%s'\n", slug)
		}
	},
}

var unpinCmd = &cobra.Command{
	Use:   "unpin <slug>",
	Short: "Unpin a mod to allow automated updates",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		slug := args[0]

		mgr := mod.NewManager(nil, "cmm.toml", "cmm.lock")

		alreadyUnpinned, err := mgr.Unpin(slug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if alreadyUnpinned {
			fmt.Printf("Already unpinned: %s\n", slug)
			return
		}

		fmt.Printf("Unpinned mod '%s'\n", slug)
	},
}

func init() {
	rootCmd.AddCommand(pinCmd)
	rootCmd.AddCommand(unpinCmd)
	pinCmd.Flags().StringVarP(&pinVersion, "version", "v", "", "Specific version to pin")
}
