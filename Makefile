.PHONY: proto-cpp proto-node

proto-player:
	protoc --go_out=./player/protos --go_opt=paths=source_relative \
	--go-grpc_out=./player/protos --go-grpc_opt=paths=source_relative \
    --proto_path=./protos ./protos/player.proto