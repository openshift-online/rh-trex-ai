module github.com/openshift-online/rh-trex-ai/components/control-plane

go 1.26.0

require (
	github.com/openshift-online/rh-trex-ai/components/api-server v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.75.1
)

require (
	golang.org/x/net v0.60.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251014184007-4626949a642f // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

replace github.com/openshift-online/rh-trex-ai/components/api-server => ../api-server
