# grpc-proxy
A basic GRPC proxy.
Currently the Go implementation is far faster than the Rust implementation (at least WRT many unary calls from a single client).

## Directories
- bin - Binaries
- cmd - The main Go files used to build the binary files
- go - The main Go implementation
- proxy - Miscellaneous test small proxies
- rust - The Rust (mostly complete) implementation
- target - Rust generated output
- tests - Tests to test the program

# TODO
- [ ] Go improve command/flag descriptions
- [ ] Improve/move test output to subdirectory
- [ ] Fix --out flag for refresh
- [ ] Add more options to better control tests
- [ ] Add tests to go directory for individual things (standard Go tests)
- [ ] Test all that's yet to be tested
