package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "streamtxd",
	Short: "Stream tx is stream tx mainnet to forknet",
}

func init() {
	rootCmd.AddCommand(startGroth16Cmd)
	rootCmd.AddCommand(startGroth16FibCmd)
	rootCmd.AddCommand(startPlonkyCmd)
	startGroth16Cmd.Flags().StringVar(&groth16Mode, "mode", "demo", "Groth16 mode: demo, produce, verify")
}

var startGroth16Cmd = &cobra.Command{
	Use:     "start-groth16",
	Short:   "Start groth16",
	Example: "go run main.go start-groth16 --mode produce; go run main.go start-groth16 --mode verify",
	Run: func(cmd *cobra.Command, args []string) {
		switch groth16Mode {
		case "produce":
			runGroth16Produce()
		case "verify":
			runGroth16Verify()
		default:
			runGroth16()
		}
	},
}

var startGroth16FibCmd = &cobra.Command{
	Use:   "start-groth16-fib",
	Short: "Start groth16 with fibonatri circuit",
	Run: func(cmd *cobra.Command, args []string) {
		runGroth16_fib()
	},
}

var startPlonkyCmd = &cobra.Command{
	Use:   "start-plonk",
	Short: "Start plonk",
	Run: func(cmd *cobra.Command, args []string) {
		runPlonk()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
