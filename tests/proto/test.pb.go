// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.22.2
// source: tests/proto/test.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type TestRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Lower string `protobuf:"bytes,1,opt,name=lower,proto3" json:"lower,omitempty"`
}

func (x *TestRequest) Reset() {
	*x = TestRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tests_proto_test_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TestRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TestRequest) ProtoMessage() {}

func (x *TestRequest) ProtoReflect() protoreflect.Message {
	mi := &file_tests_proto_test_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TestRequest.ProtoReflect.Descriptor instead.
func (*TestRequest) Descriptor() ([]byte, []int) {
	return file_tests_proto_test_proto_rawDescGZIP(), []int{0}
}

func (x *TestRequest) GetLower() string {
	if x != nil {
		return x.Lower
	}
	return ""
}

type TestResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Upper string `protobuf:"bytes,1,opt,name=upper,proto3" json:"upper,omitempty"`
}

func (x *TestResponse) Reset() {
	*x = TestResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tests_proto_test_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TestResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TestResponse) ProtoMessage() {}

func (x *TestResponse) ProtoReflect() protoreflect.Message {
	mi := &file_tests_proto_test_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TestResponse.ProtoReflect.Descriptor instead.
func (*TestResponse) Descriptor() ([]byte, []int) {
	return file_tests_proto_test_proto_rawDescGZIP(), []int{1}
}

func (x *TestResponse) GetUpper() string {
	if x != nil {
		return x.Upper
	}
	return ""
}

type TestClientStreamRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Num uint64 `protobuf:"varint,1,opt,name=num,proto3" json:"num,omitempty"`
}

func (x *TestClientStreamRequest) Reset() {
	*x = TestClientStreamRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tests_proto_test_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TestClientStreamRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TestClientStreamRequest) ProtoMessage() {}

func (x *TestClientStreamRequest) ProtoReflect() protoreflect.Message {
	mi := &file_tests_proto_test_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TestClientStreamRequest.ProtoReflect.Descriptor instead.
func (*TestClientStreamRequest) Descriptor() ([]byte, []int) {
	return file_tests_proto_test_proto_rawDescGZIP(), []int{2}
}

func (x *TestClientStreamRequest) GetNum() uint64 {
	if x != nil {
		return x.Num
	}
	return 0
}

type TestClientStreamResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Sum uint64 `protobuf:"varint,1,opt,name=sum,proto3" json:"sum,omitempty"`
}

func (x *TestClientStreamResponse) Reset() {
	*x = TestClientStreamResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tests_proto_test_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TestClientStreamResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TestClientStreamResponse) ProtoMessage() {}

func (x *TestClientStreamResponse) ProtoReflect() protoreflect.Message {
	mi := &file_tests_proto_test_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TestClientStreamResponse.ProtoReflect.Descriptor instead.
func (*TestClientStreamResponse) Descriptor() ([]byte, []int) {
	return file_tests_proto_test_proto_rawDescGZIP(), []int{3}
}

func (x *TestClientStreamResponse) GetSum() uint64 {
	if x != nil {
		return x.Sum
	}
	return 0
}

type TestServerStreamRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	FibNum uint32 `protobuf:"varint,1,opt,name=fib_num,json=fibNum,proto3" json:"fib_num,omitempty"`
}

func (x *TestServerStreamRequest) Reset() {
	*x = TestServerStreamRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tests_proto_test_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TestServerStreamRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TestServerStreamRequest) ProtoMessage() {}

func (x *TestServerStreamRequest) ProtoReflect() protoreflect.Message {
	mi := &file_tests_proto_test_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TestServerStreamRequest.ProtoReflect.Descriptor instead.
func (*TestServerStreamRequest) Descriptor() ([]byte, []int) {
	return file_tests_proto_test_proto_rawDescGZIP(), []int{4}
}

func (x *TestServerStreamRequest) GetFibNum() uint32 {
	if x != nil {
		return x.FibNum
	}
	return 0
}

type TestServerStreamResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Num uint64 `protobuf:"varint,1,opt,name=num,proto3" json:"num,omitempty"`
}

func (x *TestServerStreamResponse) Reset() {
	*x = TestServerStreamResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tests_proto_test_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TestServerStreamResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TestServerStreamResponse) ProtoMessage() {}

