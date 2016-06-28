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

	"github.com/docker/libcompose/project/options"

	"github.com/dansteen/controlled-compose/control"
	"golang.org/x/net/context"

	"github.com/spf13/cobra"
)

// buildCmd represents the rm command
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build containers",
	Long:  `Build containres`,
	Run:   build,
}

func init() {
	RootCmd.AddCommand(buildCmd)
	buildCmd.Flags().StringSliceVarP(&files, "file", "f", nil, "-f <PathToComposeFile>")
	buildCmd.Flags().StringSliceVarP(&appVersions, "app_version", "a", nil, "The version of a particular container to use.  This will overrid e what is set in the compose files for a particular image or build stanza. Format: container:version")

}

func build(cmd *cobra.Command, args []string) {
	// a project name is required
	if len(projectName) == 0 {
		cmd.Usage()
		log.Fatal("Please provide a project name")
	}

	// a file list is required
	if len(files) == 0 {
		cmd.Usage()
		log.Fatal("Please provide a list of compose files to use")
	}

	// generate our project
	project, err := control.GenProject(projectName, files, appVersions)
	if err != nil {
		log.Fatal(err)
	}

	for _, serviceName := range project.SortedServices() {
		// if our service has a build component, we build it. Otherwise we skip it.
		if project.Services[serviceName].Config().Build.Context == "" {
			fmt.Printf("%v does not have a build section. Skipping.\n", serviceName)
			continue
		}

		err := project.Services[serviceName].Build(context.Background(), options.Build{
			NoCache:     false,
			ForceRemove: true,
			Pull:        false,
		})

		if err != nil {
			log.Fatal(err)
		}
	}

}
