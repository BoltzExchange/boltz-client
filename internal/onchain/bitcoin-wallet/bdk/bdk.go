package bdk

// #include <bdk.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync/atomic"
	"unsafe"
)

// This is needed, because as of go 1.24
// type RustBuffer C.RustBuffer cannot have methods,
// RustBuffer is treated as non-local type
type GoRustBuffer struct {
	inner C.RustBuffer
}

type RustBufferI interface {
	AsReader() *bytes.Reader
	Free()
	ToGoBytes() []byte
	Data() unsafe.Pointer
	Len() uint64
	Capacity() uint64
}

func RustBufferFromExternal(b RustBufferI) GoRustBuffer {
	return GoRustBuffer{
		inner: C.RustBuffer{
			capacity: C.uint64_t(b.Capacity()),
			len:      C.uint64_t(b.Len()),
			data:     (*C.uchar)(b.Data()),
		},
	}
}

func (cb GoRustBuffer) Capacity() uint64 {
	return uint64(cb.inner.capacity)
}

func (cb GoRustBuffer) Len() uint64 {
	return uint64(cb.inner.len)
}

func (cb GoRustBuffer) Data() unsafe.Pointer {
	return unsafe.Pointer(cb.inner.data)
}

func (cb GoRustBuffer) AsReader() *bytes.Reader {
	b := unsafe.Slice((*byte)(cb.inner.data), C.uint64_t(cb.inner.len))
	return bytes.NewReader(b)
}

func (cb GoRustBuffer) Free() {
	rustCall(func(status *C.RustCallStatus) bool {
		C.ffi_bdk_rustbuffer_free(cb.inner, status)
		return false
	})
}

func (cb GoRustBuffer) ToGoBytes() []byte {
	return C.GoBytes(unsafe.Pointer(cb.inner.data), C.int(cb.inner.len))
}

func stringToRustBuffer(str string) C.RustBuffer {
	return bytesToRustBuffer([]byte(str))
}

func bytesToRustBuffer(b []byte) C.RustBuffer {
	if len(b) == 0 {
		return C.RustBuffer{}
	}
	// We can pass the pointer along here, as it is pinned
	// for the duration of this call
	foreign := C.ForeignBytes{
		len:  C.int(len(b)),
		data: (*C.uchar)(unsafe.Pointer(&b[0])),
	}

	return rustCall(func(status *C.RustCallStatus) C.RustBuffer {
		return C.ffi_bdk_rustbuffer_from_bytes(foreign, status)
	})
}

type BufLifter[GoType any] interface {
	Lift(value RustBufferI) GoType
}

type BufLowerer[GoType any] interface {
	Lower(value GoType) C.RustBuffer
}

type BufReader[GoType any] interface {
	Read(reader io.Reader) GoType
}

type BufWriter[GoType any] interface {
	Write(writer io.Writer, value GoType)
}

func LowerIntoRustBuffer[GoType any](bufWriter BufWriter[GoType], value GoType) C.RustBuffer {
	// This might be not the most efficient way but it does not require knowing allocation size
	// beforehand
	var buffer bytes.Buffer
	bufWriter.Write(&buffer, value)

	bytes, err := io.ReadAll(&buffer)
	if err != nil {
		panic(fmt.Errorf("reading written data: %w", err))
	}
	return bytesToRustBuffer(bytes)
}

func LiftFromRustBuffer[GoType any](bufReader BufReader[GoType], rbuf RustBufferI) GoType {
	defer rbuf.Free()
	reader := rbuf.AsReader()
	item := bufReader.Read(reader)
	if reader.Len() > 0 {
		// TODO: Remove this
		leftover, _ := io.ReadAll(reader)
		panic(fmt.Errorf("Junk remaining in buffer after lifting: %s", string(leftover)))
	}
	return item
}

func rustCallWithError[E any, U any](converter BufReader[*E], callback func(*C.RustCallStatus) U) (U, *E) {
	var status C.RustCallStatus
	returnValue := callback(&status)
	err := checkCallStatus(converter, status)
	return returnValue, err
}

