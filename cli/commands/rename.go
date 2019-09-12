// Package commands provides the set of CLI commands used to communicate with the AIS cluster.
// This specific file handles the CLI commands that rename entities in the cluster.
/*
 * Copyright (c) 2019, NVIDIA CORPORATION. All rights reserved.
 */
package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/NVIDIA/aistore/api"
	"github.com/NVIDIA/aistore/cmn"
	"github.com/urfave/cli"
)

var (
	renameCmdsFlags = map[string][]cli.Flag{
		subcmdRenameBucket: {},
		subcmdRenameObject: {},
	}

	renameCmds = []cli.Command{
		{
			Name:  commandRename,
			Usage: "renames entities in the cluster",
			Subcommands: []cli.Command{
				{
					Name:         subcmdRenameBucket,
					Usage:        "renames ais buckets",
					ArgsUsage:    bucketOldNewArgumentText,
					Flags:        renameCmdsFlags[subcmdRenameBucket],
					Action:       renameBucketHandler,
					BashComplete: oldAndNewBucketList([]cli.BashCompleteFunc{}, false /* separator */, cmn.AIS),
				},
				{
					Name:         subcmdRenameObject,
					Usage:        "renames an object of the ais bucket",
					ArgsUsage:    objectOldNewArgumentText,
					Flags:        renameCmdsFlags[subcmdRenameObject],
					Action:       renameObjectHandler,
					BashComplete: oldAndNewBucketList([]cli.BashCompleteFunc{}, true /* separator */, cmn.AIS),
				},
			},
		},
	}
)

func renameBucketHandler(c *cli.Context) (err error) {
	baseParams := cliAPIParams(ClusterURL)
	return renameBucket(c, baseParams)
}

func renameObjectHandler(c *cli.Context) (err error) {
	if c.NArg() != 2 {
		return incorrectUsageError(c, errors.New("invalid number of arguments"))
	}
	var (
		baseParams = cliAPIParams(ClusterURL)
		oldObjFull = c.Args().Get(0)
		newObj     = c.Args().Get(1)
		bucket     string
		oldObj     string
	)

	bucket, oldObj = splitBucketObject(oldObjFull)
	if bucket == "" {
		bucket, _ = os.LookupEnv(aisBucketEnvVar)
	}
	if oldObj == "" {
		return incorrectUsageError(c, fmt.Errorf("no object specified in '%s'", oldObjFull))
	}
	if bucket == "" {
		return incorrectUsageError(c, fmt.Errorf("no bucket specified for object '%s'", oldObj))
	}

	if err = api.RenameObject(baseParams, bucket, oldObj, newObj); err != nil {
		return
	}

	fmt.Fprintf(c.App.Writer, "%s renamed to %s\n", oldObj, newObj)
	return
}