package log

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

const (
	defaultLogLevelKey = "*"
)

// ParseLogLevel parses complex log level - comma-separated
// list of module:level pairs with an optional *:level pair (* means
// all other modules).
//
// Example:
//		ParseLogLevel("consensus:debug,mempool:debug,*:error", log.Root().SetHandler(StdoutHandler), "info")
func ParseLogLevel(lvl string, logger Logger, defaultLogLevelValue string) (Logger, error) {
	if lvl == "" {
		return nil, errors.New("Empty log level")
	}

	l := lvl

	// prefix simple one word levels (e.g. "info") with "*"
	if !strings.Contains(l, ":") {
		l = defaultLogLevelKey + ":" + l
	}

	options := make([]Option, 0)

	isDefaultLogLevelSet := false
	var option Option
	var err error

	list := strings.Split(l, ",")
	for _, item := range list {
		moduleAndLevel := strings.Split(item, ":")

		if len(moduleAndLevel) != 2 {
			return nil, fmt.Errorf("Expected list in a form of \"module:level\" pairs, given pair %s, list %s", item, list)
		}

		module := moduleAndLevel[0]
		level := moduleAndLevel[1]

		if module == defaultLogLevelKey {
			option, err = AllowLevel(level)
			if err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf("Failed to parse default log level (pair %s, list %s)", item, l))
			}
			options = append(options, option)
			isDefaultLogLevelSet = true
		} else {
			switch level {
			case "trace":
				option = AllowTranceWith("module", module)
			case "debug":
				option = AllowDebugWith("module", module)
			case "info":
				option = AllowInfoWith("module", module)
			case "warn":
				option = AllowWarnWith("module", module)
			case "error":
				option = AllowErrorWith("module", module)
			case "crit":
				option = AllowCritWith("module", module)
			case "none":
				option = AllowNoneWith("module", module)
			default:
				return nil, fmt.Errorf("Expected either \"info\", \"debug\", \"error\" or \"none\" log level, given %s (pair %s, list %s)", level, item, list)
			}
			options = append(options, option)

		}
	}

	// if "*" is not provided, set default global level
	if !isDefaultLogLevelSet {
		option, err = AllowLevel(defaultLogLevelValue)
		if err != nil {
			return nil, err
		}
		options = append(options, option)
	}

	//root.SetHandler(LvlFilterHandler(level, root.GetHandler()))
	base = NewFilter(logger, options...)
	return base, nil
}
