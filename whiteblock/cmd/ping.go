package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/whiteblock/cli/whiteblock/util"
	"golang.org/x/sys/unix"
	"log"
	"os"
)

var pingCmd = &cobra.Command{
	Use:   "ping <sending node> <receiving node>",
	Short: "Ping will send packets to a node.",
	Long: `
Ping will send packets to a node and will output information

Params: sending node, receiving node
	`,

	Run: func(cmd *cobra.Command, args []string) {

		util.CheckArguments(cmd, args, 2, 2)
		nodes := GetNodes()

		sendingNodeNumber := util.CheckAndConvertInt(args[0], "sending node number")
		receivingNodeNumber := util.CheckAndConvertInt(args[1], "receiving node number")

		util.CheckIntegerBounds(cmd, "sending node number", sendingNodeNumber, 0, len(nodes)-1)
		util.CheckIntegerBounds(cmd, "receiving node number", receivingNodeNumber, 0, len(nodes)-1)

		log.Fatal(unix.Exec(conf.SSHBinary, []string{
			"ssh", "-i", conf.SSHPrivateKey, "-o", "StrictHostKeyChecking no",
			"-o", "UserKnownHostsFile=/dev/null", "-o", "PasswordAuthentication no", "-o", "ConnectTimeout=10", "-y",
			"root@" + fmt.Sprintf(nodes[sendingNodeNumber].IP), "ping",
			fmt.Sprintf(nodes[receivingNodeNumber].IP)}, os.Environ()))
	},
}

func init() {
	RootCmd.AddCommand(pingCmd)
}
