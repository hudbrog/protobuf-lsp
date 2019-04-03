package main

import (
	"os"

	"github.com/hudbrog/protobuf-lsp/cmd"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(os.Stdout)

	cmd.Execute()
}
