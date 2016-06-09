package main

import (
	"fmt"
	"github.com/dansteen/controlled-compose/handler"
	"github.com/dansteen/controlled-compose/types"
	"golang.org/x/net/context"
	"io/ioutil"
	"log"
	"os"
	path "path/filepath"

	"regexp"
	"strings"
	"sync"

	yaml "github.com/cloudfoundry-incubator/candiedyaml"
	config "github.com/docker/libcompose/config"
	"github.com/docker/libcompose/docker"
	"github.com/docker/libcompose/docker/client"
	"github.com/docker/libcompose/project"
	"github.com/docker/libcompose/project/events"
	"github.com/docker/libcompose/project/options"
	"github.com/docker/libcompose/utils"
	//"github.com/kr/pretty"
	go_graph "github.com/alonsovidales/go_graph"
)

var stateConditions map[string]types.StateConditions

// this function will handle our config preprocessing.  We need to save off the parts of the config that we created
// as they are not passed through to the general config
func processConfig(services config.RawServiceMap) (config.RawServiceMap, error) {
	for name, config := range services {
		// collect our exit conditions
		exits := types.StateConditions{}

		// look for exit codes
		if val, ok := config["exit"]; ok {
			exits.ExitCodes = &types.ExitCodes{Codes: val.([]int)}
		}

		// look for log monitors
		if _, ok := config["filemonitor"]; ok {
			exits.FileMonitors = make(map[string][]types.FileMonitor)
			for _, monitorRaw := range config["filemonitor"].([]interface{}) {
				monitor := monitorRaw.(map[interface{}]interface{})
				// make sure our regex is valid
				regex, err := regexp.Compile(monitor["regex"].(string))
				if err != nil {
					log.Fatal(err)
				}

				// we need to make sure that any folders that are being monitored are exported, so we add any missing ones
				// we only do this if we are not monitoring STDOUT
				if monitor["file"] != "STDOUT" {
					// first we get the directory of the log we are monitoring
					dir := path.Dir(monitor["file"].(string))
					found := false
					// then we check if there are any volumes exported
					if _, found := services[name]["volumes"]; found {
						// then we check to see if our folder is present in the already exported folders for this service
						for _, volume := range services[name]["volumes"].([]string) {
							// break out the parts
							parts := strings.FieldsFunc(volume, func(c rune) bool { return c == ':' })
							if parts[1] == dir {
								found = true
								break
							}
						}
						// if no volumes were exported, we need to create the structures
					} else {
						services[name]["volumes"] = make([]string, 0)
					}

					// if we have not found it, then we add it in.  directories exported by this are named in the following fashion:
					// <current_dir>/controlled_compose_<pid>/<service_name>
					if !found {
						// build our export dir
						currDir, _ := os.Getwd()
						exportDir := path.Join(currDir, fmt.Sprintf("controlled_compose_%v", os.Getpid()), name)
						// add in our volume
						services[name]["volumes"] = append(services[name]["volumes"].([]string), fmt.Sprintf("%v:%v", dir, exportDir))
					}
				}

				// if we have not yet seen this filename we create a new storage array
				filename := monitor["file"].(string)
				if _, found := exits.FileMonitors[filename]; !found {
					exits.FileMonitors[filename] = make([]types.FileMonitor, 0)
				}
				exits.FileMonitors[filename] = append(exits.FileMonitors[filename], types.FileMonitor{
					File:   filename,
					Regex:  regex,
					Status: monitor["status"].(string),
				})
			}
		}
		// look for timeout
		if timeout, ok := config["timeout"]; ok {
			timeout := timeout.(map[interface{}]interface{})
			exits.Timeout = &types.Timeout{
				Duration: timeout["duration"].(float64),
				Status:   timeout["status"].(string),
			}
		}

		// add the exits we found to our list
		stateConditions[name] = exits
	}
	return services, nil
}

// GetIndex will find the index of a value in a slice. if no matching value is found it returns a -1
func GetIndex(slice []string, value string) int {
	for index, item := range slice {
		if value == item {
			return index
		}
	}
	return -1
}

