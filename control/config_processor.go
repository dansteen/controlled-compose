package control

import (
	"fmt"
	"github.com/dansteen/controlled-compose/types"
	"github.com/docker/libcompose/config"
	"github.com/imdario/mergo.git"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// processConfig handles our config preprocessing.  We need to save off the parts of the config that we created
// as they are not passed through to the general config
func (p *Project) processConfig(services config.RawServiceMap) (config.RawServiceMap, error) {
	for name, config := range services {

		// see if we need to swap out the version of this container
		if image, found := config["image"]; found {
			// strip off the version and repo ( if there is one)
			imageParts := strings.Split(image.(string), ":")
			imageParts = strings.Split(imageParts[0], "/")
			imageName := imageParts[len(imageParts)-1]
			// see if the name matches
			for _, app := range p.appVersions {
				// strip off the app version and repo
				appParts := strings.Split(app, ":")
				appParts = strings.Split(appParts[0], "/")
				appName := appParts[len(appParts)-1]
				if appName == imageName {
					config["image"] = app
					break
				}
			}
		}

		var serviceName string
		// see if this service extends another. if so, apply the state_conditions to that other service
		if extendsService, found := config["extends"]; found {
			extendedService := extendsService.(map[interface{}]interface{})
			serviceName = extendedService["service"].(string)
		} else {
			serviceName = name
		}

		// see if we have any state conditions applied
		if configState, found := config["state_conditions"]; found {
			configStateConditions := configState.(map[interface{}]interface{})
			// collect our exit conditions
			conditions := types.StateConditions{}

			// look for exit codes
			if codes, ok := configStateConditions["exit"]; ok {
				// this is an array, but since it's formed as an interface we need to iterate and convert
				codesInt := make([]int, 0)
				for _, val := range codes.([]interface{}) {
					codesInt = append(codesInt, int(val.(int64)))
				}
				conditions.ExitCodes = &types.ExitCodes{Codes: codesInt}
			}

			// look for log monitors
			if _, ok := configStateConditions["filemonitor"]; ok {
				conditions.FileMonitors = make(map[string][]types.FileMonitor)
				for _, monitorRaw := range configStateConditions["filemonitor"].([]interface{}) {
					monitor := monitorRaw.(map[interface{}]interface{})
					// make sure our regex is valid
					regex, err := regexp.Compile(monitor["regex"].(string))
					if err != nil {
						return nil, err
					}

					// we need to make sure that any folders that are being monitored are exported, so we add any missing ones
					// we only do this if we are not monitoring STDOUT
					if monitor["file"] != "STDOUT" && monitor["file"] != "STDERR" {
						// first we get the directory of the log we are monitoring
						dir := filepath.Dir(monitor["file"].(string))
						found := false
						// then we check if there are any volumes exported
						if _, found := services[serviceName]["volumes"]; found {
							// then we check to see if our folder is present in the already exported folders for this service
							for _, val := range services[serviceName]["volumes"].([]interface{}) {
								volume := val.(string)
								// break out the parts
								parts := strings.FieldsFunc(volume, func(c rune) bool { return c == ':' })
								if parts[1] == dir {
									found = true
									break
								}
							}
							// if no volumes were exported, we need to create the structures
						} else {
							services[serviceName]["volumes"] = make([]interface{}, 0)
						}

						// if we have not found it, then we add it in.  directories exported by this are named in the following fashion:
						// <current_dir>/controlled_compose_<pid>/<service_name>
						if !found {
							// build our export dir
							currDir, _ := os.Getwd()
							exportDir := filepath.Join(currDir, fmt.Sprintf("controlled_compose_%v", os.Getpid()), serviceName)
							// add in our volume
							services[serviceName]["volumes"] = append(services[serviceName]["volumes"].([]interface{}), fmt.Sprintf("%v:%v", dir, exportDir))
						}
					}

					// if we have not yet seen this filename we create a new storage array
					filename := monitor["file"].(string)
					if _, found := conditions.FileMonitors[filename]; !found {
						conditions.FileMonitors[filename] = make([]types.FileMonitor, 0)
					}
					conditions.FileMonitors[filename] = append(conditions.FileMonitors[filename], types.FileMonitor{
						File:   filename,
						Regex:  regex,
						Status: monitor["status"].(string),
					})
				}
			}
			// look for timeout
			if timeout, ok := configStateConditions["timeout"]; ok {
				timeout := timeout.(map[interface{}]interface{})
				conditions.Timeout = &types.Timeout{
					// TODO: technically we should be able to accept partial seconds here, but I have to figure out how to do arbitrary
					// conversion from int64 to float64 based on what is read in by the yaml parser. So for now we only accept whole seconds
					Duration: float64(timeout["duration"].(int64)),
					Status:   timeout["status"].(string),
				}
			}
			// add the conditions we found to our list.  merge them if we have already set one
			if existingConditions, found := p.StateConditions[serviceName]; found {
				// if we are setting this via extends, the values take precedence
				if serviceName != name {
					mergo.Merge(&conditions, existingConditions)
					p.StateConditions[serviceName] = conditions
				} else {
					mergo.Merge(&existingConditions, conditions)
					p.StateConditions[serviceName] = existingConditions
				}
			} else {
				// if this is a new value, we just add it in
				p.StateConditions[serviceName] = conditions
			}
		}
	}
	return services, nil
}
