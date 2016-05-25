package main

import (
	"golang.org/x/net/context"
	"log"
	"os"

	"github.com/docker/libcompose/docker"
	"github.com/docker/libcompose/project"
	"github.com/docker/libcompose/project/options"
)

func main() {
	project, err := docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeFiles: []string{os.Arg[1]},
			ProjectName:  "yeah-compose",
		},
	}, nil)

	if err != nil {
		log.Fatal(err)
	}

	err = project.Up(context.Background(), options.Up{})

	if err != nil {
		log.Fatal(err)
	}
}
