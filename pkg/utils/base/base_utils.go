package base

import (
	"fmt"
	"os"
	"os/exec"
)

func Getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func MergeMapString(stringMapList ...map[string]string) map[string]string {
	res := map[string]string{}
	for _, stringMap := range stringMapList {
		for k, v := range stringMap {
			if _, ok := res[k]; ok {
				continue
			}
			res[k] = v
		}
	}
	return res
}

// Output runs a specified command and pretty prints an possible error
func Output(command string, args ...string) (string, error) {
	output, err := exec.Command(command, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Error in command: %v => %s", err, string(output))
	}

	return string(output), nil
}