func (x *TestServerStreamResponse) ProtoReflect() protoreflect.Message {
	mi := &file_tests_proto_test_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TestServerStreamResponse.ProtoReflect.Descriptor instead.
func (*TestServerStreamResponse) Descriptor() ([]byte, []int) {
	return file_tests_proto_test_proto_rawDescGZIP(), []int{5}
}

func (x *TestServerStreamResponse) GetNum() uint64 {
	if x != nil {
		return x.Num
	}
	return 0
}

type TestBiStreamRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Start     uint64 `protobuf:"varint,1,opt,name=start,proto3" json:"start,omitempty"`
	End       uint64 `protobuf:"varint,2,opt,name=end,proto3" json:"end,omitempty"`
	Subscribe *bool  `protobuf:"varint,3,opt,name=subscribe,proto3,oneof" json:"subscribe,omitempty"`
}

func (x *TestBiStreamRequest) Reset() {
	*x = TestBiStreamRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tests_proto_test_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TestBiStreamRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TestBiStreamRequest) ProtoMessage() {}

func (x *TestBiStreamRequest) ProtoReflect() protoreflect.Message {
	mi := &file_tests_proto_test_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TestBiStreamRequest.ProtoReflect.Descriptor instead.
func (*TestBiStreamRequest) Descriptor() ([]byte, []int) {
	return file_tests_proto_test_proto_rawDescGZIP(), []int{6}
}

func (x *TestBiStreamRequest) GetStart() uint64 {
	if x != nil {
		return x.Start
	}
	return 0
}

func (x *TestBiStreamRequest) GetEnd() uint64 {
	if x != nil {
		return x.End
	}
	return 0
}

func (x *TestBiStreamRequest) GetSubscribe() bool {
	if x != nil && x.Subscribe != nil {
		return *x.Subscribe
	}
	return false
}

type TestBiStreamResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Prices []float64 `protobuf:"fixed64,1,rep,packed,name=prices,proto3" json:"prices,omitempty"`
	Start  uint64    `protobuf:"varint,2,opt,name=start,proto3" json:"start,omitempty"`
	End    uint64    `protobuf:"varint,3,opt,name=end,proto3" json:"end,omitempty"`
	Error  *string   `protobuf:"bytes,4,opt,name=error,proto3,oneof" json:"error,omitempty"`
}

func (x *TestBiStreamResponse) Reset() {
	*x = TestBiStreamResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_tests_proto_test_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TestBiStreamResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TestBiStreamResponse) ProtoMessage() {}

