package main

import (
	"fmt"
	"github.com/dansteen/controlled-compose/handler"
	"github.com/dansteen/controlled-compose/types"
	"golang.org/x/net/context"
	"log"
	"os"

	"github.com/docker/libcompose/cli/logger"
	config "github.com/docker/libcompose/config"
	"github.com/docker/libcompose/docker"
	"github.com/docker/libcompose/docker/client"
	"github.com/docker/libcompose/project"
	//	"github.com/docker/libcompose/project/events"
	"github.com/docker/libcompose/project/options"
)

// stateConditions is a list of state conditions applied to each service.  This needs to be a global since the
// processConfigs function is called as a callback
var stateConditions map[string]types.StateConditions

// GetIndex will find the index of a value in a slice. if no matching value is found it returns a -1
func GetIndex(slice []string, value string) int {
	for index, item := range slice {
		if value == item {
			return index
		}
	}
	return -1
}

func main() {

	stateConditions = make(map[string]types.StateConditions)

	// process the compose files provided on the command line for additional requirements
	var composeFiles []string
	var err error
	for _, file := range os.Args[1:] {
		composeFiles, err = processRequires(file, composeFiles)
	}

	// create a context for our project
	p_context := docker.Context{
		Context: project.Context{
			ProjectName:   "yeah-compose",
			ComposeFiles:  composeFiles,
			LoggerFactory: logger.NewColorLoggerFactory(),
		},
	}
	// set up some parse options
	parse_options := config.ParseOptions{
		Interpolate: true,
		Validate:    true,
		Preprocess:  processConfig,
	}
	// create our project (this reads in our config among other things)
	p, err := docker.NewProject(&p_context, &parse_options)
	if err != nil {
		log.Fatal(err)
	}

	// grab the configs that we passed in
	serviceConfigs := p.(*project.Project).ServiceConfigs
	fmt.Println(p.(*project.Project).NetworkConfigs)
	// save off our actual services
	services := make(map[string]project.Service)
	// create our services and store them
	for _, name := range serviceConfigs.Keys() {
		service, err := p.CreateService(name)
		if err != nil {
			log.Fatal(err)
		}
		services[name] = service
	}

	ordered_services := sortServices(services)
	fmt.Printf("Services will be started in the following order: %v", ordered_services)

	// create a connection to the docker server
	docker_client, err := client.Create(client.Options{})

	// run through and start up our services
	for _, service_name := range ordered_services {
		// the return status of our monitors
		event_response := make(chan types.ContainerStatus)
		// also an indicator that we no longer need to wait for return status
		done := make(chan struct{})

		fmt.Printf("Starting  up service - %v\n", service_name)
		err = services[service_name].Up(context.Background(), options.Up{options.Create{ForceRecreate: true}})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Started up service - %v\n", service_name)

		// get the container name for this service.
		containers, err := services[service_name].Containers(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		// We only spin up one for each services so we can just grab the first one
		container_name := containers[0].Name()

		// depending on which monitors this service uses we do different things
		// first see if there area ny state conditions at all
		if conditions, found := stateConditions[service_name]; found {
			fmt.Printf("Waiting for conditions: %+v\n", conditions)
			// check if we monitor the exit code
			if conditions.ExitCodes != nil {
				// add in a listener so we can get updates on what is happening
				//container_events := make(chan events.Event, 2)
				//p.AddListener(container_events)
				d_events, err := p.Events(context.Background(), service_name)
				if err != nil {
					log.Fatal(err)
				}
				// listen for events
				go handler.Exit(docker_client, d_events, event_response, conditions.ExitCodes, done)
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
						go handler.Output(docker_client, container_name, true, false, monitors, event_response, done)
					} else if filename == "STDERR" {
						go handler.Output(docker_client, container_name, false, true, monitors, event_response, done)
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
