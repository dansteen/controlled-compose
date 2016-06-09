// Handler provides various state hanlders for our controlled compose run
package handler

import (
	"fmt"
	"github.com/dansteen/controlled-compose/types"
	"github.com/docker/engine-api/client"
	"github.com/docker/libcompose/project/events"
	"golang.org/x/net/context"
	"log"
)

// Exit will handle the case where a container exits for whatever reason.
func Exit(client client.APIClient, container_events <-chan events.ContainerEvent, container_status chan<- types.ContainerStatus, exit_codes *types.ExitCodes) {
	for event := range container_events {
		fmt.Printf("%+v\n", event)
		// if the container has died
		if event.Event == "die" {
			// grab some information about the container that died
			info, err := client.ContainerInspect(context.Background(), event.ID)
			container_exit_code := info.ContainerJSONBase.State.ExitCode
			if err != nil {
				log.Fatal(err)
			}

			// check our conditions
			var status types.ContainerStatus
			// first check if this should not have died at all
			if exit_codes.Contains(-1) {
				status = types.ContainerStatus{
					Status:  "failure",
					Message: "Container exited but was expected to persist.",
				}
				// then check if our exit code is not listed in the successes
			} else if !exit_codes.Contains(container_exit_code) {
				status = types.ContainerStatus{
					Status:  "failure",
					Message: fmt.Sprintf("Container exited error code %v", container_exit_code),
				}
				// else it is a success yay!
			} else {
				// if it matches what we expected, we exit with success
				status = types.ContainerStatus{
					Status:  "success",
					Message: fmt.Sprintf("Container exited succesfully with exit code %v", container_exit_code),
				}

			}
			// report back our exit
			container_status <- status
		}
	}
}
