// Handler provides various state hanlders for our controlled compose run
package handler

import (
	"fmt"
	"github.com/dansteen/controlled-compose/types"
	"time"
)

// Timeout will handle timeout state conditions
func Timeout(timeout *types.Timeout, timeout_triggered chan<- types.ContainerStatus) {
	// start a timer
	timer := time.NewTimer(time.Second * time.Duration(timeout.Duration))
	// wait for the timer to run out
	<-timer.C
	// respond
	timeout_triggered <- types.ContainerStatus{
		Status:  timeout.Status,
		Message: fmt.Sprintf("%v triggered after %v seconds", timeout.Status, timeout.Duration),
	}
}
