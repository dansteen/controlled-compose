package control

import (
	"fmt"
	"github.com/twmb/algoimpl/go/graph"
	"os"

	"github.com/dansteen/controlled-compose/types"
	"github.com/docker/libcompose/cli/logger"
	config "github.com/docker/libcompose/config"
	"github.com/docker/libcompose/docker"
	"github.com/docker/libcompose/project"
	"golang.org/x/net/context"
)

type Project struct {
	StateConditions map[string]types.StateConditions
	ComposeProject  project.APIProject
	Services        map[string]project.Service
	appVersions     []string
}

// GenProject will generate a Project object using the config files passed in
func GenProject(name string, files []string, appVersions []string) (Project, error) {

	// create our project object
	p := Project{
		StateConditions: make(map[string]types.StateConditions),
	}

	// set our app verions for consumption by processConfig
	p.appVersions = appVersions

	// process the compose files provided on the command line for additional requirements
	var composeFiles []string
	var composeBytes [][]byte
	var err error
	for _, file := range files {
		composeFiles, err = processRequires(file, composeFiles)
		if err != nil {
			return p, err
		}
	}
	// we slurp our configs manually to bypass odd docker working directory behavior
	for _, file := range composeFiles {
		content, err := consumeConfig(file)
		if err != nil {
			return p, err
		}
		composeBytes = append(composeBytes, content)
	}

	// create a context for our project
	p_context := docker.Context{
		Context: project.Context{
			ProjectName:         name,
			ComposeFiles:        []string{"-"},
			ComposeBytes:        composeBytes,
			LoggerFactory:       logger.NewColorLoggerFactory(),
			IgnoreMissingConfig: false,
		},
	}

	// set up some parse options
	parse_options := config.ParseOptions{
		Interpolate: true,
		Validate:    true,
		Preprocess:  p.processConfig,
	}
	// create our project (this reads in our config among other things)
	project, err := docker.NewProject(&p_context, &parse_options)
	if err != nil {
		return p, err
	}

	p.ComposeProject = project

	// generate our services
	err = p.genServices()

	return p, err

}

// SortedServices build a sorted list of services based on each services dependencies.
// We use a topological sort for this
func (p *Project) SortedServices() []string {
	// create a new graph
	ourGraph := graph.New(graph.Directed)
	// a place to store our nodes
	nodes := make(map[string]graph.Node)

	// add in nodes for each of our services
	for name, _ := range p.Services {
		nodes[name] = ourGraph.MakeNode()
		// hook the data back into the graph (not strictly required)
		*nodes[name].Value = name
	}

	// add in our edges
	for name, service := range p.Services {
		for _, dep := range service.DependentServices() {
			// make sure the dependency exists
			if _, found := nodes[dep.Target]; !found {
				fmt.Printf("Error: Service %v depends on service %v which is not included in the config\n", name, dep.Target)
				os.Exit(1)
			}
			// add in an edge for this dependency
			ourGraph.MakeEdge(nodes[dep.Target], nodes[name])
		}
	}

	// do our sort
	sorted := ourGraph.TopologicalSort()

	// generate an array of names
	orderedServiceNames := make([]string, 0)
	for _, node := range sorted {
		orderedServiceNames = append(orderedServiceNames, (*node.Value).(string))
	}

	return orderedServiceNames
}

// Container will return the containers associated with a particular service
func (p *Project) Containers(name string) ([]project.Container, error) {
	containers, err := p.Services[name].Containers(context.Background())
	return containers, err
}

// genServices will generate services for each config passed into the supplied project
func (p *Project) genServices() error {

	// grab the configs that we passed in
	serviceConfigs := p.ComposeProject.(*project.Project).ServiceConfigs
	// save off our actual services
	services := make(map[string]project.Service)
	// create our services and store them
	for _, name := range serviceConfigs.Keys() {
		service, err := p.ComposeProject.CreateService(name)
		if err != nil {
			return err
		}
		services[name] = service
	}
	p.Services = services
	return nil
}
