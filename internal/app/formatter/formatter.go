package formatter

import (
	"fmt"
	"strings"

	protobuf "github.com/golang/protobuf/protoc-gen-go/descriptor"
)

// PrettyPrint - formats data from FileDescriptorProto and pretty prints it to debug log
func PrettyPrint(fd *protobuf.FileDescriptorProto) {
	fmt.Printf("File Name: %s\n", fd.GetName())
	fmt.Printf("Package: %s\n", fd.GetPackage())
	fmt.Printf("Messages:\n")
	printMessages(fd.GetMessageType(), 1)
	fmt.Printf("Enums:\n")
	printEnums(fd.GetEnumType())
	fmt.Printf("Services:\n")
	printServices(fd.GetService())
}

func printMessages(msgs []*protobuf.DescriptorProto, level int) {
	indent := strings.Repeat("\t", level)
	for i, v := range msgs {
		fmt.Printf("%s%d) %s\n", indent, i, v.GetName())
		if len(v.GetNestedType()) != 0 {
			printMessages(v.GetNestedType(), level+1)
		}
	}
}

func printEnums(enums []*protobuf.EnumDescriptorProto) {
	for i, v := range enums {
		fmt.Printf("%d) %s\n", i, v.GetName())
		printEnumValues(v.GetValue())
	}
}

func printEnumValues(ev []*protobuf.EnumValueDescriptorProto) {
	for _, v := range ev {
		deprecated := ""
		if v.GetOptions().GetDeprecated() {
			deprecated = "*deprecetad*"
		}
		fmt.Printf("\t%d - %s %s\n", v.GetNumber(), v.GetName(), deprecated)
	}
}

func printServices(services []*protobuf.ServiceDescriptorProto) {
	for i, v := range services {
		fmt.Printf("%d) %s\n", i, v.GetName())
		printMethods(v.GetMethod())
	}
}

func printMethods(methods []*protobuf.MethodDescriptorProto) {
	for i, v := range methods {
		fmt.Printf("\t%d) %s(%s,%s)\n", i, v.GetName(), v.GetInputType(), v.GetOutputType())
	}
}
