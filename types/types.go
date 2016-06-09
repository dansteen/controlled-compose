package types

import (
	"regexp"
)

// ContianerStatus holds the status and related status message of a container
type ContainerStatus struct {
	Status  string
	Message string
}

// FileMonitor holds information about file monitors for our containers
type FileMonitor struct {
	File   string         `the file to monitor`
	Regex  *regexp.Regexp `the regular expression to look for`
	Status string         `weather to succeed or fail`
}

// ExitCodes holds a list of exit codes
type ExitCodes struct {
	Codes []int
}

// Contains will return true if c contains the value, otherwise false
func (c ExitCodes) Contains(value int) bool {
	for _, item := range c.Codes {
		if value == item {
			return true
		}
	}
	return false
}

// Len returns the number of exit codes in c
func (c ExitCodes) Len() int {
	return len(c.Codes)
}

// Timeout holds information about a timeout that has been specified on a container
type Timeout struct {
	Duration float64
	Status   string
}

// StateConditions holds our conditions tht have been applied to services
type StateConditions struct {
	ExitCodes    *ExitCodes               `the exit code to expect. the value '-1' indicates that the process should not exit`
	FileMonitors map[string][]FileMonitor `a map of map[filepath][]FileMonitor type to store filemonitors`
	Timeout      *Timeout                 `how long we should wait (in seconds) for a success prior to automatically failing.`
}
