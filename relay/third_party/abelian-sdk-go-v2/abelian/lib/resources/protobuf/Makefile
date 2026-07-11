build:
	pwd
	rm -rf go/
	rm -rf java/
	mkdir -p go java
	protoc -I=. --go_out=./go --java_out=./java *.proto
	cp ./go/abelian.info/sdk/proto/core.pb.go ../../../lib/proto/
