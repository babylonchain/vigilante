package log

import (
	"os"

	"github.com/sirupsen/logrus"
)

var Logger = &logrus.Logger{
	Out:   os.Stderr,
	Level: logrus.DebugLevel,
	Formatter: &logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	},
}
