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
	"github.com/dansteen/controlled-compose/control"
	"github.com/dansteen/controlled-compose/handler"
	"github.com/dansteen/controlled-compose/types"
	"golang.org/x/net/context"
	"log"

	"fmt"
	"github.com/docker/libcompose/docker/client"
	"github.com/docker/libcompose/project/options"
	"os"

	"github.com/spf13/cobra"
)

// upCmd represents the up command
var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Bring up a compose project",
	Long:  `Bring up a compose project`,
	Run:   up,
}

func init() {

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// upCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	//upCmd.Flags().BoolP("file", "f", false, "Help message for toggle")
	RootCmd.AddCommand(upCmd)
	upCmd.Flags().StringSliceVarP(&files, "file", "f", nil, "-f <PathToComposeFile>")
	upCmd.Flags().StringSliceVarP(&appVersions, "app_version", "a", nil, "The version of a particular container to use.  This will overrid e what is set in the compose files for a particular image or build stanza. Format: container:version")

}

func up(cmd *cobra.Command, args []string) {

	// a file list is required
	if len(files) == 0 {
		cmd.Usage()
		log.Fatal("Please provide a list of compose files to use")
	}
	// a project name is required
	if len(projectName) == 0 {
		cmd.Usage()
		log.Fatal("Please provide a project name")
	}

	project, err := control.GenProject(projectName, files, appVersions)
	orderedServices := project.SortedServices()
	fmt.Printf("Services will be started in the following order: %v\n", orderedServices)

	// create a connection to the docker server
	dockerClient, err := client.Create(client.Options{})
	if err != nil {
		log.Fatal(err)
	}

	// run through and start up our services
	for _, service_name := range orderedServices {
		// the return status of our monitors
		event_response := make(chan types.ContainerStatus)
		// also an indicator that we no longer need to wait for return status
		done := make(chan struct{})

		fmt.Printf("Starting  up service - %v\n", service_name)
		err = project.ComposeProject.Up(context.Background(), options.Up{options.Create{ForceRecreate: true}}, service_name)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Started up service - %v\n", service_name)

		// get the container name for this service.
		containers, err := project.Containers(service_name)
		if err != nil {
			log.Fatal(err)
		}
		// We only spin up one for each services so we can just grab the first one
		container_name := containers[0].Name()

		// depending on which monitors this service uses we do different things
		// first see if there area ny state conditions at all
		if conditions, found := project.StateConditions[service_name]; found {
			fmt.Printf("Waiting for conditions: %+v\n", conditions)
			// check if we monitor the exit code
			if conditions.ExitCodes != nil {
				// add in a listener so we can get updates on what is happening
				//container_events := make(chan events.Event, 2)
				//p.AddListener(container_events)
				d_events, err := project.ComposeProject.Events(context.Background(), service_name)
				if err != nil {
					log.Fatal(err)
				}
				// listen for events
				go handler.Exit(dockerClient, d_events, event_response, conditions.ExitCodes, done)
			}

			// check if we have configured a timeout
			if conditions.Timeout != nil {
				go handler.Timeout(conditions.Timeout, event_response, done)
			}

			// check if we have  log monitors
			if conditions.FileMonitors != nil {
				// run a handler for each file
				for filename, monitors := range conditions.FileMonitors {
					// depending on what time of file/output we are monitoring we do things a bit differently
					if filename == "STDOUT" {
						go handler.Output(dockerClient, container_name, true, false, monitors, event_response, done)
					} else if filename == "STDERR" {
						go handler.Output(dockerClient, container_name, false, true, monitors, event_response, done)
					} else {
						go handler.FileMonitor(filename, monitors, event_response, done)
					}
				}
			}

			// wait until we have been given the go-ahead to move on to the next service if we need to
			fmt.Println("waiting for response")
			response := <-event_response
			fmt.Println(response)
			// we have to be sure to close this as some functions may still be running
			close(done)
			close(event_response)
			// we only continue if we returned success
			if response.Status != "success" {
				fmt.Printf("Failed! - Container %v exited with an error: %v", container_name, response.Message)
				os.Exit(1)
			}
		}
	}

	if err != nil {
		log.Fatal(err)
	}
}
