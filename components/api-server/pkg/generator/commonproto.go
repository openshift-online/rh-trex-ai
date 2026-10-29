package generator

import (
	"os"
	"path/filepath"
	"text/template"
)

// commonProtoTemplate is the canonical common.proto every generated entity
// proto imports for ObjectReference, ListMeta, Error and EventType. It is
// written by the generator into a project's proto package when missing, so a
// downstream project can use any proto_package without providing the file
// itself.
const commonProtoTemplate = `syntax = "proto3";

package {{.ProtoPackage}}.v1;

option go_package = "{{.Module}}/pkg/api/grpc/{{.ProtoPackage}}/v1;{{.ProtoPackage}}_v1";

import "google/protobuf/timestamp.proto";

message ObjectReference {
  string id = 1;
  google.protobuf.Timestamp created_at = 2;
  google.protobuf.Timestamp updated_at = 3;
  string kind = 4;
  string href = 5;
}

message ListMeta {
  int32 page = 1;
  int32 size = 2;
  int32 total = 3;
}

message Error {
  string id = 1;
  string kind = 2;
  string href = 3;
  int32 code = 4;
  string reason = 5;
  string operation_id = 6;
}

enum EventType {
  EVENT_TYPE_UNSPECIFIED = 0;
  EVENT_TYPE_CREATED = 1;
  EVENT_TYPE_UPDATED = 2;
  EVENT_TYPE_DELETED = 3;
}
`

// ensureCommonProto writes common.proto into the proto package directory when
// it does not already exist. The entity templates import
// "<package>/v1/common.proto", so downstream projects need this file for
// whatever proto_package they configure.
func ensureCommonProto(dir string, data TemplateData) error {
	path := filepath.Join(dir, "common.proto")
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	tmpl, err := template.New("common.proto").Parse(commonProtoTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return tmpl.Execute(f, data)
}
