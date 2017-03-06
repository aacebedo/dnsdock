/* loggers.go
 *
 * Copyright (C) 2016 Alexandre ACEBEDO
 *
 * This software may be modified and distributed under the terms
 * of the MIT license.  See the LICENSE file for details.
 */

package utils

import (
	"github.com/op/go-logging"
	"io/ioutil"
	"os"
)

// InitLoggers initialize loggers
func InitLoggers(verbosity int) (err error) {
	var format logging.Formatter

	var backend logging.Backend

	switch {
	case verbosity == 0:
		backend = logging.NewLogBackend(ioutil.Discard, "", 0)
	case verbosity >= 1:
		backend = logging.NewLogBackend(os.Stdout, "", 0)
	}

	format = logging.MustStringFormatter(`%{color}%{time:15:04:05.000} | %{level:.10s} ▶%{color:reset} %{message}`)

	formatter := logging.NewBackendFormatter(backend, format)
	leveledBackend := logging.AddModuleLevel(formatter)

	switch {
	case verbosity == 1:
		leveledBackend.SetLevel(logging.INFO, "")
	case verbosity >= 2:
		leveledBackend.SetLevel(logging.DEBUG, "")
	}

	logging.SetBackend(leveledBackend)
	return
}
