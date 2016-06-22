package control

import (
	yaml "github.com/cloudfoundry-incubator/candiedyaml"
	"github.com/dansteen/controlled-compose/types"
	"github.com/docker/libcompose/utils"
	"io/ioutil"
	"path/filepath"
)

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
	var requires types.Requires
	err = yaml.Unmarshal(content, &requires)
	if err != nil {
		return nil, err
	}

	// add our file to the processed list
	newFiles := append(configFiles, file)

	// then we parse each additional requirement found
	for _, require := range requires.Require {
		// requires are relative to the file being processed, so we add in the dirname for the current file
		require = filepath.Join(filepath.Dir(file), require)

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

// consumeConfig reads in a config file, and returns it as a byte array
func consumeConfig(file string) ([]byte, error) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return content, err
}