func checkCallStatus[E any](converter BufReader[*E], status C.RustCallStatus) *E {
	switch status.code {
	case 0:
		return nil
	case 1:
		return LiftFromRustBuffer(converter, GoRustBuffer{inner: status.errorBuf})
	case 2:
		// when the rust code sees a panic, it tries to construct a rustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{inner: status.errorBuf})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		panic(fmt.Errorf("unknown status code: %d", status.code))
	}
}

func checkCallStatusUnknown(status C.RustCallStatus) error {
	switch status.code {
	case 0:
		return nil
	case 1:
		panic(fmt.Errorf("function not returning an error returned an error"))
	case 2:
		// when the rust code sees a panic, it tries to construct a C.RustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{
				inner: status.errorBuf,
			})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		return fmt.Errorf("unknown status code: %d", status.code)
	}
}

func rustCall[U any](callback func(*C.RustCallStatus) U) U {
	returnValue, err := rustCallWithError[error](nil, callback)
	if err != nil {
		panic(err)
	}
	return returnValue
}

type NativeError interface {
	AsError() error
}

func writeInt8(writer io.Writer, value int8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint8(writer io.Writer, value uint8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt16(writer io.Writer, value int16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint16(writer io.Writer, value uint16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt32(writer io.Writer, value int32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint32(writer io.Writer, value uint32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt64(writer io.Writer, value int64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint64(writer io.Writer, value uint64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat32(writer io.Writer, value float32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat64(writer io.Writer, value float64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func readInt8(reader io.Reader) int8 {
	var result int8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint8(reader io.Reader) uint8 {
	var result uint8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt16(reader io.Reader) int16 {
	var result int16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint16(reader io.Reader) uint16 {
	var result uint16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt32(reader io.Reader) int32 {
	var result int32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint32(reader io.Reader) uint32 {
	var result uint32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt64(reader io.Reader) int64 {
	var result int64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint64(reader io.Reader) uint64 {
	var result uint64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat32(reader io.Reader) float32 {
	var result float32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat64(reader io.Reader) float64 {
	var result float64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func init() {

	uniffiCheckChecksums()
}

func uniffiCheckChecksums() {
	// Get the bindings contract version from our ComponentInterface
	bindingsContractVersion := 26
	// Get the scaffolding contract version by calling the into the dylib
	scaffoldingContractVersion := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.ffi_bdk_uniffi_contract_version()
	})
	if bindingsContractVersion != int(scaffoldingContractVersion) {
		// If this happens try cleaning and rebuilding your project
		panic("bdk: UniFFI contract version mismatch")
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bdk_checksum_func_derive_default_xpub()
		})
		if checksum != 18183 {
			// If this happens try cleaning and rebuilding your project
			panic("bdk: uniffi_bdk_checksum_func_derive_default_xpub: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bdk_checksum_method_wallet_apply_transaction()
		})
		if checksum != 55080 {
			// If this happens try cleaning and rebuilding your project
			panic("bdk: uniffi_bdk_checksum_method_wallet_apply_transaction: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bdk_checksum_method_wallet_balance()
		})
		if checksum != 64388 {
			// If this happens try cleaning and rebuilding your project
			panic("bdk: uniffi_bdk_checksum_method_wallet_balance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bdk_checksum_method_wallet_bump_transaction_fee()
		})
		if checksum != 16810 {
			// If this happens try cleaning and rebuilding your project
			panic("bdk: uniffi_bdk_checksum_method_wallet_bump_transaction_fee: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bdk_checksum_method_wallet_get_transactions()
		})
		if checksum != 56003 {
			// If this happens try cleaning and rebuilding your project
			panic("bdk: uniffi_bdk_checksum_method_wallet_get_transactions: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bdk_checksum_method_wallet_new_address()
		})
		if checksum != 47290 {
			// If this happens try cleaning and rebuilding your project
			panic("bdk: uniffi_bdk_checksum_method_wallet_new_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bdk_checksum_method_wallet_send_to_address()
		})
		if checksum != 10893 {
			// If this happens try cleaning and rebuilding your project
			panic("bdk: uniffi_bdk_checksum_method_wallet_send_to_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bdk_checksum_method_wallet_sync()
		})
		if checksum != 38629 {
			// If this happens try cleaning and rebuilding your project
			panic("bdk: uniffi_bdk_checksum_method_wallet_sync: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bdk_checksum_constructor_backend_new()
		})
		if checksum != 42387 {
			// If this happens try cleaning and rebuilding your project
			panic("bdk: uniffi_bdk_checksum_constructor_backend_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bdk_checksum_constructor_wallet_new()
		})
		if checksum != 57184 {
			// If this happens try cleaning and rebuilding your project
			panic("bdk: uniffi_bdk_checksum_constructor_wallet_new: UniFFI API checksum mismatch")
		}
	}
}

type FfiConverterUint32 struct{}

var FfiConverterUint32INSTANCE = FfiConverterUint32{}

func (FfiConverterUint32) Lower(value uint32) C.uint32_t {
	return C.uint32_t(value)
}

func (FfiConverterUint32) Write(writer io.Writer, value uint32) {
	writeUint32(writer, value)
}

func (FfiConverterUint32) Lift(value C.uint32_t) uint32 {
	return uint32(value)
}

func (FfiConverterUint32) Read(reader io.Reader) uint32 {
	return readUint32(reader)
}

type FfiDestroyerUint32 struct{}

func (FfiDestroyerUint32) Destroy(_ uint32) {}

type FfiConverterUint64 struct{}

var FfiConverterUint64INSTANCE = FfiConverterUint64{}

func (FfiConverterUint64) Lower(value uint64) C.uint64_t {
	return C.uint64_t(value)
}

func (FfiConverterUint64) Write(writer io.Writer, value uint64) {
	writeUint64(writer, value)
}

func (FfiConverterUint64) Lift(value C.uint64_t) uint64 {
	return uint64(value)
}

func (FfiConverterUint64) Read(reader io.Reader) uint64 {
	return readUint64(reader)
}

type FfiDestroyerUint64 struct{}

func (FfiDestroyerUint64) Destroy(_ uint64) {}

type FfiConverterInt64 struct{}

var FfiConverterInt64INSTANCE = FfiConverterInt64{}

func (FfiConverterInt64) Lower(value int64) C.int64_t {
	return C.int64_t(value)
}

func (FfiConverterInt64) Write(writer io.Writer, value int64) {
	writeInt64(writer, value)
}

func (FfiConverterInt64) Lift(value C.int64_t) int64 {
	return int64(value)
}

func (FfiConverterInt64) Read(reader io.Reader) int64 {
	return readInt64(reader)
}

type FfiDestroyerInt64 struct{}

func (FfiDestroyerInt64) Destroy(_ int64) {}

type FfiConverterFloat64 struct{}

var FfiConverterFloat64INSTANCE = FfiConverterFloat64{}

func (FfiConverterFloat64) Lower(value float64) C.double {
	return C.double(value)
}

func (FfiConverterFloat64) Write(writer io.Writer, value float64) {
	writeFloat64(writer, value)
}

func (FfiConverterFloat64) Lift(value C.double) float64 {
	return float64(value)
}

func (FfiConverterFloat64) Read(reader io.Reader) float64 {
	return readFloat64(reader)
}

type FfiDestroyerFloat64 struct{}

func (FfiDestroyerFloat64) Destroy(_ float64) {}

type FfiConverterBool struct{}

var FfiConverterBoolINSTANCE = FfiConverterBool{}

func (FfiConverterBool) Lower(value bool) C.int8_t {
	if value {
		return C.int8_t(1)
	}
	return C.int8_t(0)
}

func (FfiConverterBool) Write(writer io.Writer, value bool) {
	if value {
		writeInt8(writer, 1)
	} else {
		writeInt8(writer, 0)
	}
}

func (FfiConverterBool) Lift(value C.int8_t) bool {
	return value != 0
}

func (FfiConverterBool) Read(reader io.Reader) bool {
	return readInt8(reader) != 0
}

type FfiDestroyerBool struct{}

func (FfiDestroyerBool) Destroy(_ bool) {}

type FfiConverterString struct{}

var FfiConverterStringINSTANCE = FfiConverterString{}

func (FfiConverterString) Lift(rb RustBufferI) string {
	defer rb.Free()
	reader := rb.AsReader()
	b, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("reading reader: %w", err))
	}
	return string(b)
}

func (FfiConverterString) Read(reader io.Reader) string {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading string, expected %d, read %d", length, read_length))
	}
	return string(buffer)
}

func (FfiConverterString) Lower(value string) C.RustBuffer {
	return stringToRustBuffer(value)
}

func (FfiConverterString) Write(writer io.Writer, value string) {
	if len(value) > math.MaxInt32 {
		panic("String is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := io.WriteString(writer, value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing string, expected %d, written %d", len(value), write_length))
	}
}

type FfiDestroyerString struct{}

func (FfiDestroyerString) Destroy(_ string) {}

// Below is an implementation of synchronization requirements outlined in the link.
// https://github.com/mozilla/uniffi-rs/blob/0dc031132d9493ca812c3af6e7dd60ad2ea95bf0/uniffi_bindgen/src/bindings/kotlin/templates/ObjectRuntime.kt#L31

type FfiObject struct {
	pointer       unsafe.Pointer
	callCounter   atomic.Int64
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer
	freeFunction  func(unsafe.Pointer, *C.RustCallStatus)
	destroyed     atomic.Bool
}

func newFfiObject(
	pointer unsafe.Pointer,
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer,
	freeFunction func(unsafe.Pointer, *C.RustCallStatus),
) FfiObject {
	return FfiObject{
		pointer:       pointer,
		cloneFunction: cloneFunction,
		freeFunction:  freeFunction,
	}
}

func (ffiObject *FfiObject) incrementPointer(debugName string) unsafe.Pointer {
	for {
		counter := ffiObject.callCounter.Load()
		if counter <= -1 {
			panic(fmt.Errorf("%v object has already been destroyed", debugName))
		}
		if counter == math.MaxInt64 {
			panic(fmt.Errorf("%v object call counter would overflow", debugName))
		}
		if ffiObject.callCounter.CompareAndSwap(counter, counter+1) {
			break
		}
	}

	return rustCall(func(status *C.RustCallStatus) unsafe.Pointer {
		return ffiObject.cloneFunction(ffiObject.pointer, status)
	})
}

func (ffiObject *FfiObject) decrementPointer() {
	if ffiObject.callCounter.Add(-1) == -1 {
		ffiObject.freeRustArcPtr()
	}
}

func (ffiObject *FfiObject) destroy() {
	if ffiObject.destroyed.CompareAndSwap(false, true) {
		if ffiObject.callCounter.Add(-1) == -1 {
			ffiObject.freeRustArcPtr()
		}
	}
}

func (ffiObject *FfiObject) freeRustArcPtr() {
	rustCall(func(status *C.RustCallStatus) int32 {
		ffiObject.freeFunction(ffiObject.pointer, status)
		return 0
	})
}

type BackendInterface interface {
}
type Backend struct {
	ffiObject FfiObject
}

func NewBackend(network Network, electrumUrl string) (*Backend, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_bdk_fn_constructor_backend_new(FfiConverterNetworkINSTANCE.Lower(network), FfiConverterStringINSTANCE.Lower(electrumUrl), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Backend
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBackendINSTANCE.Lift(_uniffiRV), nil
	}
}

func (object *Backend) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterBackend struct{}

var FfiConverterBackendINSTANCE = FfiConverterBackend{}

func (c FfiConverterBackend) Lift(pointer unsafe.Pointer) *Backend {
	result := &Backend{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_bdk_fn_clone_backend(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_bdk_fn_free_backend(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Backend).Destroy)
	return result
}

func (c FfiConverterBackend) Read(reader io.Reader) *Backend {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterBackend) Lower(value *Backend) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Backend")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterBackend) Write(writer io.Writer, value *Backend) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerBackend struct{}

func (_ FfiDestroyerBackend) Destroy(value *Backend) {
	value.Destroy()
}

type WalletInterface interface {
	ApplyTransaction(txHex string) error
	Balance() (Balance, error)
	BumpTransactionFee(txId string, satPerVbyte float64) (string, error)
	GetTransactions(limit uint64, offset uint64) ([]WalletTransaction, error)
	NewAddress() (string, error)
	SendToAddress(address string, amount uint64, satPerVbyte float64, sendAll bool) (WalletSendResult, error)
	Sync() error
}
type Wallet struct {
	ffiObject FfiObject
}

func NewWallet(backend *Backend, credentials WalletCredentials, dbPath string) (*Wallet, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_bdk_fn_constructor_wallet_new(FfiConverterBackendINSTANCE.Lower(backend), FfiConverterWalletCredentialsINSTANCE.Lower(credentials), FfiConverterStringINSTANCE.Lower(dbPath), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Wallet
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWalletINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) ApplyTransaction(txHex string) error {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_bdk_fn_method_wallet_apply_transaction(
			_pointer, FfiConverterStringINSTANCE.Lower(txHex), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *Wallet) Balance() (Balance, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bdk_fn_method_wallet_balance(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue Balance
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBalanceINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) BumpTransactionFee(txId string, satPerVbyte float64) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bdk_fn_method_wallet_bump_transaction_fee(
				_pointer, FfiConverterStringINSTANCE.Lower(txId), FfiConverterFloat64INSTANCE.Lower(satPerVbyte), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) GetTransactions(limit uint64, offset uint64) ([]WalletTransaction, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bdk_fn_method_wallet_get_transactions(
				_pointer, FfiConverterUint64INSTANCE.Lower(limit), FfiConverterUint64INSTANCE.Lower(offset), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []WalletTransaction
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceWalletTransactionINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) NewAddress() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bdk_fn_method_wallet_new_address(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) SendToAddress(address string, amount uint64, satPerVbyte float64, sendAll bool) (WalletSendResult, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bdk_fn_method_wallet_send_to_address(
				_pointer, FfiConverterStringINSTANCE.Lower(address), FfiConverterUint64INSTANCE.Lower(amount), FfiConverterFloat64INSTANCE.Lower(satPerVbyte), FfiConverterBoolINSTANCE.Lower(sendAll), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue WalletSendResult
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWalletSendResultINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) Sync() error {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_bdk_fn_method_wallet_sync(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}
func (object *Wallet) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterWallet struct{}

var FfiConverterWalletINSTANCE = FfiConverterWallet{}

func (c FfiConverterWallet) Lift(pointer unsafe.Pointer) *Wallet {
	result := &Wallet{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_bdk_fn_clone_wallet(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_bdk_fn_free_wallet(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Wallet).Destroy)
	return result
}

func (c FfiConverterWallet) Read(reader io.Reader) *Wallet {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterWallet) Lower(value *Wallet) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Wallet")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterWallet) Write(writer io.Writer, value *Wallet) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerWallet struct{}

func (_ FfiDestroyerWallet) Destroy(value *Wallet) {
	value.Destroy()
}

type Balance struct {
	Confirmed   uint64
	Unconfirmed uint64
}

func (r *Balance) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Confirmed)
	FfiDestroyerUint64{}.Destroy(r.Unconfirmed)
}

type FfiConverterBalance struct{}

var FfiConverterBalanceINSTANCE = FfiConverterBalance{}

func (c FfiConverterBalance) Lift(rb RustBufferI) Balance {
	return LiftFromRustBuffer[Balance](c, rb)
}

func (c FfiConverterBalance) Read(reader io.Reader) Balance {
	return Balance{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterBalance) Lower(value Balance) C.RustBuffer {
	return LowerIntoRustBuffer[Balance](c, value)
}

func (c FfiConverterBalance) Write(writer io.Writer, value Balance) {
	FfiConverterUint64INSTANCE.Write(writer, value.Confirmed)
	FfiConverterUint64INSTANCE.Write(writer, value.Unconfirmed)
}

type FfiDestroyerBalance struct{}

func (_ FfiDestroyerBalance) Destroy(value Balance) {
	value.Destroy()
}

type WalletCredentials struct {
	Mnemonic       *string
	CoreDescriptor string
}

func (r *WalletCredentials) Destroy() {
	FfiDestroyerOptionalString{}.Destroy(r.Mnemonic)
	FfiDestroyerString{}.Destroy(r.CoreDescriptor)
}

type FfiConverterWalletCredentials struct{}

var FfiConverterWalletCredentialsINSTANCE = FfiConverterWalletCredentials{}

func (c FfiConverterWalletCredentials) Lift(rb RustBufferI) WalletCredentials {
	return LiftFromRustBuffer[WalletCredentials](c, rb)
}

func (c FfiConverterWalletCredentials) Read(reader io.Reader) WalletCredentials {
	return WalletCredentials{
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterWalletCredentials) Lower(value WalletCredentials) C.RustBuffer {
	return LowerIntoRustBuffer[WalletCredentials](c, value)
}

func (c FfiConverterWalletCredentials) Write(writer io.Writer, value WalletCredentials) {
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Mnemonic)
	FfiConverterStringINSTANCE.Write(writer, value.CoreDescriptor)
}

type FfiDestroyerWalletCredentials struct{}

func (_ FfiDestroyerWalletCredentials) Destroy(value WalletCredentials) {
	value.Destroy()
}

type WalletSendResult struct {
	TxHex      string
	Fee        uint64
	SendAmount uint64
}

func (r *WalletSendResult) Destroy() {
	FfiDestroyerString{}.Destroy(r.TxHex)
	FfiDestroyerUint64{}.Destroy(r.Fee)
	FfiDestroyerUint64{}.Destroy(r.SendAmount)
}

type FfiConverterWalletSendResult struct{}

var FfiConverterWalletSendResultINSTANCE = FfiConverterWalletSendResult{}

func (c FfiConverterWalletSendResult) Lift(rb RustBufferI) WalletSendResult {
	return LiftFromRustBuffer[WalletSendResult](c, rb)
}

func (c FfiConverterWalletSendResult) Read(reader io.Reader) WalletSendResult {
	return WalletSendResult{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterWalletSendResult) Lower(value WalletSendResult) C.RustBuffer {
	return LowerIntoRustBuffer[WalletSendResult](c, value)
}

func (c FfiConverterWalletSendResult) Write(writer io.Writer, value WalletSendResult) {
	FfiConverterStringINSTANCE.Write(writer, value.TxHex)
	FfiConverterUint64INSTANCE.Write(writer, value.Fee)
	FfiConverterUint64INSTANCE.Write(writer, value.SendAmount)
}

type FfiDestroyerWalletSendResult struct{}

func (_ FfiDestroyerWalletSendResult) Destroy(value WalletSendResult) {
	value.Destroy()
}

type WalletTransaction struct {
	Id              string
	Timestamp       uint64
	Outputs         []WalletTransactionOutput
	BlockHeight     uint32
	BalanceChange   int64
	IsConsolidation bool
}

func (r *WalletTransaction) Destroy() {
	FfiDestroyerString{}.Destroy(r.Id)
	FfiDestroyerUint64{}.Destroy(r.Timestamp)
	FfiDestroyerSequenceWalletTransactionOutput{}.Destroy(r.Outputs)
	FfiDestroyerUint32{}.Destroy(r.BlockHeight)
	FfiDestroyerInt64{}.Destroy(r.BalanceChange)
	FfiDestroyerBool{}.Destroy(r.IsConsolidation)
}

type FfiConverterWalletTransaction struct{}

var FfiConverterWalletTransactionINSTANCE = FfiConverterWalletTransaction{}

func (c FfiConverterWalletTransaction) Lift(rb RustBufferI) WalletTransaction {
	return LiftFromRustBuffer[WalletTransaction](c, rb)
}

func (c FfiConverterWalletTransaction) Read(reader io.Reader) WalletTransaction {
	return WalletTransaction{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterSequenceWalletTransactionOutputINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterInt64INSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterWalletTransaction) Lower(value WalletTransaction) C.RustBuffer {
	return LowerIntoRustBuffer[WalletTransaction](c, value)
}

func (c FfiConverterWalletTransaction) Write(writer io.Writer, value WalletTransaction) {
	FfiConverterStringINSTANCE.Write(writer, value.Id)
	FfiConverterUint64INSTANCE.Write(writer, value.Timestamp)
	FfiConverterSequenceWalletTransactionOutputINSTANCE.Write(writer, value.Outputs)
	FfiConverterUint32INSTANCE.Write(writer, value.BlockHeight)
	FfiConverterInt64INSTANCE.Write(writer, value.BalanceChange)
	FfiConverterBoolINSTANCE.Write(writer, value.IsConsolidation)
}

type FfiDestroyerWalletTransaction struct{}

func (_ FfiDestroyerWalletTransaction) Destroy(value WalletTransaction) {
	value.Destroy()
}

type WalletTransactionOutput struct {
	Address      string
	Amount       uint64
	IsOurAddress bool
}

func (r *WalletTransactionOutput) Destroy() {
	FfiDestroyerString{}.Destroy(r.Address)
	FfiDestroyerUint64{}.Destroy(r.Amount)
	FfiDestroyerBool{}.Destroy(r.IsOurAddress)
}

type FfiConverterWalletTransactionOutput struct{}

var FfiConverterWalletTransactionOutputINSTANCE = FfiConverterWalletTransactionOutput{}

func (c FfiConverterWalletTransactionOutput) Lift(rb RustBufferI) WalletTransactionOutput {
	return LiftFromRustBuffer[WalletTransactionOutput](c, rb)
}

func (c FfiConverterWalletTransactionOutput) Read(reader io.Reader) WalletTransactionOutput {
	return WalletTransactionOutput{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterWalletTransactionOutput) Lower(value WalletTransactionOutput) C.RustBuffer {
	return LowerIntoRustBuffer[WalletTransactionOutput](c, value)
}

func (c FfiConverterWalletTransactionOutput) Write(writer io.Writer, value WalletTransactionOutput) {
	FfiConverterStringINSTANCE.Write(writer, value.Address)
	FfiConverterUint64INSTANCE.Write(writer, value.Amount)
	FfiConverterBoolINSTANCE.Write(writer, value.IsOurAddress)
}

type FfiDestroyerWalletTransactionOutput struct{}

func (_ FfiDestroyerWalletTransactionOutput) Destroy(value WalletTransactionOutput) {
	value.Destroy()
}

type Error struct {
	err error
}

// Convience method to turn *Error into error
// Avoiding treating nil pointer as non nil error interface
func (err *Error) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err Error) Error() string {
	return fmt.Sprintf("Error: %s", err.err.Error())
}

func (err Error) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrErrorGeneric = fmt.Errorf("ErrorGeneric")

// Variant structs
type ErrorGeneric struct {
	Field0 string
}

func NewErrorGeneric(
	var0 string,
) *Error {
	return &Error{err: &ErrorGeneric{
		Field0: var0}}
}

func (e ErrorGeneric) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err ErrorGeneric) Error() string {
	return fmt.Sprint("Generic",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self ErrorGeneric) Is(target error) bool {
	return target == ErrErrorGeneric
}

type FfiConverterError struct{}

var FfiConverterErrorINSTANCE = FfiConverterError{}

func (c FfiConverterError) Lift(eb RustBufferI) *Error {
	return LiftFromRustBuffer[*Error](c, eb)
}

func (c FfiConverterError) Lower(value *Error) C.RustBuffer {
	return LowerIntoRustBuffer[*Error](c, value)
}

func (c FfiConverterError) Read(reader io.Reader) *Error {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &Error{&ErrorGeneric{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterError.Read()", errorID))
	}
}

func (c FfiConverterError) Write(writer io.Writer, value *Error) {
	switch variantValue := value.err.(type) {
	case *ErrorGeneric:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterError.Write", value))
	}
}

type FfiDestroyerError struct{}

func (_ FfiDestroyerError) Destroy(value *Error) {
	switch variantValue := value.err.(type) {
	case ErrorGeneric:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerError.Destroy", value))
	}
}

type Network uint

const (
	NetworkBitcoin Network = 1
	NetworkTestnet Network = 2
	NetworkRegtest Network = 3
	NetworkSignet  Network = 4
)

type FfiConverterNetwork struct{}

var FfiConverterNetworkINSTANCE = FfiConverterNetwork{}

func (c FfiConverterNetwork) Lift(rb RustBufferI) Network {
	return LiftFromRustBuffer[Network](c, rb)
}

func (c FfiConverterNetwork) Lower(value Network) C.RustBuffer {
	return LowerIntoRustBuffer[Network](c, value)
}
func (FfiConverterNetwork) Read(reader io.Reader) Network {
	id := readInt32(reader)
	return Network(id)
}

func (FfiConverterNetwork) Write(writer io.Writer, value Network) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerNetwork struct{}

func (_ FfiDestroyerNetwork) Destroy(value Network) {
}

type FfiConverterOptionalString struct{}

var FfiConverterOptionalStringINSTANCE = FfiConverterOptionalString{}

func (c FfiConverterOptionalString) Lift(rb RustBufferI) *string {
	return LiftFromRustBuffer[*string](c, rb)
}

func (_ FfiConverterOptionalString) Read(reader io.Reader) *string {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterStringINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalString) Lower(value *string) C.RustBuffer {
	return LowerIntoRustBuffer[*string](c, value)
}

func (_ FfiConverterOptionalString) Write(writer io.Writer, value *string) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalString struct{}

func (_ FfiDestroyerOptionalString) Destroy(value *string) {
	if value != nil {
		FfiDestroyerString{}.Destroy(*value)
	}
}

type FfiConverterSequenceWalletTransaction struct{}

var FfiConverterSequenceWalletTransactionINSTANCE = FfiConverterSequenceWalletTransaction{}

func (c FfiConverterSequenceWalletTransaction) Lift(rb RustBufferI) []WalletTransaction {
	return LiftFromRustBuffer[[]WalletTransaction](c, rb)
}

func (c FfiConverterSequenceWalletTransaction) Read(reader io.Reader) []WalletTransaction {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]WalletTransaction, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterWalletTransactionINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceWalletTransaction) Lower(value []WalletTransaction) C.RustBuffer {
	return LowerIntoRustBuffer[[]WalletTransaction](c, value)
}

func (c FfiConverterSequenceWalletTransaction) Write(writer io.Writer, value []WalletTransaction) {
	if len(value) > math.MaxInt32 {
		panic("[]WalletTransaction is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterWalletTransactionINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceWalletTransaction struct{}

func (FfiDestroyerSequenceWalletTransaction) Destroy(sequence []WalletTransaction) {
	for _, value := range sequence {
		FfiDestroyerWalletTransaction{}.Destroy(value)
	}
}

type FfiConverterSequenceWalletTransactionOutput struct{}

var FfiConverterSequenceWalletTransactionOutputINSTANCE = FfiConverterSequenceWalletTransactionOutput{}

func (c FfiConverterSequenceWalletTransactionOutput) Lift(rb RustBufferI) []WalletTransactionOutput {
	return LiftFromRustBuffer[[]WalletTransactionOutput](c, rb)
}

func (c FfiConverterSequenceWalletTransactionOutput) Read(reader io.Reader) []WalletTransactionOutput {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]WalletTransactionOutput, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterWalletTransactionOutputINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceWalletTransactionOutput) Lower(value []WalletTransactionOutput) C.RustBuffer {
	return LowerIntoRustBuffer[[]WalletTransactionOutput](c, value)
}

func (c FfiConverterSequenceWalletTransactionOutput) Write(writer io.Writer, value []WalletTransactionOutput) {
	if len(value) > math.MaxInt32 {
		panic("[]WalletTransactionOutput is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterWalletTransactionOutputINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceWalletTransactionOutput struct{}

func (FfiDestroyerSequenceWalletTransactionOutput) Destroy(sequence []WalletTransactionOutput) {
	for _, value := range sequence {
		FfiDestroyerWalletTransactionOutput{}.Destroy(value)
	}
}

func DeriveDefaultXpub(network Network, mnemonic string) (string, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bdk_fn_func_derive_default_xpub(FfiConverterNetworkINSTANCE.Lower(network), FfiConverterStringINSTANCE.Lower(mnemonic), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}
