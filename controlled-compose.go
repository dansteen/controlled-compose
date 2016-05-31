package main

import (
	"fmt"
	"golang.org/x/net/context"
	"log"
	"os"
	"sync"
	"time"

	docker_events "github.com/docker/engine-api/types/events"
	"github.com/docker/libcompose/docker"
	"github.com/docker/libcompose/project"
	"github.com/docker/libcompose/project/events"
	"github.com/docker/libcompose/project/options"
)

// this function will process events that happen on our containers
func processEvents(p *project.Project, container_events chan docker_events.Message) {

	for event := range container_events {
		fmt.Printf("%+v\n", event)
	}

}

// setup our go routine sync
var wg sync.WaitGroup

func main() {

	// create a context for our project
	p_context := docker.Context{
		Context: project.Context{
			ProjectName:  "yeah-compose",
			ComposeFiles: os.Args[1:],
		},
	}

	// fmt.Printf("%+v\n", p_context.Context.ComposeFiles)

	// create our project (this pulls in our config)
	p, err := docker.NewProject(&p_context, nil)

	if err != nil {
		log.Fatal(err)
	}

	// add in a listener so we can get updates on what is happening
	container_events := make(chan events.Event, 2)
	p.AddListener(container_events)
	d_events, err := p.Events(context.Background(), "moto.org-api-init.tmp")
	// run our event processor so it can pick up the events as they come in
	wg.Add(1)
	go processEvents(p.(*project.Project), d_events)

	// grab our service names
	//services := p.(*project.Project).ServiceConfigs
	//fmt.Printf("%+v\n", services.Keys()[1])

	// start up our services
	service, err := p.CreateService("moto.org-api-init.tmp")
	//fmt.Printf("%+v\n", service)
	service.Up(context.Background(), options.Up{})
	fmt.Printf("%v\n", <-container_events)

	// get a connection to our docker host
	client := p_context.ClientFactory.Create(service)
	containers, err := service.Containers(context.Background())
	container_id, err := containers[0].ID()
	//client.Events(p_context, types.EventOptions{})

	info, err := client.ContainerInspect(context.Background(), container_id)
	for info.ContainerJSONBase.State.Running == true {
		info, _ = client.ContainerInspect(context.Background(), container_id)
		fmt.Printf("%+v\n", info.ContainerJSONBase.State.Running)
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("%+v\n", info.ContainerJSONBase.State.ExitCode)
	fmt.Printf("%v\n", <-container_events)
	fmt.Println("hello")

	//info, err := service.Info(context.Background(), false)
	//fmt.Printf("%+v\n", info)
	//err = project.Up(context.Background(), options.Up{})

	if err != nil {
		log.Fatal(err)
	}

	// wait for our go routines to finish
	wg.Wait()
}