// processRequires reads in config files, scan for a "require" stanza, and then recursively process each
// file that is in that stanza.  Processing is done depth-first, and only the first instance of each file is
// processed.
func processRequires(file string, configFiles []string) ([]string, error) {
	// To parse our requires stanzas, we need to do our own unmarshaling since libcompose doesn't give
	// us access to a structured version of the config as a whole once it has processed it.
	// first read in the file provided
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	// then parse it for require stanzas
	var requires Requires
	err = yaml.Unmarshal(content, &requires)
	if err != nil {
		return nil, err
	}

	// add our file to the processed list
	newFiles := append(configFiles, file)

	// then we parse each additional requirement found
	for _, require := range requires.Require {
		// requires are relative to the file being processed, so we add in the dirname for the current file
		require = path.Join(path.Dir(file), require)

		// we only process files that are not already in our list
		if !utils.Contains(configFiles, require) {
			newFiles, err = processRequires(require, newFiles)
			if err != nil {
				return nil, err
			}
		}
	}
	return newFiles, nil
}

// setup our go routine sync
var wg sync.WaitGroup

type Requires struct {
	Require []string
}

func main() {

	// set up our list of exit conditions
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
			ProjectName:  "yeah-compose",
			ComposeFiles: composeFiles,
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

	// we need to build an ordered list of our services.  We use a topological sort for this.
	// first generate an array of service names
	var service_names []string
	for name, _ := range services {
		service_names = append(service_names, name)
	}
	// run through and generate graph edges for our services
	var edges []go_graph.Edge
	for index, name := range service_names {
		for _, dep := range services[name].DependentServices() {
			edges = append(edges, go_graph.Edge{
				From: uint64(index),
				To:   uint64(GetIndex(service_names, dep.Target)),
			})
		}
	}
	// build the graph and generate an ordered list of services
	graph := go_graph.GetGraph(edges, false)
	ordered_indecies, dag := graph.TopologicalOrder()
	var ordered_services []string
	if dag {
		for _, value := range ordered_indecies {
			ordered_services = append(ordered_services, service_names[value])
		}
	}
	// for some reason the results come back inverted so we reverse the array
	for i, j := 0, len(ordered_services)-1; i < j; i, j = i+1, j-1 {
		ordered_services[i], ordered_services[j] = ordered_services[j], ordered_services[i]
	}
	fmt.Println(ordered_services)

	// create a connection to the docker server
	docker_client, err := client.Create(client.Options{})

	// run through and start up our services
	for _, service_name := range ordered_services {
		// the return status of our monitors
		event_response := make(chan types.ContainerStatus)
		// spin up our service
		services[service_name].Up(context.Background(), options.Up{})
		// get the container name for this service.
		containers, err := services[service_name].Containers(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		// We only spin up one for each services so we can just grab the first one
		container_name := containers[0].Name()

		// depending on which monitors this service uses we do different things
		// but we need to know if we should wait ir not either way
		hasStateCondition := false
		// check if we monitor the exit code
		if stateConditions[service_name].ExitCodes != nil {
			hasStateCondition = true
			// add in a listener so we can get updates on what is happening
			container_events := make(chan events.Event, 2)
			p.AddListener(container_events)
			d_events, err := p.Events(context.Background(), service_name)
			if err != nil {
				log.Fatal(err)
			}
			// listen for events
			wg.Add(1)
			go handler.Exit(docker_client, d_events, event_response, stateConditions[service_name].ExitCodes)
		}

		// check if we have configured a timeout
		if stateConditions[service_name].Timeout != nil {
			hasStateCondition = true
			wg.Add(1)
			go handler.Timeout(stateConditions[service_name].Timeout, event_response)
		}

		// check if we have  log monitors
		if stateConditions[service_name].FileMonitors != nil {
			hasStateCondition = true
			// run a handler for each file
			for filename, monitors := range stateConditions[service_name].FileMonitors {
				wg.Add(1)
				// depending on what time of file/output we are monitoring we do things a bit differently
				if filename == "STDOUT" {
					go handler.Output(docker_client, container_name, true, false, monitors, event_response)
				} else if filename == "STDERR" {
					go handler.Output(docker_client, container_name, false, true, monitors, event_response)
				} else {
					go handler.FileMonitor(filename, monitors, event_response)
				}
			}
		}

		// wait until we have been given the go-ahead to move on to the next service if we need to
		if hasStateCondition {
			response := <-event_response
			fmt.Println(response)
			// we have to be sure to close this as some functions may still be running
			close(event_response)
		}

	}

	if err != nil {
		log.Fatal(err)
	}

	// wait for our go routines to finish
	wg.Wait()
}
