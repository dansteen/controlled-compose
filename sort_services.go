package main

import (
	go_graph "github.com/alonsovidales/go_graph"
	"github.com/docker/libcompose/project"
)

// sortServices build a sorted list of services based on each services dependencies.
// We use a topological sort for this
func sortServices(services map[string]project.Service) []string {
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
	return ordered_services
}
