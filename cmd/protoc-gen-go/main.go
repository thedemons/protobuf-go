// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The protoc-gen-go binary is a protoc plugin to generate Go code for
// both proto2 and proto3 versions of the protocol buffer language.
//
// For more information about the usage of this plugin, see:
// https://protobuf.dev/reference/go/go-generated.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gengo "google.golang.org/protobuf/cmd/protoc-gen-go/internal_gengo"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/internal/filedesc"
	"google.golang.org/protobuf/internal/version"
	"google.golang.org/protobuf/types/descriptorpb"
)

const genGoDocURL = "https://protobuf.dev/reference/go/go-generated"
const grpcDocURL = "https://grpc.io/docs/languages/go/quickstart/#regenerate-grpc-code"

func getAliasType(msg *protogen.Message) *protogen.Field {
	if msg == nil || !strings.Contains(string(msg.Comments.Leading), "protobuf:alias") {
		return nil
	}
	if len(msg.Fields) != 1 {
		return nil
	}
	return msg.Fields[0]
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Fprintf(os.Stdout, "%v %v\n", filepath.Base(os.Args[0]), version.String())
		os.Exit(0)
	}
	if len(os.Args) == 2 && os.Args[1] == "--help" {
		fmt.Fprintf(os.Stdout, "See "+genGoDocURL+" for usage information.\n")
		os.Exit(0)
	}

	var (
		flags                                 flag.FlagSet
		plugins                               = flags.String("plugins", "", "deprecated option")
		experimentalStripNonFunctionalCodegen = flags.Bool("experimental_strip_nonfunctional_codegen", false, "experimental_strip_nonfunctional_codegen true means that the plugin will not emit certain parts of the generated code in order to make it possible to compare a proto2/proto3 file with its equivalent (according to proto spec) editions file. Primarily, this is the encoded descriptor.")
	)
	protogen.Options{
		ParamFunc:                    flags.Set,
		InternalStripForEditionsDiff: experimentalStripNonFunctionalCodegen,
	}.Run(func(gen *protogen.Plugin) error {
		if *plugins != "" {
			return errors.New("protoc-gen-go: plugins are not supported; use 'protoc --go-grpc_out=...' to generate gRPC\n\n" +
				"See " + grpcDocURL + " for more information.")
		}
		for _, f := range gen.Files {
			if f.Generate {
				keepMessages := make([]*protogen.Message, 0, len(f.Messages))
				keepMessageTypes := make([]*descriptorpb.DescriptorProto, 0, len(f.Proto.MessageType))

				for msgIdx, msg := range f.Messages {
					if getAliasType(msg) != nil {
						f.Aliases = append(f.Aliases, msg)
						continue
					}

					for fieldIdx, field := range msg.Fields {
						aliasField := getAliasType(field.Message)
						if aliasField == nil {
							continue
						}

						fieldDesc := field.Desc.(*filedesc.Field)
						aliasFieldDesc := aliasField.Desc.(*filedesc.Field)
						fieldDesc.L1.Options = aliasFieldDesc.L1.Options
						fieldDesc.L1.Cardinality = aliasFieldDesc.L1.Cardinality
						fieldDesc.L1.Kind = aliasFieldDesc.L1.Kind
						fieldDesc.L1.IsProto3Optional = aliasFieldDesc.L1.IsProto3Optional
						fieldDesc.L1.Default = aliasFieldDesc.L1.Default
						fieldDesc.L1.EditionFeatures = aliasFieldDesc.L1.EditionFeatures
						fieldDesc.L1.Enum = nil
						fieldDesc.L1.Message = nil

						field.Alias = &field.Message.GoIdent
						field.Message = nil
						field.Oneof = nil
						field.Enum = nil

						kind := descriptorpb.FieldDescriptorProto_Type(fieldDesc.L1.Kind)
						fd := f.Proto.MessageType[msgIdx].Field[fieldIdx]
						fd.Type = &kind
						fd.TypeName = nil
						if fieldDesc.L1.Options != nil {
							fd.Options = fieldDesc.L1.Options().(*descriptorpb.FieldOptions)
						}
					}

					keepMessages = append(keepMessages, msg)
					keepMessageTypes = append(keepMessageTypes, f.Proto.MessageType[msgIdx])
				}
				f.Messages = keepMessages
				f.Proto.MessageType = keepMessageTypes

				gengo.GenerateFile(gen, f)
			}
		}
		gen.SupportedFeatures = gengo.SupportedFeatures
		gen.SupportedEditionsMinimum = gengo.SupportedEditionsMinimum
		gen.SupportedEditionsMaximum = gengo.SupportedEditionsMaximum
		return nil
	})
}
