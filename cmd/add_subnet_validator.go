// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/subnet-cli/client"
	"github.com/ava-labs/subnet-cli/pkg/color"
	"github.com/manifoldco/promptui"
	"github.com/onsi/ginkgo/v2/formatter"
	"github.com/spf13/cobra"
)

func newAddSubnetValidatorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subnet-validator",
		Short: "Adds a subnet to the validator",
		Long: `
Adds a subnet to the validator.

$ subnet-cli add subnet-validator \
--private-key-path=.insecure.ewoq.key \
--public-uri=http://localhost:52250 \
--subnet-id="24tZhrm8j8GCJRE9PomW8FaeqbgGS4UAQjJnqqn8pq5NwYSYV1" \
--node-ids="NodeID-4B4rc5vdD1758JSBYL1xyvE5NHGzz6xzH" \
--validate-weight=1000

`,
		RunE: createSubnetValidatorFunc,
	}

	cmd.PersistentFlags().StringVar(&subnetIDs, "subnet-id", "", "subnet ID (must be formatted in ids.ID)")
	cmd.PersistentFlags().StringSliceVar(&nodeIDs, "node-ids", nil, "a list of node IDs (must be formatted in ids.ID)")

	start := time.Now().Add(time.Minute)
	end := start.Add(50 * 24 * time.Hour)
	cmd.PersistentFlags().StringVar(&subnetValidateStarts, "validate-start", start.Format(time.RFC3339), "validate start timestamp in RFC3339 format")
	cmd.PersistentFlags().StringVar(&subnetValidateEnds, "validate-end", end.Format(time.RFC3339), "validate start timestamp in RFC3339 format")
	cmd.PersistentFlags().Uint64Var(&subnetValidateWeight, "validate-weight", 1000, "validate weight")

	return cmd
}

var errZeroValidateWeight = errors.New("zero validate weight")

func createSubnetValidatorFunc(cmd *cobra.Command, args []string) error {
	cli, info, err := InitClient(publicURI, true)
	if err != nil {
		return err
	}
	return addSubnetValidator(cli, info)
}

func addSubnetValidator(cli client.Client, info *Info) (err error) {
	info.txFee = uint64(info.feeData.TxFee)
	info.subnetID, err = ids.FromString(subnetIDs)
	if err != nil {
		return err
	}
	info.nodeIDs = make([]ids.ShortID, 0)
	for _, id := range nodeIDs {
		nodeID, err := ids.ShortFromPrefixedString(id, constants.NodeIDPrefix)
		if err != nil {
			return err
		}
		info.nodeIDs = append(info.nodeIDs, nodeID)
	}

	info.subnetValidateWeight = subnetValidateWeight
	info.validateRewardFeePercent = 0
	if info.subnetValidateWeight == 0 {
		return errZeroValidateWeight
	}

	info.validateStart, err = time.Parse(time.RFC3339, subnetValidateStarts)
	if err != nil {
		return err
	}
	info.validateEnd, err = time.Parse(time.RFC3339, subnetValidateEnds)
	if err != nil {
		return err
	}

	info.rewardAddr = ids.ShortEmpty
	info.changeAddr = ids.ShortEmpty

	if err := info.CheckBalance(); err != nil {
		return err
	}
	msg := CreateAddTable(info)
	if enablePrompt {
		msg = formatter.F("\n{{blue}}{{bold}}Ready to add subnet validator, should we continue?{{/}}\n") + msg
	}
	fmt.Fprint(formatter.ColorableStdOut, msg)

	if enablePrompt {
		prompt := promptui.Select{
			Label:  "\n",
			Stdout: os.Stdout,
			Items: []string{
				formatter.F("{{red}}No, stop it!{{/}}"),
				formatter.F("{{green}}Yes, let's create! {{bold}}{{underline}}I agree to pay the fee{{/}}{{green}}!{{/}}"),
			},
		}
		idx, _, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx == 0 {
			return nil
		}
	}

	println()
	println()
	println()
	for _, nodeID := range info.nodeIDs {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		took, err := cli.P().AddSubnetValidator(
			ctx,
			info.key,
			info.subnetID,
			nodeID,
			info.validateStart,
			info.validateEnd,
			info.subnetValidateWeight,
		)
		cancel()
		if err != nil {
			return err
		}
		color.Outf("{{magenta}}added %s to subnet %s validator set{{/}} {{light-gray}}(took %v){{/}}\n\n", nodeID, info.subnetID, took)
	}

	info.txFee = 0
	info.balance, err = cli.P().Balance(info.key)
	if err != nil {
		return err
	}
	fmt.Fprint(formatter.ColorableStdOut, CreateAddTable(info))
	return nil
}