// Copyright © 2016 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/docker/engine-api/types"
	"github.com/docker/libcompose/docker/client"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

// some variables to store our flags
var (
	force bool
)

// rmCmd represents the rm command
var rmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Delete containers associated with a project",
	Long:  `Delete containers associated with a project`,
	Run:   rm,
}

func init() {
	RootCmd.AddCommand(rmCmd)
	rmCmd.Flags().BoolVarP(&force, "force", "f", false, "Force removal of running containers")
}

func rm(cmd *cobra.Command, args []string) {
	// a project name is required
	if len(projectName) == 0 {
		cmd.Usage()
		log.Fatal("Please provide a project name")
	}

	// grab a list of containers that are running
	dockerClient, err := client.Create(client.Options{})
	if err != nil {
		log.Fatal(err)
	}

	allContainers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		log.Fatal(err)
	}

	// run through and pick out our containers
	var ourContainers []types.Container
	var runningContainers []types.Container
	for _, container := range allContainers {
		if container.Labels["com.docker.compose.project"] == projectName {
			ourContainers = append(ourContainers, container)
			if container.State == "running" {
				runningContainers = append(runningContainers, container)
			}
		}
	}

	// first check if we have specified force or if there are no running containers
	if len(runningContainers) != 0 && force == false {
		fmt.Println("The following containers are still running. Specify -f to force stop them. No action taken")
		for _, container := range runningContainers {
			fmt.Printf("%v\n", container.Names)
			os.Exit(1)
		}
	} else {
		for _, container := range ourContainers {
			fmt.Printf("Removing %v:  ", container.Names)
			err := dockerClient.ContainerRemove(context.Background(), container.ID, types.ContainerRemoveOptions{
				RemoveVolumes: true,
				RemoveLinks:   false,
				Force:         true,
			})
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%v\n", "done")
		}
	}

}
