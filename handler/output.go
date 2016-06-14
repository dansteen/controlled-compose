// Handler provides various state hanlders for our controlled compose run
package handler

import (
	"bufio"
	"fmt"
	"github.com/dansteen/controlled-compose/types"
	"github.com/docker/engine-api/client"
	dockerTypes "github.com/docker/engine-api/types"
	"golang.org/x/net/context"
	"log"
)

// Output will handle state conditions based on STDOUT or STDERR content
func Output(client client.APIClient, container_name string, stdout bool, stderr bool, monitors []types.FileMonitor, container_status chan<- types.ContainerStatus, done <-chan struct{}) {
	// if the filename is STDOUT or STDERR we handle it specially
	logReadCloser, err := client.ContainerLogs(context.Background(), container_name, dockerTypes.ContainerLogsOptions{
		ShowStdout: stdout,
		ShowStderr: stderr,
		Follow:     true,
		Tail:       "all",
	})
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(logReadCloser)
	defer logReadCloser.Close()
	// then check for our regexs
	for scanner.Scan() {
		for _, monitor := range monitors {
			if monitor.Regex.Match([]byte(scanner.Text())) == true {
				container_status <- types.ContainerStatus{
					Status:  monitor.Status,
					Message: fmt.Sprintf("%v matched %v.  %v.\n", scanner.Text(), monitor.Regex.String(), monitor.Status),
				}
				// once we have found a match we don't continue
				return
			}
			// if we get the message that we are done, we also exit
			select {
			case <-done:
				fmt.Printf("Exiting output handler for %v\n", container_name)
				return
			default:
				// nothing
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