func (x *TestBiStreamResponse) ProtoReflect() protoreflect.Message {
	mi := &file_tests_proto_test_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TestBiStreamResponse.ProtoReflect.Descriptor instead.
func (*TestBiStreamResponse) Descriptor() ([]byte, []int) {
	return file_tests_proto_test_proto_rawDescGZIP(), []int{7}
}

func (x *TestBiStreamResponse) GetPrices() []float64 {
	if x != nil {
		return x.Prices
	}
	return nil
}

func (x *TestBiStreamResponse) GetStart() uint64 {
	if x != nil {
		return x.Start
	}
	return 0
}

func (x *TestBiStreamResponse) GetEnd() uint64 {
	if x != nil {
		return x.End
	}
	return 0
}

func (x *TestBiStreamResponse) GetError() string {
	if x != nil && x.Error != nil {
		return *x.Error
	}
	return ""
}

var File_tests_proto_test_proto protoreflect.FileDescriptor

var file_tests_proto_test_proto_rawDesc = []byte{
	0x0a, 0x16, 0x74, 0x65, 0x73, 0x74, 0x73, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x74, 0x65,
	0x73, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x04, 0x74, 0x65, 0x73, 0x74, 0x22, 0x23,
	0x0a, 0x0b, 0x54, 0x65, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x14, 0x0a,
	0x05, 0x6c, 0x6f, 0x77, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x6c, 0x6f,
	0x77, 0x65, 0x72, 0x22, 0x24, 0x0a, 0x0c, 0x54, 0x65, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x75, 0x70, 0x70, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x05, 0x75, 0x70, 0x70, 0x65, 0x72, 0x22, 0x2b, 0x0a, 0x17, 0x54, 0x65, 0x73,
	0x74, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x6e, 0x75, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x04, 0x52, 0x03, 0x6e, 0x75, 0x6d, 0x22, 0x2c, 0x0a, 0x18, 0x54, 0x65, 0x73, 0x74, 0x43, 0x6c,
	0x69, 0x65, 0x6e, 0x74, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x73, 0x75, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52,
	0x03, 0x73, 0x75, 0x6d, 0x22, 0x32, 0x0a, 0x17, 0x54, 0x65, 0x73, 0x74, 0x53, 0x65, 0x72, 0x76,
	0x65, 0x72, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x17, 0x0a, 0x07, 0x66, 0x69, 0x62, 0x5f, 0x6e, 0x75, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d,
	0x52, 0x06, 0x66, 0x69, 0x62, 0x4e, 0x75, 0x6d, 0x22, 0x2c, 0x0a, 0x18, 0x54, 0x65, 0x73, 0x74,
	0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x6e, 0x75, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x04, 0x52, 0x03, 0x6e, 0x75, 0x6d, 0x22, 0x6e, 0x0a, 0x13, 0x54, 0x65, 0x73, 0x74, 0x42, 0x69,
	0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x14, 0x0a,
	0x05, 0x73, 0x74, 0x61, 0x72, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x73, 0x74,
	0x61, 0x72, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x65, 0x6e, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x03, 0x65, 0x6e, 0x64, 0x12, 0x21, 0x0a, 0x09, 0x73, 0x75, 0x62, 0x73, 0x63, 0x72, 0x69,
	0x62, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x48, 0x00, 0x52, 0x09, 0x73, 0x75, 0x62, 0x73,
	0x63, 0x72, 0x69, 0x62, 0x65, 0x88, 0x01, 0x01, 0x42, 0x0c, 0x0a, 0x0a, 0x5f, 0x73, 0x75, 0x62,
	0x73, 0x63, 0x72, 0x69, 0x62, 0x65, 0x22, 0x7b, 0x0a, 0x14, 0x54, 0x65, 0x73, 0x74, 0x42, 0x69,
	0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x16,
	0x0a, 0x06, 0x70, 0x72, 0x69, 0x63, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x01, 0x52, 0x06,
	0x70, 0x72, 0x69, 0x63, 0x65, 0x73, 0x12, 0x14, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x72, 0x74, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x73, 0x74, 0x61, 0x72, 0x74, 0x12, 0x10, 0x0a, 0x03,
	0x65, 0x6e, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x03, 0x65, 0x6e, 0x64, 0x12, 0x19,
	0x0a, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52,
	0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x88, 0x01, 0x01, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x65, 0x72,
	0x72, 0x6f, 0x72, 0x32, 0x91, 0x04, 0x0a, 0x07, 0x54, 0x65, 0x73, 0x74, 0x41, 0x50, 0x49, 0x12,
	0x2d, 0x0a, 0x04, 0x54, 0x65, 0x73, 0x74, 0x12, 0x11, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x54,
	0x65, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x12, 0x2e, 0x74, 0x65, 0x73,
	0x74, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x53,
	0x0a, 0x10, 0x54, 0x65, 0x73, 0x74, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x53, 0x74, 0x72, 0x65,
	0x61, 0x6d, 0x12, 0x1d, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x43, 0x6c,
	0x69, 0x65, 0x6e, 0x74, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x1e, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x43, 0x6c, 0x69,
	0x65, 0x6e, 0x74, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x28, 0x01, 0x12, 0x53, 0x0a, 0x10, 0x54, 0x65, 0x73, 0x74, 0x53, 0x65, 0x72, 0x76, 0x65,
	0x72, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x12, 0x1d, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x54,
	0x65, 0x73, 0x74, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1e, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x54, 0x65,
	0x73, 0x74, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x30, 0x01, 0x12, 0x49, 0x0a, 0x0c, 0x54, 0x65, 0x73, 0x74,
	0x42, 0x69, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x12, 0x19, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e,
	0x54, 0x65, 0x73, 0x74, 0x42, 0x69, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x1a, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x42,
	0x69, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x28,
	0x01, 0x30, 0x01, 0x12, 0x4c, 0x0a, 0x0f, 0x54, 0x65, 0x73, 0x74, 0x42, 0x69, 0x41, 0x62, 0x73,
	0x55, 0x70, 0x44, 0x6f, 0x77, 0x6e, 0x12, 0x19, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x54, 0x65,
	0x73, 0x74, 0x42, 0x69, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x1a, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x42, 0x69, 0x53,
	0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x28, 0x01, 0x30,
	0x01, 0x12, 0x4c, 0x0a, 0x0f, 0x54, 0x65, 0x73, 0x74, 0x42, 0x69, 0x50, 0x63, 0x74, 0x55, 0x70,
	0x44, 0x6f, 0x77, 0x6e, 0x12, 0x19, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x54, 0x65, 0x73, 0x74,
	0x42, 0x69, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x1a, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x42, 0x69, 0x53, 0x74, 0x72,
	0x65, 0x61, 0x6d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x28, 0x01, 0x30, 0x01, 0x12,
	0x46, 0x0a, 0x09, 0x54, 0x65, 0x73, 0x74, 0x42, 0x69, 0x50, 0x63, 0x74, 0x12, 0x19, 0x2e, 0x74,
	0x65, 0x73, 0x74, 0x2e, 0x54, 0x65, 0x73, 0x74, 0x42, 0x69, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1a, 0x2e, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x54,
	0x65, 0x73, 0x74, 0x42, 0x69, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x28, 0x01, 0x30, 0x01, 0x42, 0x2d, 0x5a, 0x2b, 0x67, 0x69, 0x74, 0x68, 0x75,
	0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6a, 0x6f, 0x68, 0x6e, 0x69, 0x65, 0x74, 0x72, 0x65, 0x2f,
	0x67, 0x72, 0x70, 0x63, 0x2d, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x2f, 0x74, 0x65, 0x73, 0x74, 0x73,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_tests_proto_test_proto_rawDescOnce sync.Once
	file_tests_proto_test_proto_rawDescData = file_tests_proto_test_proto_rawDesc
)

func file_tests_proto_test_proto_rawDescGZIP() []byte {
	file_tests_proto_test_proto_rawDescOnce.Do(func() {
		file_tests_proto_test_proto_rawDescData = protoimpl.X.CompressGZIP(file_tests_proto_test_proto_rawDescData)
	})
	return file_tests_proto_test_proto_rawDescData
}

var file_tests_proto_test_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_tests_proto_test_proto_goTypes = []interface{}{
	(*TestRequest)(nil),              // 0: test.TestRequest
	(*TestResponse)(nil),             // 1: test.TestResponse
	(*TestClientStreamRequest)(nil),  // 2: test.TestClientStreamRequest
	(*TestClientStreamResponse)(nil), // 3: test.TestClientStreamResponse
	(*TestServerStreamRequest)(nil),  // 4: test.TestServerStreamRequest
	(*TestServerStreamResponse)(nil), // 5: test.TestServerStreamResponse
	(*TestBiStreamRequest)(nil),      // 6: test.TestBiStreamRequest
	(*TestBiStreamResponse)(nil),     // 7: test.TestBiStreamResponse
}
var file_tests_proto_test_proto_depIdxs = []int32{
	0, // 0: test.TestAPI.Test:input_type -> test.TestRequest
	2, // 1: test.TestAPI.TestClientStream:input_type -> test.TestClientStreamRequest
	4, // 2: test.TestAPI.TestServerStream:input_type -> test.TestServerStreamRequest
	6, // 3: test.TestAPI.TestBiStream:input_type -> test.TestBiStreamRequest
	6, // 4: test.TestAPI.TestBiAbsUpDown:input_type -> test.TestBiStreamRequest
	6, // 5: test.TestAPI.TestBiPctUpDown:input_type -> test.TestBiStreamRequest
	6, // 6: test.TestAPI.TestBiPct:input_type -> test.TestBiStreamRequest
	1, // 7: test.TestAPI.Test:output_type -> test.TestResponse
	3, // 8: test.TestAPI.TestClientStream:output_type -> test.TestClientStreamResponse
	5, // 9: test.TestAPI.TestServerStream:output_type -> test.TestServerStreamResponse
	7, // 10: test.TestAPI.TestBiStream:output_type -> test.TestBiStreamResponse
	7, // 11: test.TestAPI.TestBiAbsUpDown:output_type -> test.TestBiStreamResponse
	7, // 12: test.TestAPI.TestBiPctUpDown:output_type -> test.TestBiStreamResponse
	7, // 13: test.TestAPI.TestBiPct:output_type -> test.TestBiStreamResponse
	7, // [7:14] is the sub-list for method output_type
	0, // [0:7] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_tests_proto_test_proto_init() }
func file_tests_proto_test_proto_init() {
	if File_tests_proto_test_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_tests_proto_test_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TestRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_tests_proto_test_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TestResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_tests_proto_test_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TestClientStreamRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_tests_proto_test_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TestClientStreamResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_tests_proto_test_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TestServerStreamRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_tests_proto_test_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TestServerStreamResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_tests_proto_test_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TestBiStreamRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_tests_proto_test_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TestBiStreamResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_tests_proto_test_proto_msgTypes[6].OneofWrappers = []interface{}{}
	file_tests_proto_test_proto_msgTypes[7].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_tests_proto_test_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_tests_proto_test_proto_goTypes,
		DependencyIndexes: file_tests_proto_test_proto_depIdxs,
		MessageInfos:      file_tests_proto_test_proto_msgTypes,
	}.Build()
	File_tests_proto_test_proto = out.File
	file_tests_proto_test_proto_rawDesc = nil
	file_tests_proto_test_proto_goTypes = nil
	file_tests_proto_test_proto_depIdxs = nil
}
