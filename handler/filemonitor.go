// Handler provides various state hanlders for our controlled compose run
package handler

import (
	"fmt"
	"github.com/dansteen/controlled-compose/types"
	"github.com/hpcloud/tail"
	"log"
)

// FileMonitor handles state conditions that result from content written to files
func FileMonitor(filename string, monitors []types.FileMonitor, container_status chan<- types.ContainerStatus, done <-chan struct{}) {
	// tail our file
	tail, err := tail.TailFile(filename, tail.Config{Follow: true, ReOpen: true, MustExist: false, Logger: tail.DiscardingLogger})
	if err != nil {
		log.Fatal(err)
	}

	// then check for our regexs
	for line := range tail.Lines {
		for _, monitor := range monitors {
			if monitor.Regex.Match([]byte(line.Text)) == true {
				container_status <- types.ContainerStatus{
					Status:  monitor.Status,
					Message: fmt.Sprintf("%v matched %v.  %v.\n", line.Text, monitor.Regex.String(), monitor.Status),
				}
				// once we have found a match we don't continue
				return
			}
			// if we get signalled that we are done we also exit
			select {
			case <-done:
				fmt.Printf("Exiting filemonitor for %v\n", filename)
				return
			default:
				// nothing
			}
		}
	}
}
