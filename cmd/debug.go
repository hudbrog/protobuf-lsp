package cmd

import (
	"io/ioutil"

	"github.com/golang/protobuf/proto"
	protobuf "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/hudbrog/protobuf-lsp/internal/app/formatter"
	"github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

type debugParams struct {
	inputFile string
}

func init() {
	params := &debugParams{}
	var debugCmd = &cobra.Command{
		Use:   "debug",
		Short: "Used for debugging purposes",
		Run: func(cmd *cobra.Command, args []string) {
			runDebug(params)
		},
	}
	rootCmd.AddCommand(debugCmd)
	debugCmd.Flags().StringVarP(&params.inputFile, "input", "i", "", "FileDescriptor File to work with")
	debugCmd.MarkFlagRequired("input")
}

func runDebug(params *debugParams) {
	logrus.Debugf("Trying to read: %s", params.inputFile)

	in, err := ioutil.ReadFile(params.inputFile)
	if err != nil {
		logrus.Fatalln("Error reading file:", err)
	}
	fd := new(protobuf.FileDescriptorSet)
	if err := proto.Unmarshal(in, fd); err != nil {
		logrus.Fatalln("Failed to parse FileDescriptor:", err)
	}

	for _, v := range fd.GetFile() {
		formatter.PrettyPrint(v)
	}
}
