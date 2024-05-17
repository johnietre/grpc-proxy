test-proto: tests/proto/*.proto
	protoc \
		--go_out=. --go-grpc_out=. \
		--go_opt=module=github.com/johnietre/grpc-proxy \
		--go-grpc_opt=module=github.com/johnietre/grpc-proxy \
		$^

clean-test: tests/proto/*.go
	rm $^

clean-cargo:
	cargo clean

clean: clean-test clean-cargo
