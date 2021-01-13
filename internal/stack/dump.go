package stack

import (
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/pkg/errors"
	"os"
)

type DumpOptions struct {
	Output string
}

// Dump function exports stack data and dumps them as local artifacts, which can be used for debug purposes.
func Dump(options DumpOptions) error {
	logger.Debugf("Dump Elastic stack data")

	err := dumpStackLogs(options)
	if err != nil {
		return errors.Wrap(err, "can't dump Elastic stack logs")
	}
	return nil
}

func dumpStackLogs(options DumpOptions) error {
	logger.Debugf("Dump stack logs")

	logger.Debugf("Recreate the output location (path: %s)", options.Output)
	err := os.RemoveAll(options.Output)
	if err != nil {
		return errors.Wrap(err, "can't remove output location")
	}

	err = os.MkdirAll(options.Output, 0755)
	if err != nil {
		return errors.Wrap(err, "can't create output location")
	}


}