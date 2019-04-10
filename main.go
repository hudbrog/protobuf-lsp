package main

import (
	"os"

	"github.com/hudbrog/protobuf-lsp/cmd"
	"github.com/sirupsen/logrus"
)

func main() {
	MyLogger := new(logrus.Logger)
	MyLogger.SetFormatter(&logrus.TextFormatter{})
	MyLogger.SetLevel(logrus.DebugLevel)
	f, err := os.OpenFile("filename.log", os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return
	}
	MyLogger.SetOutput(f)

	cmd.Execute(MyLogger)
}
