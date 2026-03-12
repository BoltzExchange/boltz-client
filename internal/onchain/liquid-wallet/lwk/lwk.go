package lwk

// #include <lwk.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync"
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

// C.RustBuffer fields exposed as an interface so they can be accessed in different Go packages.
// See https://github.com/golang/go/issues/13467
type ExternalCRustBuffer interface {
	Data() unsafe.Pointer
	Len() uint64
	Capacity() uint64
}

func RustBufferFromC(b C.RustBuffer) ExternalCRustBuffer {
	return GoRustBuffer{
		inner: b,
	}
}

func CFromRustBuffer(b ExternalCRustBuffer) C.RustBuffer {
	return C.RustBuffer{
		capacity: C.uint64_t(b.Capacity()),
		len:      C.uint64_t(b.Len()),
		data:     (*C.uchar)(b.Data()),
	}
}

func RustBufferFromExternal(b ExternalCRustBuffer) GoRustBuffer {
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
		C.ffi_lwk_rustbuffer_free(cb.inner, status)
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
		return C.ffi_lwk_rustbuffer_from_bytes(foreign, status)
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

	FfiConverterForeignStoreINSTANCE.register()
	FfiConverterLoggingINSTANCE.register()
	uniffiCheckChecksums()
}

func uniffiCheckChecksums() {
	// Get the bindings contract version from our ComponentInterface
	bindingsContractVersion := 29
	// Get the scaffolding contract version by calling the into the dylib
	scaffoldingContractVersion := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.ffi_lwk_uniffi_contract_version()
	})
	if bindingsContractVersion != int(scaffoldingContractVersion) {
		// If this happens try cleaning and rebuilding your project
		panic("lwk: UniFFI contract version mismatch")
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_func_derive_asset_id()
		})
		if checksum != 63625 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_func_derive_asset_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_func_derive_token_id()
		})
		if checksum != 30312 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_func_derive_token_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_func_is_provably_segwit()
		})
		if checksum != 18100 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_func_is_provably_segwit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_address_is_blinded()
		})
		if checksum != 13572 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_address_is_blinded: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_address_network()
		})
		if checksum != 47140 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_address_network: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_address_qr_code_text()
		})
		if checksum != 34918 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_address_qr_code_text: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_address_qr_code_uri()
		})
		if checksum != 36127 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_address_qr_code_uri: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_address_script_pubkey()
		})
		if checksum != 29124 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_address_script_pubkey: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_address_to_unconfidential()
		})
		if checksum != 17427 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_address_to_unconfidential: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_addressresult_address()
		})
		if checksum != 40671 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_addressresult_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_addressresult_index()
		})
		if checksum != 11830 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_addressresult_index: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp0_address()
		})
		if checksum != 28332 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp0_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp0_amp_id()
		})
		if checksum != 48524 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp0_amp_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp0_last_index()
		})
		if checksum != 19251 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp0_last_index: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp0_sign()
		})
		if checksum != 4839 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp0_sign: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp0_wollet_descriptor()
		})
		if checksum != 39206 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp0_wollet_descriptor: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp0connected_get_challenge()
		})
		if checksum != 62572 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp0connected_get_challenge: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp0connected_login()
		})
		if checksum != 38625 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp0connected_login: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp0loggedin_create_amp0_account()
		})
		if checksum != 19376 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp0loggedin_create_amp0_account: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp0loggedin_create_watch_only()
		})
		if checksum != 18697 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp0loggedin_create_watch_only: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp0loggedin_get_amp_ids()
		})
		if checksum != 11011 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp0loggedin_get_amp_ids: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp0loggedin_next_account()
		})
		if checksum != 31592 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp0loggedin_next_account: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp0pset_blinding_nonces()
		})
		if checksum != 20239 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp0pset_blinding_nonces: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp0pset_pset()
		})
		if checksum != 41127 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp0pset_pset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp2_cosign()
		})
		if checksum != 5581 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp2_cosign: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp2_descriptor_from_str()
		})
		if checksum != 752 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp2_descriptor_from_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp2_register_wallet()
		})
		if checksum != 64376 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp2_register_wallet: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_amp2descriptor_descriptor()
		})
		if checksum != 61502 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp2descriptor_descriptor: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_assetamount_amount()
		})
		if checksum != 49734 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_assetamount_amount: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_assetamount_asset()
		})
		if checksum != 51371 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_assetamount_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_assetblindingfactor_to_bytes()
		})
		if checksum != 27225 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_assetblindingfactor_to_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip21_address()
		})
		if checksum != 22514 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip21_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip21_amount()
		})
		if checksum != 26979 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip21_amount: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip21_as_str()
		})
		if checksum != 64004 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip21_as_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip21_label()
		})
		if checksum != 62483 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip21_label: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip21_lightning()
		})
		if checksum != 36718 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip21_lightning: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip21_message()
		})
		if checksum != 55940 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip21_message: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip21_offer()
		})
		if checksum != 28927 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip21_offer: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip21_payjoin()
		})
		if checksum != 54940 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip21_payjoin: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip21_payjoin_output_substitution()
		})
		if checksum != 29383 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip21_payjoin_output_substitution: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip21_silent_payment_address()
		})
		if checksum != 63677 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip21_silent_payment_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip321_amount()
		})
		if checksum != 34467 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip321_amount: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip321_ark()
		})
		if checksum != 15652 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip321_ark: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip321_as_str()
		})
		if checksum != 63544 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip321_as_str: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip321_label()
		})
		if checksum != 53097 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip321_label: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip321_lightning()
		})
		if checksum != 47241 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip321_lightning: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip321_message()
		})
		if checksum != 41299 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip321_message: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip321_offer()
		})
		if checksum != 45377 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip321_offer: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip321_payjoin()
		})
		if checksum != 63459 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip321_payjoin: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip321_payjoin_output_substitution()
		})
		if checksum != 7095 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip321_payjoin_output_substitution: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bip321_silent_payment_address()
		})
		if checksum != 55189 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bip321_silent_payment_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bitcoinaddress_is_mainnet()
		})
		if checksum != 22224 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bitcoinaddress_is_mainnet: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_blockheader_block_hash()
		})
		if checksum != 22169 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_blockheader_block_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_blockheader_height()
		})
		if checksum != 58954 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_blockheader_height: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_blockheader_merkle_root()
		})
		if checksum != 53175 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_blockheader_merkle_root: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_blockheader_prev_blockhash()
		})
		if checksum != 46170 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_blockheader_prev_blockhash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_blockheader_time()
		})
		if checksum != 56056 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_blockheader_time: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_blockheader_version()
		})
		if checksum != 22115 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_blockheader_version: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bolt11invoice_amount_milli_satoshis()
		})
		if checksum != 5904 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bolt11invoice_amount_milli_satoshis: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bolt11invoice_expiry_time()
		})
		if checksum != 5862 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bolt11invoice_expiry_time: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bolt11invoice_invoice_description()
		})
		if checksum != 18758 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bolt11invoice_invoice_description: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bolt11invoice_min_final_cltv_expiry_delta()
		})
		if checksum != 6149 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bolt11invoice_min_final_cltv_expiry_delta: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bolt11invoice_network()
		})
		if checksum != 15186 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bolt11invoice_network: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bolt11invoice_payee_pub_key()
		})
		if checksum != 52337 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bolt11invoice_payee_pub_key: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bolt11invoice_payment_hash()
		})
		if checksum != 35521 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bolt11invoice_payment_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bolt11invoice_payment_secret()
		})
		if checksum != 58541 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bolt11invoice_payment_secret: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_bolt11invoice_timestamp()
		})
		if checksum != 21308 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_bolt11invoice_timestamp: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_btc_to_lbtc()
		})
		if checksum != 27295 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_btc_to_lbtc: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_completed_swap_ids()
		})
		if checksum != 32553 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_completed_swap_ids: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_fetch_swaps_info()
		})
		if checksum != 41140 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_fetch_swaps_info: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_get_swap_data()
		})
		if checksum != 56227 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_get_swap_data: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_invoice()
		})
		if checksum != 64828 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_invoice: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_lbtc_to_btc()
		})
		if checksum != 24979 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_lbtc_to_btc: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_next_index_to_use()
		})
		if checksum != 9036 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_next_index_to_use: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_pending_swap_ids()
		})
		if checksum != 39574 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_pending_swap_ids: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_prepare_pay()
		})
		if checksum != 58796 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_prepare_pay: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_quote()
		})
		if checksum != 25212 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_quote: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_quote_receive()
		})
		if checksum != 1911 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_quote_receive: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_refresh_swap_info()
		})
		if checksum != 42107 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_refresh_swap_info: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_remove_swap()
		})
		if checksum != 32895 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_remove_swap: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_rescue_file()
		})
		if checksum != 51537 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_rescue_file: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_restorable_btc_to_lbtc_swaps()
		})
		if checksum != 64015 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_restorable_btc_to_lbtc_swaps: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_restorable_lbtc_to_btc_swaps()
		})
		if checksum != 47519 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_restorable_lbtc_to_btc_swaps: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_restorable_reverse_swaps()
		})
		if checksum != 54384 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_restorable_reverse_swaps: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_restorable_submarine_swaps()
		})
		if checksum != 29803 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_restorable_submarine_swaps: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_restore_invoice()
		})
		if checksum != 56233 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_restore_invoice: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_restore_lockup()
		})
		if checksum != 29841 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_restore_lockup: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_restore_prepare_pay()
		})
		if checksum != 43475 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_restore_prepare_pay: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_set_next_index_to_use()
		})
		if checksum != 46243 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_set_next_index_to_use: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_boltzsession_swap_restore()
		})
		if checksum != 45430 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_boltzsession_swap_restore: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_currencycode_alpha3()
		})
		if checksum != 23143 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_currencycode_alpha3: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_currencycode_exp()
		})
		if checksum != 665 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_currencycode_exp: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_currencycode_name()
		})
		if checksum != 27063 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_currencycode_name: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_electrumclient_broadcast()
		})
		if checksum != 47006 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_electrumclient_broadcast: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_electrumclient_full_scan()
		})
		if checksum != 2842 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_electrumclient_full_scan: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_electrumclient_full_scan_to_index()
		})
		if checksum != 50918 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_electrumclient_full_scan_to_index: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_electrumclient_get_tx()
		})
		if checksum != 33161 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_electrumclient_get_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_electrumclient_ping()
		})
		if checksum != 58048 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_electrumclient_ping: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_electrumclient_tip()
		})
		if checksum != 29810 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_electrumclient_tip: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_esploraclient_broadcast()
		})
		if checksum != 2593 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_esploraclient_broadcast: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_esploraclient_full_scan()
		})
		if checksum != 50594 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_esploraclient_full_scan: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_esploraclient_full_scan_to_index()
		})
		if checksum != 5341 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_esploraclient_full_scan_to_index: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_esploraclient_tip()
		})
		if checksum != 31289 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_esploraclient_tip: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_foreignstore_get()
		})
		if checksum != 36300 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_foreignstore_get: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_foreignstore_put()
		})
		if checksum != 60620 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_foreignstore_put: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_foreignstore_remove()
		})
		if checksum != 51371 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_foreignstore_remove: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_invoiceresponse_advance()
		})
		if checksum != 28093 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_invoiceresponse_advance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_invoiceresponse_bolt11_invoice()
		})
		if checksum != 7912 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_invoiceresponse_bolt11_invoice: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_invoiceresponse_boltz_fee()
		})
		if checksum != 64005 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_invoiceresponse_boltz_fee: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_invoiceresponse_claim_txid()
		})
		if checksum != 30631 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_invoiceresponse_claim_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_invoiceresponse_complete_pay()
		})
		if checksum != 2434 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_invoiceresponse_complete_pay: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_invoiceresponse_fee()
		})
		if checksum != 49159 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_invoiceresponse_fee: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_invoiceresponse_serialize()
		})
		if checksum != 38841 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_invoiceresponse_serialize: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_invoiceresponse_swap_id()
		})
		if checksum != 63267 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_invoiceresponse_swap_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_asset()
		})
		if checksum != 3815 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_asset_satoshi()
		})
		if checksum != 4114 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_asset_satoshi: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_is_confidential()
		})
		if checksum != 53528 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_is_confidential: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_is_issuance()
		})
		if checksum != 43867 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_is_issuance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_is_null()
		})
		if checksum != 40661 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_is_null: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_is_reissuance()
		})
		if checksum != 28099 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_is_reissuance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_prev_txid()
		})
		if checksum != 52687 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_prev_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_prev_vout()
		})
		if checksum != 53282 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_prev_vout: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_token()
		})
		if checksum != 35389 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_token: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_token_satoshi()
		})
		if checksum != 60126 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_token_satoshi: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lightningpayment_bolt11_invoice()
		})
		if checksum != 41990 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lightningpayment_bolt11_invoice: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lockupresponse_advance()
		})
		if checksum != 46331 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lockupresponse_advance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lockupresponse_boltz_fee()
		})
		if checksum != 33301 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lockupresponse_boltz_fee: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lockupresponse_chain_from()
		})
		if checksum != 2081 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lockupresponse_chain_from: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lockupresponse_chain_to()
		})
		if checksum != 10065 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lockupresponse_chain_to: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lockupresponse_claim_txid()
		})
		if checksum != 18687 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lockupresponse_claim_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lockupresponse_complete()
		})
		if checksum != 62964 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lockupresponse_complete: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lockupresponse_expected_amount()
		})
		if checksum != 53174 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lockupresponse_expected_amount: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lockupresponse_fee()
		})
		if checksum != 33238 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lockupresponse_fee: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lockupresponse_lockup_address()
		})
		if checksum != 49127 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lockupresponse_lockup_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lockupresponse_lockup_txid()
		})
		if checksum != 41392 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lockupresponse_lockup_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lockupresponse_serialize()
		})
		if checksum != 43231 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lockupresponse_serialize: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lockupresponse_set_lockup_txid()
		})
		if checksum != 30868 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lockupresponse_set_lockup_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lockupresponse_swap_id()
		})
		if checksum != 36526 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lockupresponse_swap_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lockupresponse_uri()
		})
		if checksum != 38808 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lockupresponse_uri: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_logging_log()
		})
		if checksum != 50033 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_logging_log: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_electrum_url()
		})
		if checksum != 44900 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_electrum_url: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_esplora_url()
		})
		if checksum != 37949 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_esplora_url: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_generate()
		})
		if checksum != 57601 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_generate: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_genesis_block_hash()
		})
		if checksum != 32389 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_genesis_block_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_get_new_address()
		})
		if checksum != 18169 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_get_new_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_height()
		})
		if checksum != 19939 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_height: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_issue_asset()
		})
		if checksum != 64492 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_issue_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_send_to_address()
		})
		if checksum != 578 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_send_to_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_waterfalls_url()
		})
		if checksum != 39201 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_waterfalls_url: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwkteststore_read()
		})
		if checksum != 60137 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwkteststore_read: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwkteststore_remove()
		})
		if checksum != 61117 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwkteststore_remove: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwkteststore_write()
		})
		if checksum != 36350 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwkteststore_write: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_mnemonic_word_count()
		})
		if checksum != 61849 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_mnemonic_word_count: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_network_default_electrum_client()
		})
		if checksum != 38637 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_network_default_electrum_client: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_network_default_esplora_client()
		})
		if checksum != 60328 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_network_default_esplora_client: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_network_genesis_block_hash()
		})
		if checksum != 65413 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_network_genesis_block_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_network_is_mainnet()
		})
		if checksum != 10603 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_network_is_mainnet: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_network_policy_asset()
		})
		if checksum != 61911 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_network_policy_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_network_tx_builder()
		})
		if checksum != 8768 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_network_tx_builder: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_outpoint_txid()
		})
		if checksum != 58690 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_outpoint_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_outpoint_vout()
		})
		if checksum != 28332 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_outpoint_vout: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_payment_bip21()
		})
		if checksum != 43062 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_payment_bip21: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_payment_bip321()
		})
		if checksum != 33020 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_payment_bip321: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_payment_bip353()
		})
		if checksum != 36470 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_payment_bip353: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_payment_bitcoin_address()
		})
		if checksum != 29551 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_payment_bitcoin_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_payment_kind()
		})
		if checksum != 21742 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_payment_kind: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_payment_lightning_invoice()
		})
		if checksum != 59752 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_payment_lightning_invoice: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_payment_lightning_offer()
		})
		if checksum != 57440 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_payment_lightning_offer: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_payment_lightning_payment()
		})
		if checksum != 42254 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_payment_lightning_payment: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_payment_liquid_address()
		})
		if checksum != 15185 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_payment_liquid_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_payment_liquid_bip21()
		})
		if checksum != 62751 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_payment_liquid_bip21: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_payment_lnurl()
		})
		if checksum != 34751 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_payment_lnurl: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_posconfig_currency()
		})
		if checksum != 11088 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_posconfig_currency: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_posconfig_descriptor()
		})
		if checksum != 14982 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_posconfig_descriptor: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_posconfig_encode()
		})
		if checksum != 63461 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_posconfig_encode: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_posconfig_show_description()
		})
		if checksum != 34682 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_posconfig_show_description: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_posconfig_show_gear()
		})
		if checksum != 60085 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_posconfig_show_gear: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_precision_sats_to_string()
		})
		if checksum != 20274 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_precision_sats_to_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_precision_string_to_sats()
		})
		if checksum != 26556 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_precision_string_to_sats: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_preparepayresponse_advance()
		})
		if checksum != 45947 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_preparepayresponse_advance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_preparepayresponse_boltz_fee()
		})
		if checksum != 897 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_preparepayresponse_boltz_fee: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_preparepayresponse_complete_pay()
		})
		if checksum != 40255 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_preparepayresponse_complete_pay: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_preparepayresponse_fee()
		})
		if checksum != 46693 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_preparepayresponse_fee: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_preparepayresponse_lockup_txid()
		})
		if checksum != 24205 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_preparepayresponse_lockup_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_preparepayresponse_serialize()
		})
		if checksum != 33437 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_preparepayresponse_serialize: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_preparepayresponse_set_lockup_txid()
		})
		if checksum != 52767 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_preparepayresponse_set_lockup_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_preparepayresponse_swap_id()
		})
		if checksum != 47814 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_preparepayresponse_swap_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_preparepayresponse_uri()
		})
		if checksum != 64009 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_preparepayresponse_uri: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_preparepayresponse_uri_address()
		})
		if checksum != 16815 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_preparepayresponse_uri_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_preparepayresponse_uri_amount()
		})
		if checksum != 36278 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_preparepayresponse_uri_amount: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_pset_combine()
		})
		if checksum != 53457 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_pset_combine: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_pset_extract_tx()
		})
		if checksum != 18364 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_pset_extract_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_pset_finalize()
		})
		if checksum != 8805 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_pset_finalize: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_pset_inputs()
		})
		if checksum != 37869 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_pset_inputs: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_pset_outputs()
		})
		if checksum != 6211 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_pset_outputs: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_pset_unique_id()
		})
		if checksum != 39035 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_pset_unique_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetbalance_balances()
		})
		if checksum != 30248 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetbalance_balances: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetbalance_fee()
		})
		if checksum != 45919 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetbalance_fee: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetbalance_recipients()
		})
		if checksum != 28110 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetbalance_recipients: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetdetails_balance()
		})
		if checksum != 59666 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetdetails_balance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetdetails_fingerprints_has()
		})
		if checksum != 6688 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetdetails_fingerprints_has: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetdetails_fingerprints_missing()
		})
		if checksum != 9065 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetdetails_fingerprints_missing: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetdetails_inputs_issuances()
		})
		if checksum != 33153 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetdetails_inputs_issuances: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetdetails_signatures()
		})
		if checksum != 7984 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetdetails_signatures: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_issuance()
		})
		if checksum != 1619 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_issuance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_issuance_asset()
		})
		if checksum != 60376 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_issuance_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_issuance_ids()
		})
		if checksum != 43762 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_issuance_ids: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_issuance_token()
		})
		if checksum != 53699 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_issuance_token: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_previous_script_pubkey()
		})
		if checksum != 42922 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_previous_script_pubkey: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_previous_txid()
		})
		if checksum != 54624 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_previous_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_previous_vout()
		})
		if checksum != 49004 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_previous_vout: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_redeem_script()
		})
		if checksum != 62835 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_redeem_script: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_sighash()
		})
		if checksum != 46072 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_sighash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetoutput_amount()
		})
		if checksum != 6432 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetoutput_amount: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetoutput_asset()
		})
		if checksum != 5720 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetoutput_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetoutput_blinder_index()
		})
		if checksum != 25708 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetoutput_blinder_index: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetoutput_script_pubkey()
		})
		if checksum != 20311 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetoutput_script_pubkey: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetsignatures_has_signature()
		})
		if checksum != 62742 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetsignatures_has_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetsignatures_missing_signature()
		})
		if checksum != 6208 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetsignatures_missing_signature: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_quotebuilder_build()
		})
		if checksum != 23099 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_quotebuilder_build: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_quotebuilder_receive()
		})
		if checksum != 32479 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_quotebuilder_receive: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_quotebuilder_send()
		})
		if checksum != 41090 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_quotebuilder_send: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_recipient_address()
		})
		if checksum != 44409 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_recipient_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_recipient_asset()
		})
		if checksum != 23419 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_recipient_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_recipient_value()
		})
		if checksum != 39598 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_recipient_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_recipient_vout()
		})
		if checksum != 24321 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_recipient_vout: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_script_bytes()
		})
		if checksum != 35040 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_script_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_script_is_provably_unspendable()
		})
		if checksum != 12490 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_script_is_provably_unspendable: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_script_jet_sha256_hex()
		})
		if checksum != 5565 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_script_jet_sha256_hex: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_script_to_asm()
		})
		if checksum != 32896 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_script_to_asm: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_secretkey_bytes()
		})
		if checksum != 43476 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_secretkey_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_secretkey_sign()
		})
		if checksum != 47116 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_secretkey_sign: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_signer_amp0_account_xpub()
		})
		if checksum != 11093 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_signer_amp0_account_xpub: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_signer_amp0_sign_challenge()
		})
		if checksum != 31984 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_signer_amp0_sign_challenge: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_signer_amp0_signer_data()
		})
		if checksum != 14976 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_signer_amp0_signer_data: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_signer_derive_bip85_mnemonic()
		})
		if checksum != 32162 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_signer_derive_bip85_mnemonic: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_signer_fingerprint()
		})
		if checksum != 51686 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_signer_fingerprint: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_signer_keyorigin_xpub()
		})
		if checksum != 48213 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_signer_keyorigin_xpub: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_signer_mnemonic()
		})
		if checksum != 41786 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_signer_mnemonic: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_signer_sign()
		})
		if checksum != 38559 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_signer_sign: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_signer_singlesig_desc()
		})
		if checksum != 29930 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_signer_singlesig_desc: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_signer_wpkh_slip77_descriptor()
		})
		if checksum != 50399 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_signer_wpkh_slip77_descriptor: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_transaction_bytes()
		})
		if checksum != 5413 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_transaction_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_transaction_discount_vsize()
		})
		if checksum != 15950 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_transaction_discount_vsize: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_transaction_fee()
		})
		if checksum != 21760 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_transaction_fee: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_transaction_inputs()
		})
		if checksum != 47178 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_transaction_inputs: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_transaction_outputs()
		})
		if checksum != 45462 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_transaction_outputs: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_transaction_to_bytes()
		})
		if checksum != 64976 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_transaction_to_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_transaction_txid()
		})
		if checksum != 16242 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_transaction_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_transaction_verify_tx_amt_proofs()
		})
		if checksum != 46626 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_transaction_verify_tx_amt_proofs: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_add_burn()
		})
		if checksum != 9804 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_add_burn: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_add_explicit_recipient()
		})
		if checksum != 40242 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_add_explicit_recipient: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_add_external_utxos()
		})
		if checksum != 22348 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_add_external_utxos: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_add_input_rangeproofs()
		})
		if checksum != 13756 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_add_input_rangeproofs: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_add_lbtc_recipient()
		})
		if checksum != 895 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_add_lbtc_recipient: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_add_recipient()
		})
		if checksum != 56700 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_add_recipient: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_drain_lbtc_to()
		})
		if checksum != 34381 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_drain_lbtc_to: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_drain_lbtc_wallet()
		})
		if checksum != 46356 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_drain_lbtc_wallet: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_fee_rate()
		})
		if checksum != 26118 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_fee_rate: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_finish()
		})
		if checksum != 3994 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_finish: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_finish_for_amp0()
		})
		if checksum != 46241 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_finish_for_amp0: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_issue_asset()
		})
		if checksum != 48258 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_issue_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_liquidex_make()
		})
		if checksum != 30487 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_liquidex_make: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_liquidex_take()
		})
		if checksum != 7163 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_liquidex_take: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_reissue_asset()
		})
		if checksum != 28240 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_reissue_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_set_wallet_utxos()
		})
		if checksum != 53946 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_set_wallet_utxos: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txin_outpoint()
		})
		if checksum != 60750 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txin_outpoint: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txin_sequence()
		})
		if checksum != 13353 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txin_sequence: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txout_asset()
		})
		if checksum != 43008 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txout_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txout_is_fee()
		})
		if checksum != 30808 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txout_is_fee: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txout_is_partially_blinded()
		})
		if checksum != 10893 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txout_is_partially_blinded: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txout_script_pubkey()
		})
		if checksum != 7466 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txout_script_pubkey: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txout_unblind()
		})
		if checksum != 11168 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txout_unblind: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txout_unconfidential_address()
		})
		if checksum != 3790 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txout_unconfidential_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txout_value()
		})
		if checksum != 6745 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txout_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txoutsecrets_asset()
		})
		if checksum != 26014 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txoutsecrets_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txoutsecrets_asset_bf()
		})
		if checksum != 3179 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txoutsecrets_asset_bf: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txoutsecrets_asset_commitment()
		})
		if checksum != 16600 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txoutsecrets_asset_commitment: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txoutsecrets_is_explicit()
		})
		if checksum != 53000 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txoutsecrets_is_explicit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txoutsecrets_value()
		})
		if checksum != 16330 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txoutsecrets_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txoutsecrets_value_bf()
		})
		if checksum != 58526 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txoutsecrets_value_bf: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txoutsecrets_value_commitment()
		})
		if checksum != 41762 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txoutsecrets_value_commitment: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txid_bytes()
		})
		if checksum != 6953 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txid_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_unvalidatedliquidexproposal_insecure_validate()
		})
		if checksum != 45940 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_unvalidatedliquidexproposal_insecure_validate: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_unvalidatedliquidexproposal_needed_tx()
		})
		if checksum != 38170 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_unvalidatedliquidexproposal_needed_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_unvalidatedliquidexproposal_validate()
		})
		if checksum != 11143 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_unvalidatedliquidexproposal_validate: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_update_only_tip()
		})
		if checksum != 55966 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_update_only_tip: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_update_serialize()
		})
		if checksum != 15229 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_update_serialize: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_validatedliquidexproposal_input()
		})
		if checksum != 14781 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_validatedliquidexproposal_input: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_validatedliquidexproposal_output()
		})
		if checksum != 46590 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_validatedliquidexproposal_output: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_valueblindingfactor_to_bytes()
		})
		if checksum != 55005 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_valueblindingfactor_to_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_balance()
		})
		if checksum != 48414 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_balance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_fee()
		})
		if checksum != 29198 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_fee: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_height()
		})
		if checksum != 56545 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_height: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_inputs()
		})
		if checksum != 45012 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_inputs: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_outputs()
		})
		if checksum != 28655 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_outputs: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_timestamp()
		})
		if checksum != 29251 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_timestamp: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_tx()
		})
		if checksum != 18508 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_txid()
		})
		if checksum != 44692 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_type_()
		})
		if checksum != 60338 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_type_: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_unblinded_url()
		})
		if checksum != 46683 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_unblinded_url: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettxout_address()
		})
		if checksum != 51786 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettxout_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettxout_ext_int()
		})
		if checksum != 47840 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettxout_ext_int: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettxout_height()
		})
		if checksum != 31312 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettxout_height: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettxout_outpoint()
		})
		if checksum != 22039 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettxout_outpoint: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettxout_script_pubkey()
		})
		if checksum != 23842 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettxout_script_pubkey: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettxout_unblinded()
		})
		if checksum != 56966 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettxout_unblinded: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettxout_wildcard_index()
		})
		if checksum != 44054 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettxout_wildcard_index: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_add_details()
		})
		if checksum != 42615 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_add_details: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_address()
		})
		if checksum != 64900 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_apply_transaction()
		})
		if checksum != 55817 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_apply_transaction: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_apply_update()
		})
		if checksum != 39211 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_apply_update: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_balance()
		})
		if checksum != 34807 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_balance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_descriptor()
		})
		if checksum != 14476 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_descriptor: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_dwid()
		})
		if checksum != 60794 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_dwid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_extract_wallet_utxos()
		})
		if checksum != 43538 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_extract_wallet_utxos: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_finalize()
		})
		if checksum != 19423 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_finalize: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_is_amp0()
		})
		if checksum != 63030 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_is_amp0: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_is_segwit()
		})
		if checksum != 18539 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_is_segwit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_max_weight_to_satisfy()
		})
		if checksum != 8240 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_max_weight_to_satisfy: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_pset_details()
		})
		if checksum != 3928 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_pset_details: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_transaction()
		})
		if checksum != 42318 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_transaction: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_transactions()
		})
		if checksum != 38030 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_transactions: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_transactions_paginated()
		})
		if checksum != 54846 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_transactions_paginated: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_txos()
		})
		if checksum != 19061 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_txos: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_unblind_utxos_with()
		})
		if checksum != 51999 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_unblind_utxos_with: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_utxos()
		})
		if checksum != 3120 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_utxos: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_wait_for_tx()
		})
		if checksum != 47828 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_wait_for_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wolletbuilder_build()
		})
		if checksum != 21047 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wolletbuilder_build: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wolletbuilder_with_legacy_fs_store()
		})
		if checksum != 35203 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wolletbuilder_with_legacy_fs_store: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wolletbuilder_with_merge_threshold()
		})
		if checksum != 37282 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wolletbuilder_with_merge_threshold: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wolletbuilder_with_store()
		})
		if checksum != 51071 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wolletbuilder_with_store: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wolletdescriptor_derive_blinding_key()
		})
		if checksum != 27121 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wolletdescriptor_derive_blinding_key: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wolletdescriptor_is_amp0()
		})
		if checksum != 49462 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wolletdescriptor_is_amp0: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wolletdescriptor_is_mainnet()
		})
		if checksum != 42870 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wolletdescriptor_is_mainnet: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wolletdescriptor_script_pubkey()
		})
		if checksum != 21566 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wolletdescriptor_script_pubkey: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wolletdescriptor_url_encoded_descriptor()
		})
		if checksum != 21106 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wolletdescriptor_url_encoded_descriptor: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_address_new()
		})
		if checksum != 52129 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_address_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_amp0_new()
		})
		if checksum != 64357 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_amp0_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_amp0connected_new()
		})
		if checksum != 62535 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_amp0connected_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_amp0pset_new()
		})
		if checksum != 58003 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_amp0pset_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_amp2_new_testnet()
		})
		if checksum != 61837 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_amp2_new_testnet: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_anyclient_from_electrum()
		})
		if checksum != 61969 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_anyclient_from_electrum: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_anyclient_from_esplora()
		})
		if checksum != 17175 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_anyclient_from_esplora: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_assetblindingfactor_from_bytes()
		})
		if checksum != 55914 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_assetblindingfactor_from_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_assetblindingfactor_from_string()
		})
		if checksum != 36114 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_assetblindingfactor_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_assetblindingfactor_zero()
		})
		if checksum != 33990 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_assetblindingfactor_zero: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_bip_new_bip49()
		})
		if checksum != 34169 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_bip_new_bip49: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_bip_new_bip84()
		})
		if checksum != 26707 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_bip_new_bip84: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_bip_new_bip87()
		})
		if checksum != 60988 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_bip_new_bip87: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_bip21_new()
		})
		if checksum != 63505 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_bip21_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_bip321_new()
		})
		if checksum != 5120 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_bip321_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_bitcoinaddress_new()
		})
		if checksum != 46661 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_bitcoinaddress_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_bolt11invoice_new()
		})
		if checksum != 63126 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_bolt11invoice_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_boltzsession_from_builder()
		})
		if checksum != 64185 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_boltzsession_from_builder: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_boltzsession_new()
		})
		if checksum != 40021 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_boltzsession_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_contract_new()
		})
		if checksum != 55905 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_contract_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_currencycode_new()
		})
		if checksum != 9828 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_currencycode_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_electrumclient_from_url()
		})
		if checksum != 21158 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_electrumclient_from_url: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_electrumclient_new()
		})
		if checksum != 26281 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_electrumclient_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_esploraclient_from_builder()
		})
		if checksum != 26617 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_esploraclient_from_builder: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_esploraclient_new()
		})
		if checksum != 42490 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_esploraclient_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_esploraclient_new_waterfalls()
		})
		if checksum != 40758 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_esploraclient_new_waterfalls: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_externalutxo_new()
		})
		if checksum != 40531 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_externalutxo_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_foreignstorelink_new()
		})
		if checksum != 29701 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_foreignstorelink_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_lightningpayment_from_bolt11_invoice()
		})
		if checksum != 15133 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_lightningpayment_from_bolt11_invoice: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_lightningpayment_new()
		})
		if checksum != 20178 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_lightningpayment_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_logginglink_new()
		})
		if checksum != 31235 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_logginglink_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_lwktestenv_new()
		})
		if checksum != 2775 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_lwktestenv_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_lwkteststore_new()
		})
		if checksum != 48161 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_lwkteststore_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_mnemonic_from_entropy()
		})
		if checksum != 36360 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_mnemonic_from_entropy: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_mnemonic_from_random()
		})
		if checksum != 35644 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_mnemonic_from_random: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_mnemonic_new()
		})
		if checksum != 33187 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_mnemonic_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_network_mainnet()
		})
		if checksum != 19485 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_network_mainnet: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_network_regtest()
		})
		if checksum != 43636 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_network_regtest: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_network_regtest_default()
		})
		if checksum != 44487 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_network_regtest_default: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_network_testnet()
		})
		if checksum != 61286 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_network_testnet: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_outpoint_from_parts()
		})
		if checksum != 30123 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_outpoint_from_parts: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_outpoint_new()
		})
		if checksum != 3858 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_outpoint_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_payment_new()
		})
		if checksum != 37193 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_payment_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_posconfig_decode()
		})
		if checksum != 60370 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_posconfig_decode: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_posconfig_new()
		})
		if checksum != 49139 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_posconfig_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_posconfig_with_options()
		})
		if checksum != 1396 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_posconfig_with_options: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_precision_new()
		})
		if checksum != 7694 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_precision_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_pset_new()
		})
		if checksum != 61694 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_pset_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_script_empty()
		})
		if checksum != 47087 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_script_empty: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_script_new()
		})
		if checksum != 12404 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_script_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_script_new_op_return()
		})
		if checksum != 7079 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_script_new_op_return: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_secretkey_from_bytes()
		})
		if checksum != 11901 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_secretkey_from_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_secretkey_from_wif()
		})
		if checksum != 14837 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_secretkey_from_wif: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_signer_new()
		})
		if checksum != 16701 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_signer_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_signer_random()
		})
		if checksum != 54097 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_signer_random: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_transaction_from_bytes()
		})
		if checksum != 6677 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_transaction_from_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_transaction_from_string()
		})
		if checksum != 61469 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_transaction_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_transaction_new()
		})
		if checksum != 34031 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_transaction_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_txbuilder_new()
		})
		if checksum != 56158 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_txbuilder_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_txout_from_explicit()
		})
		if checksum != 4839 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_txout_from_explicit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_txid_new()
		})
		if checksum != 63870 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_txid_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_unvalidatedliquidexproposal_from_pset()
		})
		if checksum != 44953 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_unvalidatedliquidexproposal_from_pset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_unvalidatedliquidexproposal_new()
		})
		if checksum != 55322 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_unvalidatedliquidexproposal_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_update_new()
		})
		if checksum != 5357 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_update_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_valueblindingfactor_from_bytes()
		})
		if checksum != 35178 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_valueblindingfactor_from_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_valueblindingfactor_from_string()
		})
		if checksum != 63848 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_valueblindingfactor_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_valueblindingfactor_zero()
		})
		if checksum != 49915 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_valueblindingfactor_zero: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_webhook_new()
		})
		if checksum != 14880 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_webhook_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_wollet_new()
		})
		if checksum != 15308 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_wollet_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_wollet_with_custom_store()
		})
		if checksum != 9255 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_wollet_with_custom_store: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_wolletbuilder_new()
		})
		if checksum != 41459 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_wolletbuilder_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_wolletdescriptor_new()
		})
		if checksum != 61281 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_wolletdescriptor_new: UniFFI API checksum mismatch")
		}
	}
}

type FfiConverterUint8 struct{}

var FfiConverterUint8INSTANCE = FfiConverterUint8{}

func (FfiConverterUint8) Lower(value uint8) C.uint8_t {
	return C.uint8_t(value)
}

func (FfiConverterUint8) Write(writer io.Writer, value uint8) {
	writeUint8(writer, value)
}

func (FfiConverterUint8) Lift(value C.uint8_t) uint8 {
	return uint8(value)
}

func (FfiConverterUint8) Read(reader io.Reader) uint8 {
	return readUint8(reader)
}

type FfiDestroyerUint8 struct{}

func (FfiDestroyerUint8) Destroy(_ uint8) {}

type FfiConverterInt8 struct{}

var FfiConverterInt8INSTANCE = FfiConverterInt8{}

func (FfiConverterInt8) Lower(value int8) C.int8_t {
	return C.int8_t(value)
}

func (FfiConverterInt8) Write(writer io.Writer, value int8) {
	writeInt8(writer, value)
}

func (FfiConverterInt8) Lift(value C.int8_t) int8 {
	return int8(value)
}

func (FfiConverterInt8) Read(reader io.Reader) int8 {
	return readInt8(reader)
}

type FfiDestroyerInt8 struct{}

func (FfiDestroyerInt8) Destroy(_ int8) {}

type FfiConverterUint16 struct{}

var FfiConverterUint16INSTANCE = FfiConverterUint16{}

func (FfiConverterUint16) Lower(value uint16) C.uint16_t {
	return C.uint16_t(value)
}

func (FfiConverterUint16) Write(writer io.Writer, value uint16) {
	writeUint16(writer, value)
}

func (FfiConverterUint16) Lift(value C.uint16_t) uint16 {
	return uint16(value)
}

func (FfiConverterUint16) Read(reader io.Reader) uint16 {
	return readUint16(reader)
}

type FfiDestroyerUint16 struct{}

func (FfiDestroyerUint16) Destroy(_ uint16) {}

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

type FfiConverterFloat32 struct{}

var FfiConverterFloat32INSTANCE = FfiConverterFloat32{}

func (FfiConverterFloat32) Lower(value float32) C.float {
	return C.float(value)
}

func (FfiConverterFloat32) Write(writer io.Writer, value float32) {
	writeFloat32(writer, value)
}

func (FfiConverterFloat32) Lift(value C.float) float32 {
	return float32(value)
}

func (FfiConverterFloat32) Read(reader io.Reader) float32 {
	return readFloat32(reader)
}

type FfiDestroyerFloat32 struct{}

func (FfiDestroyerFloat32) Destroy(_ float32) {}

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

func (c FfiConverterString) LowerExternal(value string) ExternalCRustBuffer {
	return RustBufferFromC(stringToRustBuffer(value))
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

type FfiConverterBytes struct{}

var FfiConverterBytesINSTANCE = FfiConverterBytes{}

func (c FfiConverterBytes) Lower(value []byte) C.RustBuffer {
	return LowerIntoRustBuffer[[]byte](c, value)
}

func (c FfiConverterBytes) LowerExternal(value []byte) ExternalCRustBuffer {
	return RustBufferFromC(c.Lower(value))
}

func (c FfiConverterBytes) Write(writer io.Writer, value []byte) {
	if len(value) > math.MaxInt32 {
		panic("[]byte is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := writer.Write(value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing []byte, expected %d, written %d", len(value), write_length))
	}
}

func (c FfiConverterBytes) Lift(rb RustBufferI) []byte {
	return LiftFromRustBuffer[[]byte](c, rb)
}

func (c FfiConverterBytes) Read(reader io.Reader) []byte {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading []byte, expected %d, read %d", length, read_length))
	}
	return buffer
}

type FfiDestroyerBytes struct{}

func (FfiDestroyerBytes) Destroy(_ []byte) {}

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

// A Liquid address
type AddressInterface interface {
	// Return true if the address is blinded.
	IsBlinded() bool
	// Returns the network of the address
	Network() *Network
	// Returns a string of the QR code printable in a terminal environment
	QrCodeText() (string, error)
	// Returns a string encoding an image in a uri
	//
	// The string can be open in the browser or be used as `src` field in `img` in HTML
	//
	// For max efficiency we suggest to pass `None` to `pixel_per_module`, get a very small image
	// and use styling to scale up the image in the browser. eg
	// `style="image-rendering: pixelated; border: 20px solid white;"`
	QrCodeUri(pixelPerModule *uint8) (string, error)
	// Return the script pubkey of the address.
	ScriptPubkey() *Script
	// Return the unconfidential address.
	ToUnconfidential() *Address
}

// A Liquid address
type Address struct {
	ffiObject FfiObject
}

// Construct an Address object
func NewAddress(s string) (*Address, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_address_new(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Address
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAddressINSTANCE.Lift(_uniffiRV), nil
	}
}

// Return true if the address is blinded.
func (_self *Address) IsBlinded() bool {
	_pointer := _self.ffiObject.incrementPointer("*Address")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_address_is_blinded(
			_pointer, _uniffiStatus)
	}))
}

// Returns the network of the address
func (_self *Address) Network() *Network {
	_pointer := _self.ffiObject.incrementPointer("*Address")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterNetworkINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_address_network(
			_pointer, _uniffiStatus)
	}))
}

// Returns a string of the QR code printable in a terminal environment
func (_self *Address) QrCodeText() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Address")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_address_qr_code_text(
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

// Returns a string encoding an image in a uri
//
// # The string can be open in the browser or be used as `src` field in `img` in HTML
//
// For max efficiency we suggest to pass `None` to `pixel_per_module`, get a very small image
// and use styling to scale up the image in the browser. eg
// `style="image-rendering: pixelated; border: 20px solid white;"`
func (_self *Address) QrCodeUri(pixelPerModule *uint8) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Address")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_address_qr_code_uri(
				_pointer, FfiConverterOptionalUint8INSTANCE.Lower(pixelPerModule), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// Return the script pubkey of the address.
func (_self *Address) ScriptPubkey() *Script {
	_pointer := _self.ffiObject.incrementPointer("*Address")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterScriptINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_address_script_pubkey(
			_pointer, _uniffiStatus)
	}))
}

// Return the unconfidential address.
func (_self *Address) ToUnconfidential() *Address {
	_pointer := _self.ffiObject.incrementPointer("*Address")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterAddressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_address_to_unconfidential(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Address) String() string {
	_pointer := _self.ffiObject.incrementPointer("*Address")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_address_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *Address) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAddress struct{}

var FfiConverterAddressINSTANCE = FfiConverterAddress{}

func (c FfiConverterAddress) Lift(pointer unsafe.Pointer) *Address {
	result := &Address{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_address(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_address(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Address).Destroy)
	return result
}

func (c FfiConverterAddress) Read(reader io.Reader) *Address {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAddress) Lower(value *Address) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Address")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAddress) Write(writer io.Writer, value *Address) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAddress struct{}

func (_ FfiDestroyerAddress) Destroy(value *Address) {
	value.Destroy()
}

// Value returned from asking an address to the wallet.
// Containing the confidential address and its
// derivation index (the last element in the derivation path)
type AddressResultInterface interface {
	// Return the address.
	Address() *Address
	// Return the derivation index of the address.
	Index() uint32
}

// Value returned from asking an address to the wallet.
// Containing the confidential address and its
// derivation index (the last element in the derivation path)
type AddressResult struct {
	ffiObject FfiObject
}

// Return the address.
func (_self *AddressResult) Address() *Address {
	_pointer := _self.ffiObject.incrementPointer("*AddressResult")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterAddressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_addressresult_address(
			_pointer, _uniffiStatus)
	}))
}

// Return the derivation index of the address.
func (_self *AddressResult) Index() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*AddressResult")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_addressresult_index(
			_pointer, _uniffiStatus)
	}))
}
func (object *AddressResult) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAddressResult struct{}

var FfiConverterAddressResultINSTANCE = FfiConverterAddressResult{}

func (c FfiConverterAddressResult) Lift(pointer unsafe.Pointer) *AddressResult {
	result := &AddressResult{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_addressresult(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_addressresult(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*AddressResult).Destroy)
	return result
}

func (c FfiConverterAddressResult) Read(reader io.Reader) *AddressResult {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAddressResult) Lower(value *AddressResult) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*AddressResult")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAddressResult) Write(writer io.Writer, value *AddressResult) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAddressResult struct{}

func (_ FfiDestroyerAddressResult) Destroy(value *AddressResult) {
	value.Destroy()
}

// Context for actions related to an AMP0 (sub)account
type Amp0Interface interface {
	// Get an address
	//
	// If `index` is None, a new address is returned.
	Address(index *uint32) (*AddressResult, error)
	// AMP ID
	AmpId() (string, error)
	// Index of the last returned address
	LastIndex() (uint32, error)
	// Ask AMP0 server to cosign
	Sign(pset *Amp0Pset) (*Transaction, error)
	// Wollet descriptor
	WolletDescriptor() (*WolletDescriptor, error)
}

// Context for actions related to an AMP0 (sub)account
type Amp0 struct {
	ffiObject FfiObject
}

// Construct an AMP0 context
func NewAmp0(network *Network, username string, password string, ampId string) (*Amp0, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_amp0_new(FfiConverterNetworkINSTANCE.Lower(network), FfiConverterStringINSTANCE.Lower(username), FfiConverterStringINSTANCE.Lower(password), FfiConverterStringINSTANCE.Lower(ampId), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Amp0
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAmp0INSTANCE.Lift(_uniffiRV), nil
	}
}

// Get an address
//
// If `index` is None, a new address is returned.
func (_self *Amp0) Address(index *uint32) (*AddressResult, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp0")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_amp0_address(
			_pointer, FfiConverterOptionalUint32INSTANCE.Lower(index), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *AddressResult
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAddressResultINSTANCE.Lift(_uniffiRV), nil
	}
}

// AMP ID
func (_self *Amp0) AmpId() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp0")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_amp0_amp_id(
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

// Index of the last returned address
func (_self *Amp0) LastIndex() (uint32, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp0")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_amp0_last_index(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue uint32
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUint32INSTANCE.Lift(_uniffiRV), nil
	}
}

// Ask AMP0 server to cosign
func (_self *Amp0) Sign(pset *Amp0Pset) (*Transaction, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp0")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_amp0_sign(
			_pointer, FfiConverterAmp0PsetINSTANCE.Lower(pset), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Transaction
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTransactionINSTANCE.Lift(_uniffiRV), nil
	}
}

// Wollet descriptor
func (_self *Amp0) WolletDescriptor() (*WolletDescriptor, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp0")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_amp0_wollet_descriptor(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *WolletDescriptor
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWolletDescriptorINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *Amp0) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAmp0 struct{}

var FfiConverterAmp0INSTANCE = FfiConverterAmp0{}

func (c FfiConverterAmp0) Lift(pointer unsafe.Pointer) *Amp0 {
	result := &Amp0{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_amp0(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_amp0(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Amp0).Destroy)
	return result
}

func (c FfiConverterAmp0) Read(reader io.Reader) *Amp0 {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAmp0) Lower(value *Amp0) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Amp0")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAmp0) Write(writer io.Writer, value *Amp0) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAmp0 struct{}

func (_ FfiDestroyerAmp0) Destroy(value *Amp0) {
	value.Destroy()
}

// Session connecting to AMP0
type Amp0ConnectedInterface interface {
	// Obtain a login challenge
	//
	// This must be signed with [`Signer::amp0_sign_challenge()`].
	GetChallenge() (string, error)
	// Log in
	//
	// `sig` must be obtained from [`Signer::amp0_sign_challenge()`] called with the value returned
	// by [`Amp0Connected::get_challenge()`]
	Login(sig string) (*Amp0LoggedIn, error)
}

// Session connecting to AMP0
type Amp0Connected struct {
	ffiObject FfiObject
}

// Connect and register to AMP0
func NewAmp0Connected(network *Network, signerData *Amp0SignerData) (*Amp0Connected, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_amp0connected_new(FfiConverterNetworkINSTANCE.Lower(network), FfiConverterAmp0SignerDataINSTANCE.Lower(signerData), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Amp0Connected
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAmp0ConnectedINSTANCE.Lift(_uniffiRV), nil
	}
}

// Obtain a login challenge
//
// This must be signed with [`Signer::amp0_sign_challenge()`].
func (_self *Amp0Connected) GetChallenge() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp0Connected")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_amp0connected_get_challenge(
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

// Log in
//
// `sig` must be obtained from [`Signer::amp0_sign_challenge()`] called with the value returned
// by [`Amp0Connected::get_challenge()`]
func (_self *Amp0Connected) Login(sig string) (*Amp0LoggedIn, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp0Connected")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_amp0connected_login(
			_pointer, FfiConverterStringINSTANCE.Lower(sig), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Amp0LoggedIn
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAmp0LoggedInINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *Amp0Connected) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAmp0Connected struct{}

var FfiConverterAmp0ConnectedINSTANCE = FfiConverterAmp0Connected{}

func (c FfiConverterAmp0Connected) Lift(pointer unsafe.Pointer) *Amp0Connected {
	result := &Amp0Connected{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_amp0connected(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_amp0connected(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Amp0Connected).Destroy)
	return result
}

func (c FfiConverterAmp0Connected) Read(reader io.Reader) *Amp0Connected {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAmp0Connected) Lower(value *Amp0Connected) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Amp0Connected")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAmp0Connected) Write(writer io.Writer, value *Amp0Connected) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAmp0Connected struct{}

func (_ FfiDestroyerAmp0Connected) Destroy(value *Amp0Connected) {
	value.Destroy()
}

// Session logged in AMP0
type Amp0LoggedInInterface interface {
	// Create a new AMP0 account
	//
	// `account_xpub` must be obtained from [`Signer::amp0_account_xpub()`] called with the value obtained from
	// [`Amp0LoggedIn::next_account()`]
	CreateAmp0Account(pointer uint32, accountXpub string) (string, error)
	// Create a new Watch-Only entry for this wallet
	CreateWatchOnly(username string, password string) error
	// List of AMP IDs.
	GetAmpIds() ([]string, error)
	// Get the next account for AMP0 account creation
	//
	// This must be given to [`Signer::amp0_account_xpub()`] to obtain the xpub to pass to
	// [`Amp0LoggedIn::create_amp0_account()`]
	NextAccount() (uint32, error)
}

// Session logged in AMP0
type Amp0LoggedIn struct {
	ffiObject FfiObject
}

// Create a new AMP0 account
//
// `account_xpub` must be obtained from [`Signer::amp0_account_xpub()`] called with the value obtained from
// [`Amp0LoggedIn::next_account()`]
func (_self *Amp0LoggedIn) CreateAmp0Account(pointer uint32, accountXpub string) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp0LoggedIn")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_amp0loggedin_create_amp0_account(
				_pointer, FfiConverterUint32INSTANCE.Lower(pointer), FfiConverterStringINSTANCE.Lower(accountXpub), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// Create a new Watch-Only entry for this wallet
func (_self *Amp0LoggedIn) CreateWatchOnly(username string, password string) error {
	_pointer := _self.ffiObject.incrementPointer("*Amp0LoggedIn")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_amp0loggedin_create_watch_only(
			_pointer, FfiConverterStringINSTANCE.Lower(username), FfiConverterStringINSTANCE.Lower(password), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// List of AMP IDs.
func (_self *Amp0LoggedIn) GetAmpIds() ([]string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp0LoggedIn")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_amp0loggedin_get_amp_ids(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the next account for AMP0 account creation
//
// This must be given to [`Signer::amp0_account_xpub()`] to obtain the xpub to pass to
// [`Amp0LoggedIn::create_amp0_account()`]
func (_self *Amp0LoggedIn) NextAccount() (uint32, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp0LoggedIn")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_amp0loggedin_next_account(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue uint32
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUint32INSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *Amp0LoggedIn) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAmp0LoggedIn struct{}

var FfiConverterAmp0LoggedInINSTANCE = FfiConverterAmp0LoggedIn{}

func (c FfiConverterAmp0LoggedIn) Lift(pointer unsafe.Pointer) *Amp0LoggedIn {
	result := &Amp0LoggedIn{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_amp0loggedin(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_amp0loggedin(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Amp0LoggedIn).Destroy)
	return result
}

func (c FfiConverterAmp0LoggedIn) Read(reader io.Reader) *Amp0LoggedIn {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAmp0LoggedIn) Lower(value *Amp0LoggedIn) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Amp0LoggedIn")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAmp0LoggedIn) Write(writer io.Writer, value *Amp0LoggedIn) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAmp0LoggedIn struct{}

func (_ FfiDestroyerAmp0LoggedIn) Destroy(value *Amp0LoggedIn) {
	value.Destroy()
}

// A PSET to use with AMP0
type Amp0PsetInterface interface {
	// Get blinding nonces
	BlindingNonces() ([]string, error)
	// Get the PSET
	Pset() (*Pset, error)
}

// A PSET to use with AMP0
type Amp0Pset struct {
	ffiObject FfiObject
}

// Construct a PSET to use with AMP0
func NewAmp0Pset(pset *Pset, blindingNonces []string) (*Amp0Pset, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_amp0pset_new(FfiConverterPsetINSTANCE.Lower(pset), FfiConverterSequenceStringINSTANCE.Lower(blindingNonces), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Amp0Pset
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAmp0PsetINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get blinding nonces
func (_self *Amp0Pset) BlindingNonces() ([]string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp0Pset")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_amp0pset_blinding_nonces(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the PSET
func (_self *Amp0Pset) Pset() (*Pset, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp0Pset")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_amp0pset_pset(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Pset
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPsetINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *Amp0Pset) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAmp0Pset struct{}

var FfiConverterAmp0PsetINSTANCE = FfiConverterAmp0Pset{}

func (c FfiConverterAmp0Pset) Lift(pointer unsafe.Pointer) *Amp0Pset {
	result := &Amp0Pset{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_amp0pset(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_amp0pset(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Amp0Pset).Destroy)
	return result
}

func (c FfiConverterAmp0Pset) Read(reader io.Reader) *Amp0Pset {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAmp0Pset) Lower(value *Amp0Pset) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Amp0Pset")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAmp0Pset) Write(writer io.Writer, value *Amp0Pset) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAmp0Pset struct{}

func (_ FfiDestroyerAmp0Pset) Destroy(value *Amp0Pset) {
	value.Destroy()
}

// Signer information necessary for full login to AMP0
type Amp0SignerDataInterface interface {
}

// Signer information necessary for full login to AMP0
type Amp0SignerData struct {
	ffiObject FfiObject
}

func (_self *Amp0SignerData) String() string {
	_pointer := _self.ffiObject.incrementPointer("*Amp0SignerData")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_amp0signerdata_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *Amp0SignerData) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAmp0SignerData struct{}

var FfiConverterAmp0SignerDataINSTANCE = FfiConverterAmp0SignerData{}

func (c FfiConverterAmp0SignerData) Lift(pointer unsafe.Pointer) *Amp0SignerData {
	result := &Amp0SignerData{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_amp0signerdata(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_amp0signerdata(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Amp0SignerData).Destroy)
	return result
}

func (c FfiConverterAmp0SignerData) Read(reader io.Reader) *Amp0SignerData {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAmp0SignerData) Lower(value *Amp0SignerData) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Amp0SignerData")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAmp0SignerData) Write(writer io.Writer, value *Amp0SignerData) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAmp0SignerData struct{}

func (_ FfiDestroyerAmp0SignerData) Destroy(value *Amp0SignerData) {
	value.Destroy()
}

// Wrapper over [`lwk_wollet::amp2::Amp2`]
type Amp2Interface interface {
	// Ask the AMP2 server to cosign a PSET
	Cosign(pset *Pset) (*Pset, error)
	// Create an AMP2 wallet descriptor from the keyorigin xpub of a signer
	DescriptorFromStr(keyoriginXpub string) (*Amp2Descriptor, error)
	// Register an AMP2 wallet with the AMP2 server
	RegisterWallet(desc *Amp2Descriptor) (string, error)
}

// Wrapper over [`lwk_wollet::amp2::Amp2`]
type Amp2 struct {
	ffiObject FfiObject
}

// Construct an AMP2 context for Liquid Testnet
func Amp2NewTestnet() *Amp2 {
	return FfiConverterAmp2INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_amp2_new_testnet(_uniffiStatus)
	}))
}

// Ask the AMP2 server to cosign a PSET
func (_self *Amp2) Cosign(pset *Pset) (*Pset, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp2")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_amp2_cosign(
			_pointer, FfiConverterPsetINSTANCE.Lower(pset), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Pset
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPsetINSTANCE.Lift(_uniffiRV), nil
	}
}

// Create an AMP2 wallet descriptor from the keyorigin xpub of a signer
func (_self *Amp2) DescriptorFromStr(keyoriginXpub string) (*Amp2Descriptor, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp2")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_amp2_descriptor_from_str(
			_pointer, FfiConverterStringINSTANCE.Lower(keyoriginXpub), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Amp2Descriptor
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAmp2DescriptorINSTANCE.Lift(_uniffiRV), nil
	}
}

// Register an AMP2 wallet with the AMP2 server
func (_self *Amp2) RegisterWallet(desc *Amp2Descriptor) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp2")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_amp2_register_wallet(
				_pointer, FfiConverterAmp2DescriptorINSTANCE.Lower(desc), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *Amp2) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAmp2 struct{}

var FfiConverterAmp2INSTANCE = FfiConverterAmp2{}

func (c FfiConverterAmp2) Lift(pointer unsafe.Pointer) *Amp2 {
	result := &Amp2{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_amp2(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_amp2(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Amp2).Destroy)
	return result
}

func (c FfiConverterAmp2) Read(reader io.Reader) *Amp2 {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAmp2) Lower(value *Amp2) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Amp2")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAmp2) Write(writer io.Writer, value *Amp2) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAmp2 struct{}

func (_ FfiDestroyerAmp2) Destroy(value *Amp2) {
	value.Destroy()
}

// Wrapper over [`lwk_wollet::amp2::Amp2Descriptor`]
type Amp2DescriptorInterface interface {
	Descriptor() *WolletDescriptor
}

// Wrapper over [`lwk_wollet::amp2::Amp2Descriptor`]
type Amp2Descriptor struct {
	ffiObject FfiObject
}

func (_self *Amp2Descriptor) Descriptor() *WolletDescriptor {
	_pointer := _self.ffiObject.incrementPointer("*Amp2Descriptor")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterWolletDescriptorINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_amp2descriptor_descriptor(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Amp2Descriptor) String() string {
	_pointer := _self.ffiObject.incrementPointer("*Amp2Descriptor")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_amp2descriptor_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *Amp2Descriptor) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAmp2Descriptor struct{}

var FfiConverterAmp2DescriptorINSTANCE = FfiConverterAmp2Descriptor{}

func (c FfiConverterAmp2Descriptor) Lift(pointer unsafe.Pointer) *Amp2Descriptor {
	result := &Amp2Descriptor{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_amp2descriptor(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_amp2descriptor(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Amp2Descriptor).Destroy)
	return result
}

func (c FfiConverterAmp2Descriptor) Read(reader io.Reader) *Amp2Descriptor {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAmp2Descriptor) Lower(value *Amp2Descriptor) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Amp2Descriptor")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAmp2Descriptor) Write(writer io.Writer, value *Amp2Descriptor) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAmp2Descriptor struct{}

func (_ FfiDestroyerAmp2Descriptor) Destroy(value *Amp2Descriptor) {
	value.Destroy()
}

type AnyClientInterface interface {
}
type AnyClient struct {
	ffiObject FfiObject
}

func AnyClientFromElectrum(client *ElectrumClient) *AnyClient {
	return FfiConverterAnyClientINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_anyclient_from_electrum(FfiConverterElectrumClientINSTANCE.Lower(client), _uniffiStatus)
	}))
}

func AnyClientFromEsplora(client *EsploraClient) *AnyClient {
	return FfiConverterAnyClientINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_anyclient_from_esplora(FfiConverterEsploraClientINSTANCE.Lower(client), _uniffiStatus)
	}))
}

func (object *AnyClient) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAnyClient struct{}

var FfiConverterAnyClientINSTANCE = FfiConverterAnyClient{}

func (c FfiConverterAnyClient) Lift(pointer unsafe.Pointer) *AnyClient {
	result := &AnyClient{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_anyclient(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_anyclient(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*AnyClient).Destroy)
	return result
}

func (c FfiConverterAnyClient) Read(reader io.Reader) *AnyClient {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAnyClient) Lower(value *AnyClient) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*AnyClient")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAnyClient) Write(writer io.Writer, value *AnyClient) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAnyClient struct{}

func (_ FfiDestroyerAnyClient) Destroy(value *AnyClient) {
	value.Destroy()
}

// An asset identifier and an amount
type AssetAmountInterface interface {
	// Return the amount of the asset
	Amount() uint64
	// Return the asset of the amount
	Asset() AssetId
}

// An asset identifier and an amount
type AssetAmount struct {
	ffiObject FfiObject
}

// Return the amount of the asset
func (_self *AssetAmount) Amount() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*AssetAmount")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_assetamount_amount(
			_pointer, _uniffiStatus)
	}))
}

// Return the asset of the amount
func (_self *AssetAmount) Asset() AssetId {
	_pointer := _self.ffiObject.incrementPointer("*AssetAmount")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeAssetIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_assetamount_asset(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *AssetAmount) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAssetAmount struct{}

var FfiConverterAssetAmountINSTANCE = FfiConverterAssetAmount{}

func (c FfiConverterAssetAmount) Lift(pointer unsafe.Pointer) *AssetAmount {
	result := &AssetAmount{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_assetamount(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_assetamount(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*AssetAmount).Destroy)
	return result
}

func (c FfiConverterAssetAmount) Read(reader io.Reader) *AssetAmount {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAssetAmount) Lower(value *AssetAmount) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*AssetAmount")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAssetAmount) Write(writer io.Writer, value *AssetAmount) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAssetAmount struct{}

func (_ FfiDestroyerAssetAmount) Destroy(value *AssetAmount) {
	value.Destroy()
}

// A blinding factor for asset commitments.
type AssetBlindingFactorInterface interface {
	// Returns the bytes (32 bytes).
	ToBytes() []byte
}

// A blinding factor for asset commitments.
type AssetBlindingFactor struct {
	ffiObject FfiObject
}

// Create from bytes.
func AssetBlindingFactorFromBytes(bytes []byte) (*AssetBlindingFactor, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_assetblindingfactor_from_bytes(FfiConverterBytesINSTANCE.Lower(bytes), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *AssetBlindingFactor
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAssetBlindingFactorINSTANCE.Lift(_uniffiRV), nil
	}
}

// Creates from a hex string.
func AssetBlindingFactorFromString(s string) (*AssetBlindingFactor, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_assetblindingfactor_from_string(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *AssetBlindingFactor
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAssetBlindingFactorINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get a unblinded/zero asset blinding factor
func AssetBlindingFactorZero() *AssetBlindingFactor {
	return FfiConverterAssetBlindingFactorINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_assetblindingfactor_zero(_uniffiStatus)
	}))
}

// Returns the bytes (32 bytes).
func (_self *AssetBlindingFactor) ToBytes() []byte {
	_pointer := _self.ffiObject.incrementPointer("*AssetBlindingFactor")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_assetblindingfactor_to_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *AssetBlindingFactor) String() string {
	_pointer := _self.ffiObject.incrementPointer("*AssetBlindingFactor")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_assetblindingfactor_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *AssetBlindingFactor) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAssetBlindingFactor struct{}

var FfiConverterAssetBlindingFactorINSTANCE = FfiConverterAssetBlindingFactor{}

func (c FfiConverterAssetBlindingFactor) Lift(pointer unsafe.Pointer) *AssetBlindingFactor {
	result := &AssetBlindingFactor{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_assetblindingfactor(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_assetblindingfactor(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*AssetBlindingFactor).Destroy)
	return result
}

func (c FfiConverterAssetBlindingFactor) Read(reader io.Reader) *AssetBlindingFactor {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAssetBlindingFactor) Lower(value *AssetBlindingFactor) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*AssetBlindingFactor")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterAssetBlindingFactor) Write(writer io.Writer, value *AssetBlindingFactor) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAssetBlindingFactor struct{}

func (_ FfiDestroyerAssetBlindingFactor) Destroy(value *AssetBlindingFactor) {
	value.Destroy()
}

// wrapper over [`lwk_common::Bip`]
type BipInterface interface {
}

// wrapper over [`lwk_common::Bip`]
type Bip struct {
	ffiObject FfiObject
}

// For P2SH-P2WPKH wallets
func BipNewBip49() *Bip {
	return FfiConverterBipINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_bip_new_bip49(_uniffiStatus)
	}))
}

// For P2WPKH wallets
func BipNewBip84() *Bip {
	return FfiConverterBipINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_bip_new_bip84(_uniffiStatus)
	}))
}

// For multisig wallets
func BipNewBip87() *Bip {
	return FfiConverterBipINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_bip_new_bip87(_uniffiStatus)
	}))
}

func (object *Bip) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterBip struct{}

var FfiConverterBipINSTANCE = FfiConverterBip{}

func (c FfiConverterBip) Lift(pointer unsafe.Pointer) *Bip {
	result := &Bip{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_bip(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_bip(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Bip).Destroy)
	return result
}

func (c FfiConverterBip) Read(reader io.Reader) *Bip {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterBip) Lower(value *Bip) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Bip")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterBip) Write(writer io.Writer, value *Bip) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerBip struct{}

func (_ FfiDestroyerBip) Destroy(value *Bip) {
	value.Destroy()
}

// A parsed Bitcoin BIP21 URI with optional parameters.
//
// BIP21 URIs have the format: `bitcoin:<address>?amount=<amount>&label=<label>&message=<message>`
// They can also include lightning parameters like `lightning=<bolt11>` or `lno=<bolt12>`.
type Bip21Interface interface {
	// Returns the Bitcoin address from the BIP21 URI
	Address() *BitcoinAddress
	// Returns the amount in satoshis if present
	Amount() *uint64
	// Returns the original URI string
	AsStr() string
	// Returns the label if present
	Label() *string
	// Returns the lightning BOLT11 invoice as a string if present
	Lightning() **Bolt11Invoice
	// Returns the message if present
	Message() *string
	// Returns the BOLT12 offer as a string if present
	Offer() *string
	// Returns the payjoin endpoint URL if present
	Payjoin() *string
	// Returns whether payjoin output substitution is allowed (defaults to true if absent)
	PayjoinOutputSubstitution() bool
	// Returns the silent payment address (BIP-352) if present
	SilentPaymentAddress() *string
}

// A parsed Bitcoin BIP21 URI with optional parameters.
//
// BIP21 URIs have the format: `bitcoin:<address>?amount=<amount>&label=<label>&message=<message>`
// They can also include lightning parameters like `lightning=<bolt11>` or `lno=<bolt12>`.
type Bip21 struct {
	ffiObject FfiObject
}

// Parse a BIP21 URI string
func NewBip21(s string) (*Bip21, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_bip21_new(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Bip21
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBip21INSTANCE.Lift(_uniffiRV), nil
	}
}

// Returns the Bitcoin address from the BIP21 URI
func (_self *Bip21) Address() *BitcoinAddress {
	_pointer := _self.ffiObject.incrementPointer("*Bip21")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBitcoinAddressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_bip21_address(
			_pointer, _uniffiStatus)
	}))
}

// Returns the amount in satoshis if present
func (_self *Bip21) Amount() *uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Bip21")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip21_amount(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the original URI string
func (_self *Bip21) AsStr() string {
	_pointer := _self.ffiObject.incrementPointer("*Bip21")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip21_as_str(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the label if present
func (_self *Bip21) Label() *string {
	_pointer := _self.ffiObject.incrementPointer("*Bip21")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip21_label(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the lightning BOLT11 invoice as a string if present
func (_self *Bip21) Lightning() **Bolt11Invoice {
	_pointer := _self.ffiObject.incrementPointer("*Bip21")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalBolt11InvoiceINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip21_lightning(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the message if present
func (_self *Bip21) Message() *string {
	_pointer := _self.ffiObject.incrementPointer("*Bip21")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip21_message(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the BOLT12 offer as a string if present
func (_self *Bip21) Offer() *string {
	_pointer := _self.ffiObject.incrementPointer("*Bip21")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip21_offer(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the payjoin endpoint URL if present
func (_self *Bip21) Payjoin() *string {
	_pointer := _self.ffiObject.incrementPointer("*Bip21")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip21_payjoin(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns whether payjoin output substitution is allowed (defaults to true if absent)
func (_self *Bip21) PayjoinOutputSubstitution() bool {
	_pointer := _self.ffiObject.incrementPointer("*Bip21")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_bip21_payjoin_output_substitution(
			_pointer, _uniffiStatus)
	}))
}

// Returns the silent payment address (BIP-352) if present
func (_self *Bip21) SilentPaymentAddress() *string {
	_pointer := _self.ffiObject.incrementPointer("*Bip21")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip21_silent_payment_address(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *Bip21) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterBip21 struct{}

var FfiConverterBip21INSTANCE = FfiConverterBip21{}

func (c FfiConverterBip21) Lift(pointer unsafe.Pointer) *Bip21 {
	result := &Bip21{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_bip21(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_bip21(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Bip21).Destroy)
	return result
}

func (c FfiConverterBip21) Read(reader io.Reader) *Bip21 {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterBip21) Lower(value *Bip21) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Bip21")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterBip21) Write(writer io.Writer, value *Bip21) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerBip21 struct{}

func (_ FfiDestroyerBip21) Destroy(value *Bip21) {
	value.Destroy()
}

// A parsed Bitcoin BIP321 URI with optional parameters.
//
// BIP321 extends BIP21 by allowing URIs without a bitcoin address in the path,
// as long as there is at least one payment instruction in the query parameters.
//
// For example: `bitcoin:?ark=ark1qq...&amount=0.00000222`
type Bip321Interface interface {
	// Returns the amount in satoshis if present
	Amount() *uint64
	// Returns the ark address if present
	Ark() *string
	// Returns the original URI string
	AsStr() string
	// Returns the label if present
	Label() *string
	// Returns the lightning BOLT11 invoice as a string if present
	Lightning() **Bolt11Invoice
	// Returns the message if present
	Message() *string
	// Returns the BOLT12 offer as a string if present
	Offer() *string
	// Returns the payjoin endpoint URL if present
	Payjoin() *string
	// Returns whether payjoin output substitution is allowed (defaults to true if absent)
	PayjoinOutputSubstitution() bool
	// Returns the silent payment address (BIP-352) if present
	SilentPaymentAddress() *string
}

// A parsed Bitcoin BIP321 URI with optional parameters.
//
// BIP321 extends BIP21 by allowing URIs without a bitcoin address in the path,
// as long as there is at least one payment instruction in the query parameters.
//
// For example: `bitcoin:?ark=ark1qq...&amount=0.00000222`
type Bip321 struct {
	ffiObject FfiObject
}

// Parse a BIP321 URI string
func NewBip321(s string) (*Bip321, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_bip321_new(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Bip321
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBip321INSTANCE.Lift(_uniffiRV), nil
	}
}

// Returns the amount in satoshis if present
func (_self *Bip321) Amount() *uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Bip321")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip321_amount(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the ark address if present
func (_self *Bip321) Ark() *string {
	_pointer := _self.ffiObject.incrementPointer("*Bip321")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip321_ark(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the original URI string
func (_self *Bip321) AsStr() string {
	_pointer := _self.ffiObject.incrementPointer("*Bip321")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip321_as_str(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the label if present
func (_self *Bip321) Label() *string {
	_pointer := _self.ffiObject.incrementPointer("*Bip321")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip321_label(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the lightning BOLT11 invoice as a string if present
func (_self *Bip321) Lightning() **Bolt11Invoice {
	_pointer := _self.ffiObject.incrementPointer("*Bip321")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalBolt11InvoiceINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip321_lightning(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the message if present
func (_self *Bip321) Message() *string {
	_pointer := _self.ffiObject.incrementPointer("*Bip321")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip321_message(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the BOLT12 offer as a string if present
func (_self *Bip321) Offer() *string {
	_pointer := _self.ffiObject.incrementPointer("*Bip321")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip321_offer(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the payjoin endpoint URL if present
func (_self *Bip321) Payjoin() *string {
	_pointer := _self.ffiObject.incrementPointer("*Bip321")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip321_payjoin(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns whether payjoin output substitution is allowed (defaults to true if absent)
func (_self *Bip321) PayjoinOutputSubstitution() bool {
	_pointer := _self.ffiObject.incrementPointer("*Bip321")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_bip321_payjoin_output_substitution(
			_pointer, _uniffiStatus)
	}))
}

// Returns the silent payment address (BIP-352) if present
func (_self *Bip321) SilentPaymentAddress() *string {
	_pointer := _self.ffiObject.incrementPointer("*Bip321")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bip321_silent_payment_address(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *Bip321) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterBip321 struct{}

var FfiConverterBip321INSTANCE = FfiConverterBip321{}

func (c FfiConverterBip321) Lift(pointer unsafe.Pointer) *Bip321 {
	result := &Bip321{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_bip321(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_bip321(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Bip321).Destroy)
	return result
}

func (c FfiConverterBip321) Read(reader io.Reader) *Bip321 {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterBip321) Lower(value *Bip321) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Bip321")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterBip321) Write(writer io.Writer, value *Bip321) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerBip321 struct{}

func (_ FfiDestroyerBip321) Destroy(value *Bip321) {
	value.Destroy()
}

// A valid Bitcoin address
type BitcoinAddressInterface interface {
	// Returns the network of the address
	IsMainnet() bool
}

// A valid Bitcoin address
type BitcoinAddress struct {
	ffiObject FfiObject
}

// Construct an Address object
func NewBitcoinAddress(s string) (*BitcoinAddress, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_bitcoinaddress_new(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *BitcoinAddress
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBitcoinAddressINSTANCE.Lift(_uniffiRV), nil
	}
}

// Returns the network of the address
func (_self *BitcoinAddress) IsMainnet() bool {
	_pointer := _self.ffiObject.incrementPointer("*BitcoinAddress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_bitcoinaddress_is_mainnet(
			_pointer, _uniffiStatus)
	}))
}

func (_self *BitcoinAddress) String() string {
	_pointer := _self.ffiObject.incrementPointer("*BitcoinAddress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bitcoinaddress_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *BitcoinAddress) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterBitcoinAddress struct{}

var FfiConverterBitcoinAddressINSTANCE = FfiConverterBitcoinAddress{}

func (c FfiConverterBitcoinAddress) Lift(pointer unsafe.Pointer) *BitcoinAddress {
	result := &BitcoinAddress{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_bitcoinaddress(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_bitcoinaddress(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*BitcoinAddress).Destroy)
	return result
}

func (c FfiConverterBitcoinAddress) Read(reader io.Reader) *BitcoinAddress {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterBitcoinAddress) Lower(value *BitcoinAddress) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*BitcoinAddress")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterBitcoinAddress) Write(writer io.Writer, value *BitcoinAddress) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerBitcoinAddress struct{}

func (_ FfiDestroyerBitcoinAddress) Destroy(value *BitcoinAddress) {
	value.Destroy()
}

// Wrapper over [`elements::BlockHeader`]
type BlockHeaderInterface interface {
	// Get the block hash
	BlockHash() string
	// Get the block height
	Height() uint32
	// Get the merkle root
	MerkleRoot() string
	// Get the previous block hash
	PrevBlockhash() string
	// Get the block timestamp
	Time() uint32
	// Get the block version
	Version() uint32
}

// Wrapper over [`elements::BlockHeader`]
type BlockHeader struct {
	ffiObject FfiObject
}

// Get the block hash
func (_self *BlockHeader) BlockHash() string {
	_pointer := _self.ffiObject.incrementPointer("*BlockHeader")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_blockheader_block_hash(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the block height
func (_self *BlockHeader) Height() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*BlockHeader")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_blockheader_height(
			_pointer, _uniffiStatus)
	}))
}

// Get the merkle root
func (_self *BlockHeader) MerkleRoot() string {
	_pointer := _self.ffiObject.incrementPointer("*BlockHeader")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_blockheader_merkle_root(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the previous block hash
func (_self *BlockHeader) PrevBlockhash() string {
	_pointer := _self.ffiObject.incrementPointer("*BlockHeader")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_blockheader_prev_blockhash(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the block timestamp
func (_self *BlockHeader) Time() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*BlockHeader")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_blockheader_time(
			_pointer, _uniffiStatus)
	}))
}

// Get the block version
func (_self *BlockHeader) Version() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*BlockHeader")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_blockheader_version(
			_pointer, _uniffiStatus)
	}))
}
func (object *BlockHeader) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterBlockHeader struct{}

var FfiConverterBlockHeaderINSTANCE = FfiConverterBlockHeader{}

func (c FfiConverterBlockHeader) Lift(pointer unsafe.Pointer) *BlockHeader {
	result := &BlockHeader{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_blockheader(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_blockheader(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*BlockHeader).Destroy)
	return result
}

func (c FfiConverterBlockHeader) Read(reader io.Reader) *BlockHeader {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterBlockHeader) Lower(value *BlockHeader) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*BlockHeader")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterBlockHeader) Write(writer io.Writer, value *BlockHeader) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerBlockHeader struct{}

func (_ FfiDestroyerBlockHeader) Destroy(value *BlockHeader) {
	value.Destroy()
}

// Represents a syntactically and semantically correct lightning BOLT11 invoice.
type Bolt11InvoiceInterface interface {
	// Returns the amount in millisatoshis if present, None if it's an "any amount" invoice
	AmountMilliSatoshis() *uint64
	// Returns the expiry time in seconds (default is 3600 seconds / 1 hour if not specified)
	ExpiryTime() uint64
	// Returns the invoice description as a string
	InvoiceDescription() string
	// Returns the minimum CLTV expiry delta
	MinFinalCltvExpiryDelta() uint64
	// Returns the network (bitcoin, testnet, signet, regtest)
	Network() string
	// Returns the payee's public key if present as a hex string
	PayeePubKey() *string
	// Returns the payment hash as a hex string
	PaymentHash() string
	// Returns the payment secret as a debug string
	PaymentSecret() string
	// Returns the invoice timestamp as seconds since Unix epoch
	Timestamp() uint64
}

// Represents a syntactically and semantically correct lightning BOLT11 invoice.
type Bolt11Invoice struct {
	ffiObject FfiObject
}

// Construct a Bolt11Invoice from a string
func NewBolt11Invoice(s string) (*Bolt11Invoice, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_bolt11invoice_new(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Bolt11Invoice
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBolt11InvoiceINSTANCE.Lift(_uniffiRV), nil
	}
}

// Returns the amount in millisatoshis if present, None if it's an "any amount" invoice
func (_self *Bolt11Invoice) AmountMilliSatoshis() *uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Bolt11Invoice")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bolt11invoice_amount_milli_satoshis(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the expiry time in seconds (default is 3600 seconds / 1 hour if not specified)
func (_self *Bolt11Invoice) ExpiryTime() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Bolt11Invoice")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_bolt11invoice_expiry_time(
			_pointer, _uniffiStatus)
	}))
}

// Returns the invoice description as a string
func (_self *Bolt11Invoice) InvoiceDescription() string {
	_pointer := _self.ffiObject.incrementPointer("*Bolt11Invoice")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bolt11invoice_invoice_description(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the minimum CLTV expiry delta
func (_self *Bolt11Invoice) MinFinalCltvExpiryDelta() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Bolt11Invoice")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_bolt11invoice_min_final_cltv_expiry_delta(
			_pointer, _uniffiStatus)
	}))
}

// Returns the network (bitcoin, testnet, signet, regtest)
func (_self *Bolt11Invoice) Network() string {
	_pointer := _self.ffiObject.incrementPointer("*Bolt11Invoice")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bolt11invoice_network(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the payee's public key if present as a hex string
func (_self *Bolt11Invoice) PayeePubKey() *string {
	_pointer := _self.ffiObject.incrementPointer("*Bolt11Invoice")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bolt11invoice_payee_pub_key(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the payment hash as a hex string
func (_self *Bolt11Invoice) PaymentHash() string {
	_pointer := _self.ffiObject.incrementPointer("*Bolt11Invoice")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bolt11invoice_payment_hash(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the payment secret as a debug string
func (_self *Bolt11Invoice) PaymentSecret() string {
	_pointer := _self.ffiObject.incrementPointer("*Bolt11Invoice")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bolt11invoice_payment_secret(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the invoice timestamp as seconds since Unix epoch
func (_self *Bolt11Invoice) Timestamp() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Bolt11Invoice")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_bolt11invoice_timestamp(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Bolt11Invoice) String() string {
	_pointer := _self.ffiObject.incrementPointer("*Bolt11Invoice")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_bolt11invoice_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *Bolt11Invoice) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterBolt11Invoice struct{}

var FfiConverterBolt11InvoiceINSTANCE = FfiConverterBolt11Invoice{}

func (c FfiConverterBolt11Invoice) Lift(pointer unsafe.Pointer) *Bolt11Invoice {
	result := &Bolt11Invoice{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_bolt11invoice(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_bolt11invoice(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Bolt11Invoice).Destroy)
	return result
}

func (c FfiConverterBolt11Invoice) Read(reader io.Reader) *Bolt11Invoice {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterBolt11Invoice) Lower(value *Bolt11Invoice) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Bolt11Invoice")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterBolt11Invoice) Write(writer io.Writer, value *Bolt11Invoice) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerBolt11Invoice struct{}

func (_ FfiDestroyerBolt11Invoice) Destroy(value *Bolt11Invoice) {
	value.Destroy()
}

// A session to pay and receive lightning payments.
//
// Lightning payments are done via LBTC swaps using Boltz.
//
// See `BoltzSessionBuilder` for various options to configure the session.
type BoltzSessionInterface interface {
	// Create an onchain swap to convert BTC to LBTC
	BtcToLbtc(amount uint64, refundAddress *BitcoinAddress, claimAddress *Address, webhook **WebHook) (*LockupResponse, error)
	// Get the list of completed swap IDs from the store
	//
	// Returns an error if no store is configured.
	CompletedSwapIds() ([]string, error)
	// Fetch informations, such as min and max amounts, about the reverse and submarine pairs from the boltz api.
	FetchSwapsInfo() (string, error)
	// Get the raw swap data (JSON) for a specific swap ID from the store
	//
	// Returns `None` if no store is configured or the swap doesn't exist.
	GetSwapData(swapId string) (*string, error)
	// Create a new invoice for a given amount and a claim address to receive the payment
	Invoice(amount uint64, description *string, claimAddress *Address, webhook **WebHook) (*InvoiceResponse, error)
	// Create an onchain swap to convert LBTC to BTC
	LbtcToBtc(amount uint64, refundAddress *Address, claimAddress *BitcoinAddress, webhook **WebHook) (*LockupResponse, error)
	// Get the next index to use for deriving keypairs
	NextIndexToUse() uint32
	// Get the list of pending swap IDs from the store
	//
	// Returns an error if no store is configured.
	PendingSwapIds() ([]string, error)
	// Prepare to pay a bolt11 invoice
	PreparePay(lightningPayment *LightningPayment, refundAddress *Address, webhook **WebHook) (*PreparePayResponse, error)
	// Create a quote builder for calculating swap fees
	//
	// This uses the cached pairs data from session initialization.
	//
	// # Example
	// ```ignore
	// let builder = session.quote(25000);
	// builder.send(SwapAsset::Lightning);
	// builder.receive(SwapAsset::Liquid);
	// let quote = builder.build()?;
	// ```
	Quote(sendAmount uint64) *QuoteBuilder
	// Create a quote builder for calculating send amount from desired receive amount
	//
	// This is the inverse of [`BoltzSession::quote()`] - given the amount you want
	// to receive, it calculates how much you need to send.
	//
	// # Example
	// ```ignore
	// let builder = session.quote_receive(24887);
	// builder.send(SwapAsset::Lightning);
	// builder.receive(SwapAsset::Liquid);
	// let quote = builder.build()?;
	// // quote.send_amount will be 25000
	// ```
	QuoteReceive(receiveAmount uint64) *QuoteBuilder
	// Refresh the cached pairs data from the Boltz API
	//
	// This updates the internal cache used by [`BoltzSession::quote()`].
	// Call this if you need up-to-date fee information after the session was created.
	RefreshSwapInfo() error
	// Remove a swap from the store
	//
	// Returns an error if no store is configured.
	RemoveSwap(swapId string) error
	// Generate a rescue file with lightning session mnemonic.
	//
	// The rescue file is a JSON file that contains the swaps mnemonic.
	// It can be used on the Boltz web app to bring non terminated swaps to completition.
	RescueFile() (string, error)
	// From the swaps returned by the boltz api via [`BoltzSession::swap_restore`]:
	//
	// - filter the BTC to LBTC swaps
	// - add information from the session
	// - return typed data
	//
	// The claim and refund addresses don't need to be the same used when creating the swap.
	RestorableBtcToLbtcSwaps(swapList *SwapList, claimAddress *Address, refundAddress *BitcoinAddress) ([]string, error)
	// From the swaps returned by the boltz api via [`BoltzSession::swap_restore`]:
	//
	// - filter the LBTC to BTC swaps
	// - add information from the session
	// - return typed data
	//
	// The claim and refund addresses don't need to be the same used when creating the swap.
	RestorableLbtcToBtcSwaps(swapList *SwapList, claimAddress *BitcoinAddress, refundAddress *Address) ([]string, error)
	// From the swaps returned by the boltz api via [`BoltzSession::swap_restore`]:
	//
	// - filter the reverse swaps
	// - add information from the session
	// - return typed data
	//
	// The claim address doesn't need to be the same used when creating the swap.
	RestorableReverseSwaps(swapList *SwapList, claimAddress *Address) ([]string, error)
	// From the swaps returned by the boltz api via [`BoltzSession::swap_restore`]:
	//
	// - filter the submarine swaps
	// - add information from the session
	// - return typed data
	//
	// The refund address doesn't need to be the same used when creating the swap.
	RestorableSubmarineSwaps(swapList *SwapList, refundAddress *Address) ([]string, error)
	// Restore an invoice flow from its serialized data see `InvoiceResponse::serialize`
	RestoreInvoice(data string) (*InvoiceResponse, error)
	// Restore an onchain swap from its serialized data see `LockupResponse::serialize`
	RestoreLockup(data string) (*LockupResponse, error)
	// Restore a payment from its serialized data see `PreparePayResponse::serialize`
	RestorePreparePay(data string) (*PreparePayResponse, error)
	// Set the next index to use for deriving keypairs
	//
	// This may be necessary to handle multiple sessions with the same mnemonic.
	SetNextIndexToUse(nextIndexToUse uint32)
	// Returns a the list of all the swaps ever done with the session mnemonic.
	//
	// The object returned can be converted to a json String with toString()
	SwapRestore() (*SwapList, error)
}

// A session to pay and receive lightning payments.
//
// Lightning payments are done via LBTC swaps using Boltz.
//
// See `BoltzSessionBuilder` for various options to configure the session.
type BoltzSession struct {
	ffiObject FfiObject
}

// Create the lightning session with default settings
//
// This uses default timeout and generates a random mnemonic.
// For custom configuration, use [`BoltzSession::from_builder()`] instead.
func NewBoltzSession(network *Network, client *AnyClient) (*BoltzSession, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_boltzsession_new(FfiConverterNetworkINSTANCE.Lower(network), FfiConverterAnyClientINSTANCE.Lower(client), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *BoltzSession
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBoltzSessionINSTANCE.Lift(_uniffiRV), nil
	}
}

// Create the lightning session from a builder
func BoltzSessionFromBuilder(builder BoltzSessionBuilder) (*BoltzSession, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_boltzsession_from_builder(FfiConverterBoltzSessionBuilderINSTANCE.Lower(builder), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *BoltzSession
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBoltzSessionINSTANCE.Lift(_uniffiRV), nil
	}
}

// Create an onchain swap to convert BTC to LBTC
func (_self *BoltzSession) BtcToLbtc(amount uint64, refundAddress *BitcoinAddress, claimAddress *Address, webhook **WebHook) (*LockupResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_boltzsession_btc_to_lbtc(
			_pointer, FfiConverterUint64INSTANCE.Lower(amount), FfiConverterBitcoinAddressINSTANCE.Lower(refundAddress), FfiConverterAddressINSTANCE.Lower(claimAddress), FfiConverterOptionalWebHookINSTANCE.Lower(webhook), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LockupResponse
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLockupResponseINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the list of completed swap IDs from the store
//
// Returns an error if no store is configured.
func (_self *BoltzSession) CompletedSwapIds() ([]string, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_boltzsession_completed_swap_ids(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// Fetch informations, such as min and max amounts, about the reverse and submarine pairs from the boltz api.
func (_self *BoltzSession) FetchSwapsInfo() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_boltzsession_fetch_swaps_info(
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

// Get the raw swap data (JSON) for a specific swap ID from the store
//
// Returns `None` if no store is configured or the swap doesn't exist.
func (_self *BoltzSession) GetSwapData(swapId string) (*string, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_boltzsession_get_swap_data(
				_pointer, FfiConverterStringINSTANCE.Lower(swapId), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// Create a new invoice for a given amount and a claim address to receive the payment
func (_self *BoltzSession) Invoice(amount uint64, description *string, claimAddress *Address, webhook **WebHook) (*InvoiceResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_boltzsession_invoice(
			_pointer, FfiConverterUint64INSTANCE.Lower(amount), FfiConverterOptionalStringINSTANCE.Lower(description), FfiConverterAddressINSTANCE.Lower(claimAddress), FfiConverterOptionalWebHookINSTANCE.Lower(webhook), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *InvoiceResponse
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterInvoiceResponseINSTANCE.Lift(_uniffiRV), nil
	}
}

// Create an onchain swap to convert LBTC to BTC
func (_self *BoltzSession) LbtcToBtc(amount uint64, refundAddress *Address, claimAddress *BitcoinAddress, webhook **WebHook) (*LockupResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_boltzsession_lbtc_to_btc(
			_pointer, FfiConverterUint64INSTANCE.Lower(amount), FfiConverterAddressINSTANCE.Lower(refundAddress), FfiConverterBitcoinAddressINSTANCE.Lower(claimAddress), FfiConverterOptionalWebHookINSTANCE.Lower(webhook), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LockupResponse
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLockupResponseINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the next index to use for deriving keypairs
func (_self *BoltzSession) NextIndexToUse() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_boltzsession_next_index_to_use(
			_pointer, _uniffiStatus)
	}))
}

// Get the list of pending swap IDs from the store
//
// Returns an error if no store is configured.
func (_self *BoltzSession) PendingSwapIds() ([]string, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_boltzsession_pending_swap_ids(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// Prepare to pay a bolt11 invoice
func (_self *BoltzSession) PreparePay(lightningPayment *LightningPayment, refundAddress *Address, webhook **WebHook) (*PreparePayResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_boltzsession_prepare_pay(
			_pointer, FfiConverterLightningPaymentINSTANCE.Lower(lightningPayment), FfiConverterAddressINSTANCE.Lower(refundAddress), FfiConverterOptionalWebHookINSTANCE.Lower(webhook), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *PreparePayResponse
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPreparePayResponseINSTANCE.Lift(_uniffiRV), nil
	}
}

// Create a quote builder for calculating swap fees
//
// This uses the cached pairs data from session initialization.
//
// # Example
// ```ignore
// let builder = session.quote(25000);
// builder.send(SwapAsset::Lightning);
// builder.receive(SwapAsset::Liquid);
// let quote = builder.build()?;
// ```
func (_self *BoltzSession) Quote(sendAmount uint64) *QuoteBuilder {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterQuoteBuilderINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_boltzsession_quote(
			_pointer, FfiConverterUint64INSTANCE.Lower(sendAmount), _uniffiStatus)
	}))
}

// Create a quote builder for calculating send amount from desired receive amount
//
// This is the inverse of [`BoltzSession::quote()`] - given the amount you want
// to receive, it calculates how much you need to send.
//
// # Example
// ```ignore
// let builder = session.quote_receive(24887);
// builder.send(SwapAsset::Lightning);
// builder.receive(SwapAsset::Liquid);
// let quote = builder.build()?;
// // quote.send_amount will be 25000
// ```
func (_self *BoltzSession) QuoteReceive(receiveAmount uint64) *QuoteBuilder {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterQuoteBuilderINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_boltzsession_quote_receive(
			_pointer, FfiConverterUint64INSTANCE.Lower(receiveAmount), _uniffiStatus)
	}))
}

// Refresh the cached pairs data from the Boltz API
//
// This updates the internal cache used by [`BoltzSession::quote()`].
// Call this if you need up-to-date fee information after the session was created.
func (_self *BoltzSession) RefreshSwapInfo() error {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_boltzsession_refresh_swap_info(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Remove a swap from the store
//
// Returns an error if no store is configured.
func (_self *BoltzSession) RemoveSwap(swapId string) error {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_boltzsession_remove_swap(
			_pointer, FfiConverterStringINSTANCE.Lower(swapId), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Generate a rescue file with lightning session mnemonic.
//
// The rescue file is a JSON file that contains the swaps mnemonic.
// It can be used on the Boltz web app to bring non terminated swaps to completition.
func (_self *BoltzSession) RescueFile() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_boltzsession_rescue_file(
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

// From the swaps returned by the boltz api via [`BoltzSession::swap_restore`]:
//
// - filter the BTC to LBTC swaps
// - add information from the session
// - return typed data
//
// The claim and refund addresses don't need to be the same used when creating the swap.
func (_self *BoltzSession) RestorableBtcToLbtcSwaps(swapList *SwapList, claimAddress *Address, refundAddress *BitcoinAddress) ([]string, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_boltzsession_restorable_btc_to_lbtc_swaps(
				_pointer, FfiConverterSwapListINSTANCE.Lower(swapList), FfiConverterAddressINSTANCE.Lower(claimAddress), FfiConverterBitcoinAddressINSTANCE.Lower(refundAddress), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// From the swaps returned by the boltz api via [`BoltzSession::swap_restore`]:
//
// - filter the LBTC to BTC swaps
// - add information from the session
// - return typed data
//
// The claim and refund addresses don't need to be the same used when creating the swap.
func (_self *BoltzSession) RestorableLbtcToBtcSwaps(swapList *SwapList, claimAddress *BitcoinAddress, refundAddress *Address) ([]string, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_boltzsession_restorable_lbtc_to_btc_swaps(
				_pointer, FfiConverterSwapListINSTANCE.Lower(swapList), FfiConverterBitcoinAddressINSTANCE.Lower(claimAddress), FfiConverterAddressINSTANCE.Lower(refundAddress), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// From the swaps returned by the boltz api via [`BoltzSession::swap_restore`]:
//
// - filter the reverse swaps
// - add information from the session
// - return typed data
//
// The claim address doesn't need to be the same used when creating the swap.
func (_self *BoltzSession) RestorableReverseSwaps(swapList *SwapList, claimAddress *Address) ([]string, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_boltzsession_restorable_reverse_swaps(
				_pointer, FfiConverterSwapListINSTANCE.Lower(swapList), FfiConverterAddressINSTANCE.Lower(claimAddress), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// From the swaps returned by the boltz api via [`BoltzSession::swap_restore`]:
//
// - filter the submarine swaps
// - add information from the session
// - return typed data
//
// The refund address doesn't need to be the same used when creating the swap.
func (_self *BoltzSession) RestorableSubmarineSwaps(swapList *SwapList, refundAddress *Address) ([]string, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_boltzsession_restorable_submarine_swaps(
				_pointer, FfiConverterSwapListINSTANCE.Lower(swapList), FfiConverterAddressINSTANCE.Lower(refundAddress), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// Restore an invoice flow from its serialized data see `InvoiceResponse::serialize`
func (_self *BoltzSession) RestoreInvoice(data string) (*InvoiceResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_boltzsession_restore_invoice(
			_pointer, FfiConverterStringINSTANCE.Lower(data), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *InvoiceResponse
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterInvoiceResponseINSTANCE.Lift(_uniffiRV), nil
	}
}

// Restore an onchain swap from its serialized data see `LockupResponse::serialize`
func (_self *BoltzSession) RestoreLockup(data string) (*LockupResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_boltzsession_restore_lockup(
			_pointer, FfiConverterStringINSTANCE.Lower(data), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LockupResponse
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLockupResponseINSTANCE.Lift(_uniffiRV), nil
	}
}

// Restore a payment from its serialized data see `PreparePayResponse::serialize`
func (_self *BoltzSession) RestorePreparePay(data string) (*PreparePayResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_boltzsession_restore_prepare_pay(
			_pointer, FfiConverterStringINSTANCE.Lower(data), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *PreparePayResponse
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPreparePayResponseINSTANCE.Lift(_uniffiRV), nil
	}
}

// Set the next index to use for deriving keypairs
//
// This may be necessary to handle multiple sessions with the same mnemonic.
func (_self *BoltzSession) SetNextIndexToUse(nextIndexToUse uint32) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_boltzsession_set_next_index_to_use(
			_pointer, FfiConverterUint32INSTANCE.Lower(nextIndexToUse), _uniffiStatus)
		return false
	})
}

// Returns a the list of all the swaps ever done with the session mnemonic.
//
// The object returned can be converted to a json String with toString()
func (_self *BoltzSession) SwapRestore() (*SwapList, error) {
	_pointer := _self.ffiObject.incrementPointer("*BoltzSession")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_boltzsession_swap_restore(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *SwapList
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSwapListINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *BoltzSession) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterBoltzSession struct{}

var FfiConverterBoltzSessionINSTANCE = FfiConverterBoltzSession{}

func (c FfiConverterBoltzSession) Lift(pointer unsafe.Pointer) *BoltzSession {
	result := &BoltzSession{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_boltzsession(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_boltzsession(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*BoltzSession).Destroy)
	return result
}

func (c FfiConverterBoltzSession) Read(reader io.Reader) *BoltzSession {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterBoltzSession) Lower(value *BoltzSession) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*BoltzSession")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterBoltzSession) Write(writer io.Writer, value *BoltzSession) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerBoltzSession struct{}

func (_ FfiDestroyerBoltzSession) Destroy(value *BoltzSession) {
	value.Destroy()
}

// Wrapper over [`lwk_wollet::Contract`]
type ContractInterface interface {
}

// Wrapper over [`lwk_wollet::Contract`]
type Contract struct {
	ffiObject FfiObject
}

// Construct a Contract object
func NewContract(domain string, issuerPubkey string, name string, precision uint8, ticker string, version uint8) (*Contract, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_contract_new(FfiConverterStringINSTANCE.Lower(domain), FfiConverterStringINSTANCE.Lower(issuerPubkey), FfiConverterStringINSTANCE.Lower(name), FfiConverterUint8INSTANCE.Lower(precision), FfiConverterStringINSTANCE.Lower(ticker), FfiConverterUint8INSTANCE.Lower(version), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Contract
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterContractINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Contract) String() string {
	_pointer := _self.ffiObject.incrementPointer("*Contract")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_contract_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *Contract) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterContract struct{}

var FfiConverterContractINSTANCE = FfiConverterContract{}

func (c FfiConverterContract) Lift(pointer unsafe.Pointer) *Contract {
	result := &Contract{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_contract(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_contract(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Contract).Destroy)
	return result
}

func (c FfiConverterContract) Read(reader io.Reader) *Contract {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterContract) Lower(value *Contract) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Contract")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterContract) Write(writer io.Writer, value *Contract) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerContract struct{}

func (_ FfiDestroyerContract) Destroy(value *Contract) {
	value.Destroy()
}

// Currency code as defined by ISO 4217
type CurrencyCodeInterface interface {
	// Get the alpha3 code (e.g., "USD")
	Alpha3() string
	// Get the number of decimals for this currency
	Exp() int8
	// Get the currency name (e.g., "US Dollar")
	Name() string
}

// Currency code as defined by ISO 4217
type CurrencyCode struct {
	ffiObject FfiObject
}

// Create a CurrencyCode from an alpha3 code (e.g., "USD", "EUR")
func NewCurrencyCode(alpha3 string) (*CurrencyCode, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_currencycode_new(FfiConverterStringINSTANCE.Lower(alpha3), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *CurrencyCode
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterCurrencyCodeINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the alpha3 code (e.g., "USD")
func (_self *CurrencyCode) Alpha3() string {
	_pointer := _self.ffiObject.incrementPointer("*CurrencyCode")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_currencycode_alpha3(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the number of decimals for this currency
func (_self *CurrencyCode) Exp() int8 {
	_pointer := _self.ffiObject.incrementPointer("*CurrencyCode")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterInt8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_currencycode_exp(
			_pointer, _uniffiStatus)
	}))
}

// Get the currency name (e.g., "US Dollar")
func (_self *CurrencyCode) Name() string {
	_pointer := _self.ffiObject.incrementPointer("*CurrencyCode")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_currencycode_name(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *CurrencyCode) String() string {
	_pointer := _self.ffiObject.incrementPointer("*CurrencyCode")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_currencycode_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *CurrencyCode) Eq(other *CurrencyCode) bool {
	_pointer := _self.ffiObject.incrementPointer("*CurrencyCode")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_currencycode_uniffi_trait_eq_eq(
			_pointer, FfiConverterCurrencyCodeINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *CurrencyCode) Ne(other *CurrencyCode) bool {
	_pointer := _self.ffiObject.incrementPointer("*CurrencyCode")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_currencycode_uniffi_trait_eq_ne(
			_pointer, FfiConverterCurrencyCodeINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (object *CurrencyCode) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterCurrencyCode struct{}

var FfiConverterCurrencyCodeINSTANCE = FfiConverterCurrencyCode{}

func (c FfiConverterCurrencyCode) Lift(pointer unsafe.Pointer) *CurrencyCode {
	result := &CurrencyCode{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_currencycode(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_currencycode(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*CurrencyCode).Destroy)
	return result
}

func (c FfiConverterCurrencyCode) Read(reader io.Reader) *CurrencyCode {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterCurrencyCode) Lower(value *CurrencyCode) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*CurrencyCode")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterCurrencyCode) Write(writer io.Writer, value *CurrencyCode) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerCurrencyCode struct{}

func (_ FfiDestroyerCurrencyCode) Destroy(value *CurrencyCode) {
	value.Destroy()
}

// A client to issue TCP requests to an electrum server.
type ElectrumClientInterface interface {
	// Broadcast a transaction to the network so that a miner can include it in a block.
	Broadcast(tx *Transaction) (*Txid, error)
	// Scan the blockchain for the scripts generated by a watch-only wallet
	//
	// This method scans both external and internal address chains, stopping after finding
	// 20 consecutive unused addresses (the gap limit) as recommended by
	// [BIP44](https://github.com/bitcoin/bips/blob/master/bip-0044.mediawiki#address-gap-limit).
	//
	// Returns `Some(Update)` if any changes were found during scanning, or `None` if no changes
	// were detected.
	//
	// To scan beyond the gap limit use `full_scan_to_index()` instead.
	FullScan(wollet *Wollet) (**Update, error)
	// Scan the blockchain for the scripts generated by a watch-only wallet up to a specified derivation index
	//
	// While `full_scan()` stops after finding 20 consecutive unused addresses (the gap limit),
	// this method will scan at least up to the given derivation index. This is useful to prevent
	// missing funds in cases where outputs exist beyond the gap limit.
	//
	// Will scan both external and internal address chains up to the given index for maximum safety,
	// even though internal addresses may not need such deep scanning.
	//
	// If transactions are found beyond the gap limit during this scan, subsequent calls to
	// `full_scan()` will automatically scan up to the highest used index, preventing any
	// previously-found transactions from being missed.
	FullScanToIndex(wollet *Wollet, index uint32) (**Update, error)
	// Fetch the transaction with the given id
	GetTx(txid *Txid) (*Transaction, error)
	// Ping the Electrum server
	Ping() error
	// Return the current tip of the blockchain
	Tip() (*BlockHeader, error)
}

// A client to issue TCP requests to an electrum server.
type ElectrumClient struct {
	ffiObject FfiObject
}

// Construct an Electrum client
func NewElectrumClient(electrumUrl string, tls bool, validateDomain bool) (*ElectrumClient, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_electrumclient_new(FfiConverterStringINSTANCE.Lower(electrumUrl), FfiConverterBoolINSTANCE.Lower(tls), FfiConverterBoolINSTANCE.Lower(validateDomain), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ElectrumClient
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterElectrumClientINSTANCE.Lift(_uniffiRV), nil
	}
}

// Construct an electrum client from an Electrum URL
func ElectrumClientFromUrl(electrumUrl string) (*ElectrumClient, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_electrumclient_from_url(FfiConverterStringINSTANCE.Lower(electrumUrl), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ElectrumClient
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterElectrumClientINSTANCE.Lift(_uniffiRV), nil
	}
}

// Broadcast a transaction to the network so that a miner can include it in a block.
func (_self *ElectrumClient) Broadcast(tx *Transaction) (*Txid, error) {
	_pointer := _self.ffiObject.incrementPointer("*ElectrumClient")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_electrumclient_broadcast(
			_pointer, FfiConverterTransactionINSTANCE.Lower(tx), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Txid
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxidINSTANCE.Lift(_uniffiRV), nil
	}
}

// Scan the blockchain for the scripts generated by a watch-only wallet
//
// This method scans both external and internal address chains, stopping after finding
// 20 consecutive unused addresses (the gap limit) as recommended by
// [BIP44](https://github.com/bitcoin/bips/blob/master/bip-0044.mediawiki#address-gap-limit).
//
// Returns `Some(Update)` if any changes were found during scanning, or `None` if no changes
// were detected.
//
// To scan beyond the gap limit use `full_scan_to_index()` instead.
func (_self *ElectrumClient) FullScan(wollet *Wollet) (**Update, error) {
	_pointer := _self.ffiObject.incrementPointer("*ElectrumClient")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_electrumclient_full_scan(
				_pointer, FfiConverterWolletINSTANCE.Lower(wollet), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue **Update
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalUpdateINSTANCE.Lift(_uniffiRV), nil
	}
}

// Scan the blockchain for the scripts generated by a watch-only wallet up to a specified derivation index
//
// While `full_scan()` stops after finding 20 consecutive unused addresses (the gap limit),
// this method will scan at least up to the given derivation index. This is useful to prevent
// missing funds in cases where outputs exist beyond the gap limit.
//
// Will scan both external and internal address chains up to the given index for maximum safety,
// even though internal addresses may not need such deep scanning.
//
// If transactions are found beyond the gap limit during this scan, subsequent calls to
// `full_scan()` will automatically scan up to the highest used index, preventing any
// previously-found transactions from being missed.
func (_self *ElectrumClient) FullScanToIndex(wollet *Wollet, index uint32) (**Update, error) {
	_pointer := _self.ffiObject.incrementPointer("*ElectrumClient")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_electrumclient_full_scan_to_index(
				_pointer, FfiConverterWolletINSTANCE.Lower(wollet), FfiConverterUint32INSTANCE.Lower(index), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue **Update
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalUpdateINSTANCE.Lift(_uniffiRV), nil
	}
}

// Fetch the transaction with the given id
func (_self *ElectrumClient) GetTx(txid *Txid) (*Transaction, error) {
	_pointer := _self.ffiObject.incrementPointer("*ElectrumClient")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_electrumclient_get_tx(
			_pointer, FfiConverterTxidINSTANCE.Lower(txid), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Transaction
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTransactionINSTANCE.Lift(_uniffiRV), nil
	}
}

// Ping the Electrum server
func (_self *ElectrumClient) Ping() error {
	_pointer := _self.ffiObject.incrementPointer("*ElectrumClient")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_electrumclient_ping(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Return the current tip of the blockchain
func (_self *ElectrumClient) Tip() (*BlockHeader, error) {
	_pointer := _self.ffiObject.incrementPointer("*ElectrumClient")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_electrumclient_tip(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *BlockHeader
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBlockHeaderINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *ElectrumClient) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterElectrumClient struct{}

var FfiConverterElectrumClientINSTANCE = FfiConverterElectrumClient{}

func (c FfiConverterElectrumClient) Lift(pointer unsafe.Pointer) *ElectrumClient {
	result := &ElectrumClient{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_electrumclient(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_electrumclient(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ElectrumClient).Destroy)
	return result
}

func (c FfiConverterElectrumClient) Read(reader io.Reader) *ElectrumClient {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterElectrumClient) Lower(value *ElectrumClient) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*ElectrumClient")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterElectrumClient) Write(writer io.Writer, value *ElectrumClient) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerElectrumClient struct{}

func (_ FfiDestroyerElectrumClient) Destroy(value *ElectrumClient) {
	value.Destroy()
}

// A blockchain backend implementation based on the
// [esplora HTTP API](https://github.com/blockstream/esplora/blob/master/API.md)
// But can also use the [waterfalls](https://github.com/RCasatta/waterfalls) endpoint to
// speed up the scan if supported by the server.
type EsploraClientInterface interface {
	// Broadcast a transaction to the network so that a miner can include it in a block.
	Broadcast(tx *Transaction) (*Txid, error)
	// Scan the blockchain for the scripts generated by a watch-only wallet
	//
	// This method scans both external and internal address chains, stopping after finding
	// 20 consecutive unused addresses (the gap limit) as recommended by
	// [BIP44](https://github.com/bitcoin/bips/blob/master/bip-0044.mediawiki#address-gap-limit).
	//
	// Returns `Some(Update)` if any changes were found during scanning, or `None` if no changes
	// were detected.
	//
	// To scan beyond the gap limit use `full_scan_to_index()` instead.
	FullScan(wollet *Wollet) (**Update, error)
	// Scan the blockchain for the scripts generated by a watch-only wallet up to a specified derivation index
	//
	// While `full_scan()` stops after finding 20 consecutive unused addresses (the gap limit),
	// this method will scan at least up to the given derivation index. This is useful to prevent
	// missing funds in cases where outputs exist beyond the gap limit.
	//
	// Will scan both external and internal address chains up to the given index for maximum safety,
	// even though internal addresses may not need such deep scanning.
	//
	// If transactions are found beyond the gap limit during this scan, subsequent calls to
	// `full_scan()` will automatically scan up to the highest used index, preventing any
	// previously-found transactions from being missed.
	FullScanToIndex(wollet *Wollet, index uint32) (**Update, error)
	// See [`BlockchainBackend::tip`]
	Tip() (*BlockHeader, error)
}

// A blockchain backend implementation based on the
// [esplora HTTP API](https://github.com/blockstream/esplora/blob/master/API.md)
// But can also use the [waterfalls](https://github.com/RCasatta/waterfalls) endpoint to
// speed up the scan if supported by the server.
type EsploraClient struct {
	ffiObject FfiObject
}

// Construct an Esplora Client
func NewEsploraClient(url string, network *Network) (*EsploraClient, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_esploraclient_new(FfiConverterStringINSTANCE.Lower(url), FfiConverterNetworkINSTANCE.Lower(network), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *EsploraClient
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterEsploraClientINSTANCE.Lift(_uniffiRV), nil
	}
}

// Construct an Esplora Client from an `EsploraClientBuilder`
func EsploraClientFromBuilder(builder EsploraClientBuilder) (*EsploraClient, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_esploraclient_from_builder(FfiConverterEsploraClientBuilderINSTANCE.Lower(builder), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *EsploraClient
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterEsploraClientINSTANCE.Lift(_uniffiRV), nil
	}
}

// Construct an Esplora Client using Waterfalls endpoint
func EsploraClientNewWaterfalls(url string, network *Network) (*EsploraClient, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_esploraclient_new_waterfalls(FfiConverterStringINSTANCE.Lower(url), FfiConverterNetworkINSTANCE.Lower(network), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *EsploraClient
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterEsploraClientINSTANCE.Lift(_uniffiRV), nil
	}
}

// Broadcast a transaction to the network so that a miner can include it in a block.
func (_self *EsploraClient) Broadcast(tx *Transaction) (*Txid, error) {
	_pointer := _self.ffiObject.incrementPointer("*EsploraClient")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_esploraclient_broadcast(
			_pointer, FfiConverterTransactionINSTANCE.Lower(tx), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Txid
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxidINSTANCE.Lift(_uniffiRV), nil
	}
}

// Scan the blockchain for the scripts generated by a watch-only wallet
//
// This method scans both external and internal address chains, stopping after finding
// 20 consecutive unused addresses (the gap limit) as recommended by
// [BIP44](https://github.com/bitcoin/bips/blob/master/bip-0044.mediawiki#address-gap-limit).
//
// Returns `Some(Update)` if any changes were found during scanning, or `None` if no changes
// were detected.
//
// To scan beyond the gap limit use `full_scan_to_index()` instead.
func (_self *EsploraClient) FullScan(wollet *Wollet) (**Update, error) {
	_pointer := _self.ffiObject.incrementPointer("*EsploraClient")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_esploraclient_full_scan(
				_pointer, FfiConverterWolletINSTANCE.Lower(wollet), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue **Update
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalUpdateINSTANCE.Lift(_uniffiRV), nil
	}
}

// Scan the blockchain for the scripts generated by a watch-only wallet up to a specified derivation index
//
// While `full_scan()` stops after finding 20 consecutive unused addresses (the gap limit),
// this method will scan at least up to the given derivation index. This is useful to prevent
// missing funds in cases where outputs exist beyond the gap limit.
//
// Will scan both external and internal address chains up to the given index for maximum safety,
// even though internal addresses may not need such deep scanning.
//
// If transactions are found beyond the gap limit during this scan, subsequent calls to
// `full_scan()` will automatically scan up to the highest used index, preventing any
// previously-found transactions from being missed.
func (_self *EsploraClient) FullScanToIndex(wollet *Wollet, index uint32) (**Update, error) {
	_pointer := _self.ffiObject.incrementPointer("*EsploraClient")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_esploraclient_full_scan_to_index(
				_pointer, FfiConverterWolletINSTANCE.Lower(wollet), FfiConverterUint32INSTANCE.Lower(index), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue **Update
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalUpdateINSTANCE.Lift(_uniffiRV), nil
	}
}

// See [`BlockchainBackend::tip`]
func (_self *EsploraClient) Tip() (*BlockHeader, error) {
	_pointer := _self.ffiObject.incrementPointer("*EsploraClient")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_esploraclient_tip(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *BlockHeader
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBlockHeaderINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *EsploraClient) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterEsploraClient struct{}

var FfiConverterEsploraClientINSTANCE = FfiConverterEsploraClient{}

func (c FfiConverterEsploraClient) Lift(pointer unsafe.Pointer) *EsploraClient {
	result := &EsploraClient{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_esploraclient(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_esploraclient(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*EsploraClient).Destroy)
	return result
}

func (c FfiConverterEsploraClient) Read(reader io.Reader) *EsploraClient {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterEsploraClient) Lower(value *EsploraClient) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*EsploraClient")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterEsploraClient) Write(writer io.Writer, value *EsploraClient) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerEsploraClient struct{}

func (_ FfiDestroyerEsploraClient) Destroy(value *EsploraClient) {
	value.Destroy()
}

// An external UTXO, owned by another wallet
type ExternalUtxoInterface interface {
}

// An external UTXO, owned by another wallet
type ExternalUtxo struct {
	ffiObject FfiObject
}

// Construct an ExternalUtxo
func NewExternalUtxo(vout uint32, tx *Transaction, unblinded *TxOutSecrets, maxWeightToSatisfy uint32, isSegwit bool) (*ExternalUtxo, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_externalutxo_new(FfiConverterUint32INSTANCE.Lower(vout), FfiConverterTransactionINSTANCE.Lower(tx), FfiConverterTxOutSecretsINSTANCE.Lower(unblinded), FfiConverterUint32INSTANCE.Lower(maxWeightToSatisfy), FfiConverterBoolINSTANCE.Lower(isSegwit), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ExternalUtxo
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterExternalUtxoINSTANCE.Lift(_uniffiRV), nil
	}
}

func (object *ExternalUtxo) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterExternalUtxo struct{}

var FfiConverterExternalUtxoINSTANCE = FfiConverterExternalUtxo{}

func (c FfiConverterExternalUtxo) Lift(pointer unsafe.Pointer) *ExternalUtxo {
	result := &ExternalUtxo{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_externalutxo(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_externalutxo(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ExternalUtxo).Destroy)
	return result
}

func (c FfiConverterExternalUtxo) Read(reader io.Reader) *ExternalUtxo {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterExternalUtxo) Lower(value *ExternalUtxo) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*ExternalUtxo")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterExternalUtxo) Write(writer io.Writer, value *ExternalUtxo) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerExternalUtxo struct{}

func (_ FfiDestroyerExternalUtxo) Destroy(value *ExternalUtxo) {
	value.Destroy()
}

// An FFI-safe key-value storage trait for caller-defined persistence.
//
// Keys are strings to allow namespacing (e.g., "Liquid:Tx:abcd1234").
// Values are byte arrays for flexibility in serialization.
type ForeignStore interface {
	// Retrieve a value by key.
	//
	// Returns `Ok(None)` if the key does not exist.
	Get(key string) (*[]byte, error)
	// Insert or update a value.
	Put(key string, value []byte) error
	// Remove a value by key.
	//
	// Returns `Ok(())` even if the key did not exist.
	Remove(key string) error
}

// An FFI-safe key-value storage trait for caller-defined persistence.
//
// Keys are strings to allow namespacing (e.g., "Liquid:Tx:abcd1234").
// Values are byte arrays for flexibility in serialization.
type ForeignStoreImpl struct {
	ffiObject FfiObject
}

// Retrieve a value by key.
//
// Returns `Ok(None)` if the key does not exist.
func (_self *ForeignStoreImpl) Get(key string) (*[]byte, error) {
	_pointer := _self.ffiObject.incrementPointer("ForeignStore")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_foreignstore_get(
				_pointer, FfiConverterStringINSTANCE.Lower(key), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *[]byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalBytesINSTANCE.Lift(_uniffiRV), nil
	}
}

// Insert or update a value.
func (_self *ForeignStoreImpl) Put(key string, value []byte) error {
	_pointer := _self.ffiObject.incrementPointer("ForeignStore")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_foreignstore_put(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterBytesINSTANCE.Lower(value), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Remove a value by key.
//
// Returns `Ok(())` even if the key did not exist.
func (_self *ForeignStoreImpl) Remove(key string) error {
	_pointer := _self.ffiObject.incrementPointer("ForeignStore")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_foreignstore_remove(
			_pointer, FfiConverterStringINSTANCE.Lower(key), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}
func (object *ForeignStoreImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterForeignStore struct {
	handleMap *concurrentHandleMap[ForeignStore]
}

var FfiConverterForeignStoreINSTANCE = FfiConverterForeignStore{
	handleMap: newConcurrentHandleMap[ForeignStore](),
}

func (c FfiConverterForeignStore) Lift(pointer unsafe.Pointer) ForeignStore {
	result := &ForeignStoreImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_foreignstore(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_foreignstore(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ForeignStoreImpl).Destroy)
	return result
}

func (c FfiConverterForeignStore) Read(reader io.Reader) ForeignStore {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterForeignStore) Lower(value ForeignStore) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterForeignStore) Write(writer io.Writer, value ForeignStore) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerForeignStore struct{}

func (_ FfiDestroyerForeignStore) Destroy(value ForeignStore) {
	if val, ok := value.(*ForeignStoreImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *ForeignStoreImpl")
	}
}

type uniffiCallbackResult C.int8_t

const (
	uniffiIdxCallbackFree               uniffiCallbackResult = 0
	uniffiCallbackResultSuccess         uniffiCallbackResult = 0
	uniffiCallbackResultError           uniffiCallbackResult = 1
	uniffiCallbackUnexpectedResultError uniffiCallbackResult = 2
	uniffiCallbackCancelled             uniffiCallbackResult = 3
)

type concurrentHandleMap[T any] struct {
	handles       map[uint64]T
	currentHandle uint64
	lock          sync.RWMutex
}

func newConcurrentHandleMap[T any]() *concurrentHandleMap[T] {
	return &concurrentHandleMap[T]{
		handles: map[uint64]T{},
	}
}

func (cm *concurrentHandleMap[T]) insert(obj T) uint64 {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	cm.currentHandle = cm.currentHandle + 1
	cm.handles[cm.currentHandle] = obj
	return cm.currentHandle
}

func (cm *concurrentHandleMap[T]) remove(handle uint64) {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	delete(cm.handles, handle)
}

func (cm *concurrentHandleMap[T]) tryGet(handle uint64) (T, bool) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()

	val, ok := cm.handles[handle]
	return val, ok
}

//export lwk_cgo_dispatchCallbackInterfaceForeignStoreMethod0
func lwk_cgo_dispatchCallbackInterfaceForeignStoreMethod0(uniffiHandle C.uint64_t, key C.RustBuffer, uniffiOutReturn *C.RustBuffer, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterForeignStoreINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	res, err :=
		uniffiObj.Get(
			FfiConverterStringINSTANCE.Lift(GoRustBuffer{
				inner: key,
			}),
		)

	if err != nil {
		var actualError *LwkError
		if errors.As(err, &actualError) {
			*callStatus = C.RustCallStatus{
				code:     C.int8_t(uniffiCallbackResultError),
				errorBuf: FfiConverterLwkErrorINSTANCE.Lower(actualError),
			}
		} else {
			*callStatus = C.RustCallStatus{
				code: C.int8_t(uniffiCallbackUnexpectedResultError),
			}
		}
		return
	}

	*uniffiOutReturn = FfiConverterOptionalBytesINSTANCE.Lower(res)
}

//export lwk_cgo_dispatchCallbackInterfaceForeignStoreMethod1
func lwk_cgo_dispatchCallbackInterfaceForeignStoreMethod1(uniffiHandle C.uint64_t, key C.RustBuffer, value C.RustBuffer, uniffiOutReturn *C.void, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterForeignStoreINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	err :=
		uniffiObj.Put(
			FfiConverterStringINSTANCE.Lift(GoRustBuffer{
				inner: key,
			}),
			FfiConverterBytesINSTANCE.Lift(GoRustBuffer{
				inner: value,
			}),
		)

	if err != nil {
		var actualError *LwkError
		if errors.As(err, &actualError) {
			*callStatus = C.RustCallStatus{
				code:     C.int8_t(uniffiCallbackResultError),
				errorBuf: FfiConverterLwkErrorINSTANCE.Lower(actualError),
			}
		} else {
			*callStatus = C.RustCallStatus{
				code: C.int8_t(uniffiCallbackUnexpectedResultError),
			}
		}
		return
	}

}

//export lwk_cgo_dispatchCallbackInterfaceForeignStoreMethod2
func lwk_cgo_dispatchCallbackInterfaceForeignStoreMethod2(uniffiHandle C.uint64_t, key C.RustBuffer, uniffiOutReturn *C.void, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterForeignStoreINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	err :=
		uniffiObj.Remove(
			FfiConverterStringINSTANCE.Lift(GoRustBuffer{
				inner: key,
			}),
		)

	if err != nil {
		var actualError *LwkError
		if errors.As(err, &actualError) {
			*callStatus = C.RustCallStatus{
				code:     C.int8_t(uniffiCallbackResultError),
				errorBuf: FfiConverterLwkErrorINSTANCE.Lower(actualError),
			}
		} else {
			*callStatus = C.RustCallStatus{
				code: C.int8_t(uniffiCallbackUnexpectedResultError),
			}
		}
		return
	}

}

var UniffiVTableCallbackInterfaceForeignStoreINSTANCE = C.UniffiVTableCallbackInterfaceForeignStore{
	get:    (C.UniffiCallbackInterfaceForeignStoreMethod0)(C.lwk_cgo_dispatchCallbackInterfaceForeignStoreMethod0),
	put:    (C.UniffiCallbackInterfaceForeignStoreMethod1)(C.lwk_cgo_dispatchCallbackInterfaceForeignStoreMethod1),
	remove: (C.UniffiCallbackInterfaceForeignStoreMethod2)(C.lwk_cgo_dispatchCallbackInterfaceForeignStoreMethod2),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.lwk_cgo_dispatchCallbackInterfaceForeignStoreFree),
}

//export lwk_cgo_dispatchCallbackInterfaceForeignStoreFree
func lwk_cgo_dispatchCallbackInterfaceForeignStoreFree(handle C.uint64_t) {
	FfiConverterForeignStoreINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterForeignStore) register() {
	C.uniffi_lwk_fn_init_callback_vtable_foreignstore(&UniffiVTableCallbackInterfaceForeignStoreINSTANCE)
}

// A bridge that connects a [`ForeignStore`] to [`lwk_common::Store`].
type ForeignStoreLinkInterface interface {
}

// A bridge that connects a [`ForeignStore`] to [`lwk_common::Store`].
type ForeignStoreLink struct {
	ffiObject FfiObject
}

// Create a new `ForeignStoreLink` from a foreign store implementation.
func NewForeignStoreLink(store ForeignStore) *ForeignStoreLink {
	return FfiConverterForeignStoreLinkINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_foreignstorelink_new(FfiConverterForeignStoreINSTANCE.Lower(store), _uniffiStatus)
	}))
}

func (object *ForeignStoreLink) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterForeignStoreLink struct{}

var FfiConverterForeignStoreLinkINSTANCE = FfiConverterForeignStoreLink{}

func (c FfiConverterForeignStoreLink) Lift(pointer unsafe.Pointer) *ForeignStoreLink {
	result := &ForeignStoreLink{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_foreignstorelink(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_foreignstorelink(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ForeignStoreLink).Destroy)
	return result
}

func (c FfiConverterForeignStoreLink) Read(reader io.Reader) *ForeignStoreLink {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterForeignStoreLink) Lower(value *ForeignStoreLink) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*ForeignStoreLink")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterForeignStoreLink) Write(writer io.Writer, value *ForeignStoreLink) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerForeignStoreLink struct{}

func (_ FfiDestroyerForeignStoreLink) Destroy(value *ForeignStoreLink) {
	value.Destroy()
}

type InvoiceResponseInterface interface {
	Advance() (PaymentState, error)
	Bolt11Invoice() (*Bolt11Invoice, error)
	// The fee of the swap provider
	//
	// It is equal to the invoice amount multiplied by the boltz fee rate.
	// For example for receiving an invoice of 10000 satoshi with a 0.25% rate would be 25 satoshi.
	BoltzFee() (*uint64, error)
	// The txid of the claim transaction of the swap
	ClaimTxid() (*string, error)
	CompletePay() (bool, error)
	// The fee of the swap provider and the network fee
	//
	// It is equal to the amount of the invoice minus the amount of the onchain transaction.
	Fee() (*uint64, error)
	// Serialize the prepare pay response data to a json string
	//
	// This can be used to restore the prepare pay response after a crash
	Serialize() (string, error)
	SwapId() (string, error)
}
type InvoiceResponse struct {
	ffiObject FfiObject
}

func (_self *InvoiceResponse) Advance() (PaymentState, error) {
	_pointer := _self.ffiObject.incrementPointer("*InvoiceResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_invoiceresponse_advance(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue PaymentState
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPaymentStateINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *InvoiceResponse) Bolt11Invoice() (*Bolt11Invoice, error) {
	_pointer := _self.ffiObject.incrementPointer("*InvoiceResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_invoiceresponse_bolt11_invoice(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Bolt11Invoice
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBolt11InvoiceINSTANCE.Lift(_uniffiRV), nil
	}
}

// The fee of the swap provider
//
// It is equal to the invoice amount multiplied by the boltz fee rate.
// For example for receiving an invoice of 10000 satoshi with a 0.25% rate would be 25 satoshi.
func (_self *InvoiceResponse) BoltzFee() (*uint64, error) {
	_pointer := _self.ffiObject.incrementPointer("*InvoiceResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_invoiceresponse_boltz_fee(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *uint64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalUint64INSTANCE.Lift(_uniffiRV), nil
	}
}

// The txid of the claim transaction of the swap
func (_self *InvoiceResponse) ClaimTxid() (*string, error) {
	_pointer := _self.ffiObject.incrementPointer("*InvoiceResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_invoiceresponse_claim_txid(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *InvoiceResponse) CompletePay() (bool, error) {
	_pointer := _self.ffiObject.incrementPointer("*InvoiceResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_invoiceresponse_complete_pay(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue bool
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBoolINSTANCE.Lift(_uniffiRV), nil
	}
}

// The fee of the swap provider and the network fee
//
// It is equal to the amount of the invoice minus the amount of the onchain transaction.
func (_self *InvoiceResponse) Fee() (*uint64, error) {
	_pointer := _self.ffiObject.incrementPointer("*InvoiceResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_invoiceresponse_fee(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *uint64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalUint64INSTANCE.Lift(_uniffiRV), nil
	}
}

// Serialize the prepare pay response data to a json string
//
// This can be used to restore the prepare pay response after a crash
func (_self *InvoiceResponse) Serialize() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*InvoiceResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_invoiceresponse_serialize(
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

func (_self *InvoiceResponse) SwapId() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*InvoiceResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_invoiceresponse_swap_id(
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
func (object *InvoiceResponse) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterInvoiceResponse struct{}

var FfiConverterInvoiceResponseINSTANCE = FfiConverterInvoiceResponse{}

func (c FfiConverterInvoiceResponse) Lift(pointer unsafe.Pointer) *InvoiceResponse {
	result := &InvoiceResponse{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_invoiceresponse(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_invoiceresponse(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*InvoiceResponse).Destroy)
	return result
}

func (c FfiConverterInvoiceResponse) Read(reader io.Reader) *InvoiceResponse {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterInvoiceResponse) Lower(value *InvoiceResponse) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*InvoiceResponse")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterInvoiceResponse) Write(writer io.Writer, value *InvoiceResponse) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerInvoiceResponse struct{}

func (_ FfiDestroyerInvoiceResponse) Destroy(value *InvoiceResponse) {
	value.Destroy()
}

// The details of an issuance or reissuance
type IssuanceInterface interface {
	// Return the asset id or None if it's a null issuance
	Asset() *AssetId
	// Return the amount of the asset in satoshis
	AssetSatoshi() *uint64
	// Return true if the issuance or reissuance is confidential
	IsConfidential() bool
	// Return true if this is effectively an issuance
	IsIssuance() bool
	// Return true if the issuance or reissuance is null
	IsNull() bool
	// Return true if this is effectively a reissuance
	IsReissuance() bool
	// Return the previous transaction id or None if it's a null issuance
	PrevTxid() **Txid
	// Return the previous output index or None if it's a null issuance
	PrevVout() *uint32
	// Return the token id or None if it's a null issuance
	Token() *AssetId
	// Return the amount of the reissuance token in satoshis
	TokenSatoshi() *uint64
}

// The details of an issuance or reissuance
type Issuance struct {
	ffiObject FfiObject
}

// Return the asset id or None if it's a null issuance
func (_self *Issuance) Asset() *AssetId {
	_pointer := _self.ffiObject.incrementPointer("*Issuance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalTypeAssetIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_issuance_asset(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the amount of the asset in satoshis
func (_self *Issuance) AssetSatoshi() *uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Issuance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_issuance_asset_satoshi(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return true if the issuance or reissuance is confidential
func (_self *Issuance) IsConfidential() bool {
	_pointer := _self.ffiObject.incrementPointer("*Issuance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_issuance_is_confidential(
			_pointer, _uniffiStatus)
	}))
}

// Return true if this is effectively an issuance
func (_self *Issuance) IsIssuance() bool {
	_pointer := _self.ffiObject.incrementPointer("*Issuance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_issuance_is_issuance(
			_pointer, _uniffiStatus)
	}))
}

// Return true if the issuance or reissuance is null
func (_self *Issuance) IsNull() bool {
	_pointer := _self.ffiObject.incrementPointer("*Issuance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_issuance_is_null(
			_pointer, _uniffiStatus)
	}))
}

// Return true if this is effectively a reissuance
func (_self *Issuance) IsReissuance() bool {
	_pointer := _self.ffiObject.incrementPointer("*Issuance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_issuance_is_reissuance(
			_pointer, _uniffiStatus)
	}))
}

// Return the previous transaction id or None if it's a null issuance
func (_self *Issuance) PrevTxid() **Txid {
	_pointer := _self.ffiObject.incrementPointer("*Issuance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalTxidINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_issuance_prev_txid(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the previous output index or None if it's a null issuance
func (_self *Issuance) PrevVout() *uint32 {
	_pointer := _self.ffiObject.incrementPointer("*Issuance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_issuance_prev_vout(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the token id or None if it's a null issuance
func (_self *Issuance) Token() *AssetId {
	_pointer := _self.ffiObject.incrementPointer("*Issuance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalTypeAssetIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_issuance_token(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the amount of the reissuance token in satoshis
func (_self *Issuance) TokenSatoshi() *uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Issuance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_issuance_token_satoshi(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *Issuance) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterIssuance struct{}

var FfiConverterIssuanceINSTANCE = FfiConverterIssuance{}

func (c FfiConverterIssuance) Lift(pointer unsafe.Pointer) *Issuance {
	result := &Issuance{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_issuance(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_issuance(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Issuance).Destroy)
	return result
}

func (c FfiConverterIssuance) Read(reader io.Reader) *Issuance {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterIssuance) Lower(value *Issuance) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Issuance")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterIssuance) Write(writer io.Writer, value *Issuance) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerIssuance struct{}

func (_ FfiDestroyerIssuance) Destroy(value *Issuance) {
	value.Destroy()
}

// Represents a lightning payment (bolt11 invoice or bolt12 offer)
type LightningPaymentInterface interface {
	// Returns the bolt11 invoice if the lightning payment is a bolt11 invoice
	Bolt11Invoice() **Bolt11Invoice
}

// Represents a lightning payment (bolt11 invoice or bolt12 offer)
type LightningPayment struct {
	ffiObject FfiObject
}

// Construct a lightning payment (bolt11 invoice or bolt12 offer) from a string
func NewLightningPayment(s string) (*LightningPayment, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_lightningpayment_new(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *LightningPayment
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterLightningPaymentINSTANCE.Lift(_uniffiRV), nil
	}
}

// Construct a lightning payment (bolt11 invoice or bolt12 offer) from a bolt11 invoice
func LightningPaymentFromBolt11Invoice(invoice *Bolt11Invoice) *LightningPayment {
	return FfiConverterLightningPaymentINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_lightningpayment_from_bolt11_invoice(FfiConverterBolt11InvoiceINSTANCE.Lower(invoice), _uniffiStatus)
	}))
}

// Returns the bolt11 invoice if the lightning payment is a bolt11 invoice
func (_self *LightningPayment) Bolt11Invoice() **Bolt11Invoice {
	_pointer := _self.ffiObject.incrementPointer("*LightningPayment")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalBolt11InvoiceINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lightningpayment_bolt11_invoice(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *LightningPayment) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLightningPayment struct{}

var FfiConverterLightningPaymentINSTANCE = FfiConverterLightningPayment{}

func (c FfiConverterLightningPayment) Lift(pointer unsafe.Pointer) *LightningPayment {
	result := &LightningPayment{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_lightningpayment(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_lightningpayment(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LightningPayment).Destroy)
	return result
}

func (c FfiConverterLightningPayment) Read(reader io.Reader) *LightningPayment {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLightningPayment) Lower(value *LightningPayment) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*LightningPayment")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterLightningPayment) Write(writer io.Writer, value *LightningPayment) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLightningPayment struct{}

func (_ FfiDestroyerLightningPayment) Destroy(value *LightningPayment) {
	value.Destroy()
}

type LockupResponseInterface interface {
	Advance() (PaymentState, error)
	// The fee of the swap provider
	//
	// It is equal to the swap amount multiplied by the boltz fee rate.
	BoltzFee() (*uint64, error)
	ChainFrom() (string, error)
	ChainTo() (string, error)
	// The txid of the claim transaction of the swap
	ClaimTxid() (*string, error)
	Complete() (bool, error)
	ExpectedAmount() (uint64, error)
	// The fee of the swap provider and the network fee
	//
	// It is equal to the amount requested minus the amount sent to the claim address.
	Fee() (*uint64, error)
	LockupAddress() (string, error)
	// The txid of the lockup transaction of the swap
	LockupTxid() (*string, error)
	Serialize() (string, error)
	// Optionally set the lockup transaction txid.
	//
	// This can be useful when the app creates and broadcasts the lockup transaction and wants to
	// persist the txid immediately before websocket updates arrive from Boltz. It helps avoid a
	// race where a fast retry flow could submit the lockup transaction twice.
	SetLockupTxid(txid string) error
	SwapId() (string, error)
	// The BIP21 URI for the lockup address, if provided by Boltz
	Uri() (*string, error)
}
type LockupResponse struct {
	ffiObject FfiObject
}

func (_self *LockupResponse) Advance() (PaymentState, error) {
	_pointer := _self.ffiObject.incrementPointer("*LockupResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lockupresponse_advance(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue PaymentState
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPaymentStateINSTANCE.Lift(_uniffiRV), nil
	}
}

// The fee of the swap provider
//
// It is equal to the swap amount multiplied by the boltz fee rate.
func (_self *LockupResponse) BoltzFee() (*uint64, error) {
	_pointer := _self.ffiObject.incrementPointer("*LockupResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lockupresponse_boltz_fee(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *uint64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalUint64INSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LockupResponse) ChainFrom() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*LockupResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lockupresponse_chain_from(
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

func (_self *LockupResponse) ChainTo() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*LockupResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lockupresponse_chain_to(
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

// The txid of the claim transaction of the swap
func (_self *LockupResponse) ClaimTxid() (*string, error) {
	_pointer := _self.ffiObject.incrementPointer("*LockupResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lockupresponse_claim_txid(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LockupResponse) Complete() (bool, error) {
	_pointer := _self.ffiObject.incrementPointer("*LockupResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_lockupresponse_complete(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue bool
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBoolINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LockupResponse) ExpectedAmount() (uint64, error) {
	_pointer := _self.ffiObject.incrementPointer("*LockupResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_lockupresponse_expected_amount(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue uint64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUint64INSTANCE.Lift(_uniffiRV), nil
	}
}

// The fee of the swap provider and the network fee
//
// It is equal to the amount requested minus the amount sent to the claim address.
func (_self *LockupResponse) Fee() (*uint64, error) {
	_pointer := _self.ffiObject.incrementPointer("*LockupResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lockupresponse_fee(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *uint64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalUint64INSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LockupResponse) LockupAddress() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*LockupResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lockupresponse_lockup_address(
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

// The txid of the lockup transaction of the swap
func (_self *LockupResponse) LockupTxid() (*string, error) {
	_pointer := _self.ffiObject.incrementPointer("*LockupResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lockupresponse_lockup_txid(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *LockupResponse) Serialize() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*LockupResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lockupresponse_serialize(
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

// Optionally set the lockup transaction txid.
//
// This can be useful when the app creates and broadcasts the lockup transaction and wants to
// persist the txid immediately before websocket updates arrive from Boltz. It helps avoid a
// race where a fast retry flow could submit the lockup transaction twice.
func (_self *LockupResponse) SetLockupTxid(txid string) error {
	_pointer := _self.ffiObject.incrementPointer("*LockupResponse")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_lockupresponse_set_lockup_txid(
			_pointer, FfiConverterStringINSTANCE.Lower(txid), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *LockupResponse) SwapId() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*LockupResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lockupresponse_swap_id(
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

// The BIP21 URI for the lockup address, if provided by Boltz
func (_self *LockupResponse) Uri() (*string, error) {
	_pointer := _self.ffiObject.incrementPointer("*LockupResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lockupresponse_uri(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalStringINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *LockupResponse) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLockupResponse struct{}

var FfiConverterLockupResponseINSTANCE = FfiConverterLockupResponse{}

func (c FfiConverterLockupResponse) Lift(pointer unsafe.Pointer) *LockupResponse {
	result := &LockupResponse{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_lockupresponse(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_lockupresponse(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LockupResponse).Destroy)
	return result
}

func (c FfiConverterLockupResponse) Read(reader io.Reader) *LockupResponse {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLockupResponse) Lower(value *LockupResponse) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*LockupResponse")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterLockupResponse) Write(writer io.Writer, value *LockupResponse) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLockupResponse struct{}

func (_ FfiDestroyerLockupResponse) Destroy(value *LockupResponse) {
	value.Destroy()
}

// An exported trait for handling logging messages.
//
// Implement this trait to receive and handle logging messages from the lightning session.
type Logging interface {
	// Log a message with the given level
	Log(level LogLevel, message string)
}

// An exported trait for handling logging messages.
//
// Implement this trait to receive and handle logging messages from the lightning session.
type LoggingImpl struct {
	ffiObject FfiObject
}

// Log a message with the given level
func (_self *LoggingImpl) Log(level LogLevel, message string) {
	_pointer := _self.ffiObject.incrementPointer("Logging")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_logging_log(
			_pointer, FfiConverterLogLevelINSTANCE.Lower(level), FfiConverterStringINSTANCE.Lower(message), _uniffiStatus)
		return false
	})
}
func (object *LoggingImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLogging struct {
	handleMap *concurrentHandleMap[Logging]
}

var FfiConverterLoggingINSTANCE = FfiConverterLogging{
	handleMap: newConcurrentHandleMap[Logging](),
}

func (c FfiConverterLogging) Lift(pointer unsafe.Pointer) Logging {
	result := &LoggingImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_logging(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_logging(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LoggingImpl).Destroy)
	return result
}

func (c FfiConverterLogging) Read(reader io.Reader) Logging {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLogging) Lower(value Logging) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterLogging) Write(writer io.Writer, value Logging) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLogging struct{}

func (_ FfiDestroyerLogging) Destroy(value Logging) {
	if val, ok := value.(*LoggingImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *LoggingImpl")
	}
}

//export lwk_cgo_dispatchCallbackInterfaceLoggingMethod0
func lwk_cgo_dispatchCallbackInterfaceLoggingMethod0(uniffiHandle C.uint64_t, level C.RustBuffer, message C.RustBuffer, uniffiOutReturn *C.void, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterLoggingINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	uniffiObj.Log(
		FfiConverterLogLevelINSTANCE.Lift(GoRustBuffer{
			inner: level,
		}),
		FfiConverterStringINSTANCE.Lift(GoRustBuffer{
			inner: message,
		}),
	)

}

var UniffiVTableCallbackInterfaceLoggingINSTANCE = C.UniffiVTableCallbackInterfaceLogging{
	log: (C.UniffiCallbackInterfaceLoggingMethod0)(C.lwk_cgo_dispatchCallbackInterfaceLoggingMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.lwk_cgo_dispatchCallbackInterfaceLoggingFree),
}

//export lwk_cgo_dispatchCallbackInterfaceLoggingFree
func lwk_cgo_dispatchCallbackInterfaceLoggingFree(handle C.uint64_t) {
	FfiConverterLoggingINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterLogging) register() {
	C.uniffi_lwk_fn_init_callback_vtable_logging(&UniffiVTableCallbackInterfaceLoggingINSTANCE)
}

// An object to define logging at the caller level
type LoggingLinkInterface interface {
}

// An object to define logging at the caller level
type LoggingLink struct {
	ffiObject FfiObject
}

// Create a new `LoggingLink`
func NewLoggingLink(logging Logging) *LoggingLink {
	return FfiConverterLoggingLinkINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_logginglink_new(FfiConverterLoggingINSTANCE.Lower(logging), _uniffiStatus)
	}))
}

func (object *LoggingLink) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLoggingLink struct{}

var FfiConverterLoggingLinkINSTANCE = FfiConverterLoggingLink{}

func (c FfiConverterLoggingLink) Lift(pointer unsafe.Pointer) *LoggingLink {
	result := &LoggingLink{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_logginglink(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_logginglink(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LoggingLink).Destroy)
	return result
}

func (c FfiConverterLoggingLink) Read(reader io.Reader) *LoggingLink {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLoggingLink) Lower(value *LoggingLink) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*LoggingLink")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterLoggingLink) Write(writer io.Writer, value *LoggingLink) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLoggingLink struct{}

func (_ FfiDestroyerLoggingLink) Destroy(value *LoggingLink) {
	value.Destroy()
}

// Test environment
//
// Regtest with:
// * Elements node
// * Electrum server
// * Esplora server
// * Waterfalls server
//
// Wrapper over [`lwk_test_util::TestEnv`]
type LwkTestEnvInterface interface {
	// Get the Electrum URL of the test environment
	ElectrumUrl() string
	// Get the Esplora URL of the test environment
	EsploraUrl() string
	// Generate `blocks` blocks from the node
	Generate(blocks uint32)
	// Get the genesis block hash from the running node.
	GenesisBlockHash() string
	// Get a new address from the node
	GetNewAddress() *Address
	// Get the height of the node
	Height() uint64
	// Issue `satoshi` of an asset from the node
	IssueAsset(satoshi uint64) AssetId
	// Send `satoshi` to `address` from the node
	SendToAddress(address *Address, satoshi uint64, asset *AssetId) *Txid
	// Get the Waterfalls URL of the test environment
	WaterfallsUrl() string
}

// Test environment
//
// Regtest with:
// * Elements node
// * Electrum server
// * Esplora server
// * Waterfalls server
//
// Wrapper over [`lwk_test_util::TestEnv`]
type LwkTestEnv struct {
	ffiObject FfiObject
}

// Creates a new test environment
func NewLwkTestEnv() *LwkTestEnv {
	return FfiConverterLwkTestEnvINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_lwktestenv_new(_uniffiStatus)
	}))
}

// Get the Electrum URL of the test environment
func (_self *LwkTestEnv) ElectrumUrl() string {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestEnv")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lwktestenv_electrum_url(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the Esplora URL of the test environment
func (_self *LwkTestEnv) EsploraUrl() string {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestEnv")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lwktestenv_esplora_url(
				_pointer, _uniffiStatus),
		}
	}))
}

// Generate `blocks` blocks from the node
func (_self *LwkTestEnv) Generate(blocks uint32) {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestEnv")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_lwktestenv_generate(
			_pointer, FfiConverterUint32INSTANCE.Lower(blocks), _uniffiStatus)
		return false
	})
}

// Get the genesis block hash from the running node.
func (_self *LwkTestEnv) GenesisBlockHash() string {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestEnv")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lwktestenv_genesis_block_hash(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get a new address from the node
func (_self *LwkTestEnv) GetNewAddress() *Address {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestEnv")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterAddressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_lwktestenv_get_new_address(
			_pointer, _uniffiStatus)
	}))
}

// Get the height of the node
func (_self *LwkTestEnv) Height() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestEnv")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_lwktestenv_height(
			_pointer, _uniffiStatus)
	}))
}

// Issue `satoshi` of an asset from the node
func (_self *LwkTestEnv) IssueAsset(satoshi uint64) AssetId {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestEnv")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeAssetIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lwktestenv_issue_asset(
				_pointer, FfiConverterUint64INSTANCE.Lower(satoshi), _uniffiStatus),
		}
	}))
}

// Send `satoshi` to `address` from the node
func (_self *LwkTestEnv) SendToAddress(address *Address, satoshi uint64, asset *AssetId) *Txid {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestEnv")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTxidINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_lwktestenv_send_to_address(
			_pointer, FfiConverterAddressINSTANCE.Lower(address), FfiConverterUint64INSTANCE.Lower(satoshi), FfiConverterOptionalTypeAssetIdINSTANCE.Lower(asset), _uniffiStatus)
	}))
}

// Get the Waterfalls URL of the test environment
func (_self *LwkTestEnv) WaterfallsUrl() string {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestEnv")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lwktestenv_waterfalls_url(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *LwkTestEnv) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLwkTestEnv struct{}

var FfiConverterLwkTestEnvINSTANCE = FfiConverterLwkTestEnv{}

func (c FfiConverterLwkTestEnv) Lift(pointer unsafe.Pointer) *LwkTestEnv {
	result := &LwkTestEnv{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_lwktestenv(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_lwktestenv(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LwkTestEnv).Destroy)
	return result
}

func (c FfiConverterLwkTestEnv) Read(reader io.Reader) *LwkTestEnv {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLwkTestEnv) Lower(value *LwkTestEnv) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*LwkTestEnv")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterLwkTestEnv) Write(writer io.Writer, value *LwkTestEnv) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLwkTestEnv struct{}

func (_ FfiDestroyerLwkTestEnv) Destroy(value *LwkTestEnv) {
	value.Destroy()
}

// A lightweight test helper for [`ForeignStore`](crate::ForeignStore) implementations.
//
// Use this to verify that Rust can correctly read/write through an FFI store.
type LwkTestStoreInterface interface {
	// Read a value from the store.
	Read(key string) (*[]byte, error)
	// Remove a key from the store.
	Remove(key string) error
	// Write a key-value pair to the store.
	Write(key string, value []byte) error
}

// A lightweight test helper for [`ForeignStore`](crate::ForeignStore) implementations.
//
// Use this to verify that Rust can correctly read/write through an FFI store.
type LwkTestStore struct {
	ffiObject FfiObject
}

// Create a new test store helper wrapping the given store.
func NewLwkTestStore(store *ForeignStoreLink) *LwkTestStore {
	return FfiConverterLwkTestStoreINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_lwkteststore_new(FfiConverterForeignStoreLinkINSTANCE.Lower(store), _uniffiStatus)
	}))
}

// Read a value from the store.
func (_self *LwkTestStore) Read(key string) (*[]byte, error) {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestStore")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_lwkteststore_read(
				_pointer, FfiConverterStringINSTANCE.Lower(key), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *[]byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalBytesINSTANCE.Lift(_uniffiRV), nil
	}
}

// Remove a key from the store.
func (_self *LwkTestStore) Remove(key string) error {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestStore")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_lwkteststore_remove(
			_pointer, FfiConverterStringINSTANCE.Lower(key), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Write a key-value pair to the store.
func (_self *LwkTestStore) Write(key string, value []byte) error {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestStore")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_lwkteststore_write(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterBytesINSTANCE.Lower(value), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}
func (object *LwkTestStore) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLwkTestStore struct{}

var FfiConverterLwkTestStoreINSTANCE = FfiConverterLwkTestStore{}

func (c FfiConverterLwkTestStore) Lift(pointer unsafe.Pointer) *LwkTestStore {
	result := &LwkTestStore{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_lwkteststore(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_lwkteststore(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*LwkTestStore).Destroy)
	return result
}

func (c FfiConverterLwkTestStore) Read(reader io.Reader) *LwkTestStore {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLwkTestStore) Lower(value *LwkTestStore) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*LwkTestStore")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterLwkTestStore) Write(writer io.Writer, value *LwkTestStore) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLwkTestStore struct{}

func (_ FfiDestroyerLwkTestStore) Destroy(value *LwkTestStore) {
	value.Destroy()
}

// Wrapper over [`bip39::Mnemonic`]
type MnemonicInterface interface {
	// Get the number of words in this mnemonic
	WordCount() uint8
}

// Wrapper over [`bip39::Mnemonic`]
type Mnemonic struct {
	ffiObject FfiObject
}

// Construct a Mnemonic type
func NewMnemonic(s string) (*Mnemonic, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_mnemonic_new(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Mnemonic
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterMnemonicINSTANCE.Lift(_uniffiRV), nil
	}
}

// Creates a Mnemonic from entropy, at least 16 bytes are needed.
func MnemonicFromEntropy(b []byte) (*Mnemonic, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_mnemonic_from_entropy(FfiConverterBytesINSTANCE.Lower(b), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Mnemonic
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterMnemonicINSTANCE.Lift(_uniffiRV), nil
	}
}

// Creates a random Mnemonic of given words (12,15,18,21,24)
func MnemonicFromRandom(wordCount uint8) (*Mnemonic, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_mnemonic_from_random(FfiConverterUint8INSTANCE.Lower(wordCount), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Mnemonic
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterMnemonicINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the number of words in this mnemonic
func (_self *Mnemonic) WordCount() uint8 {
	_pointer := _self.ffiObject.incrementPointer("*Mnemonic")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint8_t {
		return C.uniffi_lwk_fn_method_mnemonic_word_count(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Mnemonic) String() string {
	_pointer := _self.ffiObject.incrementPointer("*Mnemonic")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_mnemonic_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *Mnemonic) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterMnemonic struct{}

var FfiConverterMnemonicINSTANCE = FfiConverterMnemonic{}

func (c FfiConverterMnemonic) Lift(pointer unsafe.Pointer) *Mnemonic {
	result := &Mnemonic{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_mnemonic(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_mnemonic(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Mnemonic).Destroy)
	return result
}

func (c FfiConverterMnemonic) Read(reader io.Reader) *Mnemonic {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterMnemonic) Lower(value *Mnemonic) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Mnemonic")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterMnemonic) Write(writer io.Writer, value *Mnemonic) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerMnemonic struct{}

func (_ FfiDestroyerMnemonic) Destroy(value *Mnemonic) {
	value.Destroy()
}

// The network of the elements blockchain.
type NetworkInterface interface {
	// Return the default electrum client for this network
	DefaultElectrumClient() (*ElectrumClient, error)
	// Return the default esplora client for this network
	DefaultEsploraClient() (*EsploraClient, error)
	// Return the genesis block hash for this network as hex string.
	GenesisBlockHash() string
	// Return true if the network is the mainnet network
	IsMainnet() bool
	// Return the policy asset (eg LBTC for mainnet) for this network
	PolicyAsset() AssetId
	// Return a new `TxBuilder` for this network
	TxBuilder() *TxBuilder
}

// The network of the elements blockchain.
type Network struct {
	ffiObject FfiObject
}

// Return the mainnet network
func NetworkMainnet() *Network {
	return FfiConverterNetworkINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_network_mainnet(_uniffiStatus)
	}))
}

// Return the regtest network with the given policy asset
func NetworkRegtest(policyAsset AssetId) *Network {
	return FfiConverterNetworkINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_network_regtest(FfiConverterTypeAssetIdINSTANCE.Lower(policyAsset), _uniffiStatus)
	}))
}

// Return the default regtest network with the default policy asset
func NetworkRegtestDefault() *Network {
	return FfiConverterNetworkINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_network_regtest_default(_uniffiStatus)
	}))
}

// Return the testnet network
func NetworkTestnet() *Network {
	return FfiConverterNetworkINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_network_testnet(_uniffiStatus)
	}))
}

// Return the default electrum client for this network
func (_self *Network) DefaultElectrumClient() (*ElectrumClient, error) {
	_pointer := _self.ffiObject.incrementPointer("*Network")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_network_default_electrum_client(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ElectrumClient
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterElectrumClientINSTANCE.Lift(_uniffiRV), nil
	}
}

// Return the default esplora client for this network
func (_self *Network) DefaultEsploraClient() (*EsploraClient, error) {
	_pointer := _self.ffiObject.incrementPointer("*Network")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_network_default_esplora_client(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *EsploraClient
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterEsploraClientINSTANCE.Lift(_uniffiRV), nil
	}
}

// Return the genesis block hash for this network as hex string.
func (_self *Network) GenesisBlockHash() string {
	_pointer := _self.ffiObject.incrementPointer("*Network")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_network_genesis_block_hash(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return true if the network is the mainnet network
func (_self *Network) IsMainnet() bool {
	_pointer := _self.ffiObject.incrementPointer("*Network")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_network_is_mainnet(
			_pointer, _uniffiStatus)
	}))
}

// Return the policy asset (eg LBTC for mainnet) for this network
func (_self *Network) PolicyAsset() AssetId {
	_pointer := _self.ffiObject.incrementPointer("*Network")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeAssetIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_network_policy_asset(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return a new `TxBuilder` for this network
func (_self *Network) TxBuilder() *TxBuilder {
	_pointer := _self.ffiObject.incrementPointer("*Network")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTxBuilderINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_network_tx_builder(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Network) String() string {
	_pointer := _self.ffiObject.incrementPointer("*Network")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_network_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Network) Eq(other *Network) bool {
	_pointer := _self.ffiObject.incrementPointer("*Network")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_network_uniffi_trait_eq_eq(
			_pointer, FfiConverterNetworkINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *Network) Ne(other *Network) bool {
	_pointer := _self.ffiObject.incrementPointer("*Network")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_network_uniffi_trait_eq_ne(
			_pointer, FfiConverterNetworkINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (object *Network) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterNetwork struct{}

var FfiConverterNetworkINSTANCE = FfiConverterNetwork{}

func (c FfiConverterNetwork) Lift(pointer unsafe.Pointer) *Network {
	result := &Network{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_network(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_network(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Network).Destroy)
	return result
}

func (c FfiConverterNetwork) Read(reader io.Reader) *Network {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterNetwork) Lower(value *Network) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Network")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterNetwork) Write(writer io.Writer, value *Network) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerNetwork struct{}

func (_ FfiDestroyerNetwork) Destroy(value *Network) {
	value.Destroy()
}

// A reference to a transaction output
type OutPointInterface interface {
	// Return the transaction identifier.
	Txid() *Txid
	// Return the output index.
	Vout() uint32
}

// A reference to a transaction output
type OutPoint struct {
	ffiObject FfiObject
}

// Construct an OutPoint object from its string representation.
// For example: "[elements]0000000000000000000000000000000000000000000000000000000000000001:1"
// To create the string representation of an outpoint use `to_string()`.
func NewOutPoint(s string) (*OutPoint, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_outpoint_new(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *OutPoint
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOutPointINSTANCE.Lift(_uniffiRV), nil
	}
}

// Create an OutPoint from a transaction id and output index.
func OutPointFromParts(txid *Txid, vout uint32) *OutPoint {
	return FfiConverterOutPointINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_outpoint_from_parts(FfiConverterTxidINSTANCE.Lower(txid), FfiConverterUint32INSTANCE.Lower(vout), _uniffiStatus)
	}))
}

// Return the transaction identifier.
func (_self *OutPoint) Txid() *Txid {
	_pointer := _self.ffiObject.incrementPointer("*OutPoint")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTxidINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_outpoint_txid(
			_pointer, _uniffiStatus)
	}))
}

// Return the output index.
func (_self *OutPoint) Vout() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*OutPoint")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_outpoint_vout(
			_pointer, _uniffiStatus)
	}))
}

func (_self *OutPoint) String() string {
	_pointer := _self.ffiObject.incrementPointer("*OutPoint")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_outpoint_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *OutPoint) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterOutPoint struct{}

var FfiConverterOutPointINSTANCE = FfiConverterOutPoint{}

func (c FfiConverterOutPoint) Lift(pointer unsafe.Pointer) *OutPoint {
	result := &OutPoint{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_outpoint(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_outpoint(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*OutPoint).Destroy)
	return result
}

func (c FfiConverterOutPoint) Read(reader io.Reader) *OutPoint {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterOutPoint) Lower(value *OutPoint) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*OutPoint")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterOutPoint) Write(writer io.Writer, value *OutPoint) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerOutPoint struct{}

func (_ FfiDestroyerOutPoint) Destroy(value *OutPoint) {
	value.Destroy()
}

// A parsed payment category from a payment instruction string.
//
// This can be a Bitcoin address, Liquid address, Lightning invoice,
// Lightning offer, LNURL, BIP353, BIP21 URI, or Liquid BIP21 URI.
type PaymentInterface interface {
	// Returns the BIP21 URI if this is a Bip21 category, None otherwise
	Bip21() **Bip21
	// Returns the BIP321 URI if this is a Bip321 category, None otherwise
	Bip321() **Bip321
	// Returns the BIP353 address (without the ₿ prefix) if this is a Bip353 category, None otherwise
	Bip353() *string
	// Returns the Bitcoin address if this is a BitcoinAddress category, None otherwise
	//
	// Returns the address portion of the original input string
	BitcoinAddress() **BitcoinAddress
	// Returns the kind of payment category
	Kind() PaymentKind
	// Returns the Lightning invoice if this is a `LightningInvoice` category, `None` otherwise
	LightningInvoice() **Bolt11Invoice
	// Returns the Lightning offer as a string if this is a LightningOffer category, None otherwise
	LightningOffer() *string
	// Returns a `LightningPayment`` if this category is payable via Lightning
	//
	// Returns `Some` for `LightningInvoice`, `LightningOffer`, and `LnUrl` categories.
	// The returned `LightningPayment` can be used with `BoltzSession::prepare_pay()`.
	LightningPayment() **LightningPayment
	// Returns the Liquid address if this is a LiquidAddress category, None otherwise
	LiquidAddress() **Address
	// Returns the Liquid BIP21 details if this is a LiquidBip21 category, None otherwise
	LiquidBip21() *LiquidBip21
	// Returns the LNURL as a string if this is an LnUrl category, None otherwise
	Lnurl() *string
}

// A parsed payment category from a payment instruction string.
//
// This can be a Bitcoin address, Liquid address, Lightning invoice,
// Lightning offer, LNURL, BIP353, BIP21 URI, or Liquid BIP21 URI.
type Payment struct {
	ffiObject FfiObject
}

// Parse a payment instruction string into a PaymentCategory
func NewPayment(s string) (*Payment, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_payment_new(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Payment
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPaymentINSTANCE.Lift(_uniffiRV), nil
	}
}

// Returns the BIP21 URI if this is a Bip21 category, None otherwise
func (_self *Payment) Bip21() **Bip21 {
	_pointer := _self.ffiObject.incrementPointer("*Payment")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalBip21INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_payment_bip21(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the BIP321 URI if this is a Bip321 category, None otherwise
func (_self *Payment) Bip321() **Bip321 {
	_pointer := _self.ffiObject.incrementPointer("*Payment")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalBip321INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_payment_bip321(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the BIP353 address (without the ₿ prefix) if this is a Bip353 category, None otherwise
func (_self *Payment) Bip353() *string {
	_pointer := _self.ffiObject.incrementPointer("*Payment")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_payment_bip353(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the Bitcoin address if this is a BitcoinAddress category, None otherwise
//
// Returns the address portion of the original input string
func (_self *Payment) BitcoinAddress() **BitcoinAddress {
	_pointer := _self.ffiObject.incrementPointer("*Payment")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalBitcoinAddressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_payment_bitcoin_address(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the kind of payment category
func (_self *Payment) Kind() PaymentKind {
	_pointer := _self.ffiObject.incrementPointer("*Payment")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterPaymentKindINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_payment_kind(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the Lightning invoice if this is a `LightningInvoice` category, `None` otherwise
func (_self *Payment) LightningInvoice() **Bolt11Invoice {
	_pointer := _self.ffiObject.incrementPointer("*Payment")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalBolt11InvoiceINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_payment_lightning_invoice(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the Lightning offer as a string if this is a LightningOffer category, None otherwise
func (_self *Payment) LightningOffer() *string {
	_pointer := _self.ffiObject.incrementPointer("*Payment")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_payment_lightning_offer(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns a `LightningPayment“ if this category is payable via Lightning
//
// Returns `Some` for `LightningInvoice`, `LightningOffer`, and `LnUrl` categories.
// The returned `LightningPayment` can be used with `BoltzSession::prepare_pay()`.
func (_self *Payment) LightningPayment() **LightningPayment {
	_pointer := _self.ffiObject.incrementPointer("*Payment")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLightningPaymentINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_payment_lightning_payment(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the Liquid address if this is a LiquidAddress category, None otherwise
func (_self *Payment) LiquidAddress() **Address {
	_pointer := _self.ffiObject.incrementPointer("*Payment")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalAddressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_payment_liquid_address(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the Liquid BIP21 details if this is a LiquidBip21 category, None otherwise
func (_self *Payment) LiquidBip21() *LiquidBip21 {
	_pointer := _self.ffiObject.incrementPointer("*Payment")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalLiquidBip21INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_payment_liquid_bip21(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the LNURL as a string if this is an LnUrl category, None otherwise
func (_self *Payment) Lnurl() *string {
	_pointer := _self.ffiObject.incrementPointer("*Payment")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_payment_lnurl(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *Payment) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterPayment struct{}

var FfiConverterPaymentINSTANCE = FfiConverterPayment{}

func (c FfiConverterPayment) Lift(pointer unsafe.Pointer) *Payment {
	result := &Payment{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_payment(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_payment(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Payment).Destroy)
	return result
}

func (c FfiConverterPayment) Read(reader io.Reader) *Payment {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterPayment) Lower(value *Payment) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Payment")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterPayment) Write(writer io.Writer, value *Payment) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerPayment struct{}

func (_ FfiDestroyerPayment) Destroy(value *Payment) {
	value.Destroy()
}

// POS (Point of Sale) configuration for encoding/decoding
type PosConfigInterface interface {
	// Get the currency code
	Currency() *CurrencyCode
	// Get the wallet descriptor
	Descriptor() *WolletDescriptor
	// Encode the POS configuration to a URL-safe base64 string
	Encode() (string, error)
	// Get whether to show the description/note field
	ShowDescription() *bool
	// Get whether to show the gear/settings button
	ShowGear() *bool
}

// POS (Point of Sale) configuration for encoding/decoding
type PosConfig struct {
	ffiObject FfiObject
}

// Create a new POS configuration
func NewPosConfig(descriptor *WolletDescriptor, currency *CurrencyCode) *PosConfig {
	return FfiConverterPosConfigINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_posconfig_new(FfiConverterWolletDescriptorINSTANCE.Lower(descriptor), FfiConverterCurrencyCodeINSTANCE.Lower(currency), _uniffiStatus)
	}))
}

// Decode a POS configuration from a URL-safe base64 encoded string
func PosConfigDecode(encoded string) (*PosConfig, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_posconfig_decode(FfiConverterStringINSTANCE.Lower(encoded), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *PosConfig
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPosConfigINSTANCE.Lift(_uniffiRV), nil
	}
}

// Create a POS configuration with all options
func PosConfigWithOptions(descriptor *WolletDescriptor, currency *CurrencyCode, showGear *bool, showDescription *bool) *PosConfig {
	return FfiConverterPosConfigINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_posconfig_with_options(FfiConverterWolletDescriptorINSTANCE.Lower(descriptor), FfiConverterCurrencyCodeINSTANCE.Lower(currency), FfiConverterOptionalBoolINSTANCE.Lower(showGear), FfiConverterOptionalBoolINSTANCE.Lower(showDescription), _uniffiStatus)
	}))
}

// Get the currency code
func (_self *PosConfig) Currency() *CurrencyCode {
	_pointer := _self.ffiObject.incrementPointer("*PosConfig")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterCurrencyCodeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_posconfig_currency(
			_pointer, _uniffiStatus)
	}))
}

// Get the wallet descriptor
func (_self *PosConfig) Descriptor() *WolletDescriptor {
	_pointer := _self.ffiObject.incrementPointer("*PosConfig")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterWolletDescriptorINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_posconfig_descriptor(
			_pointer, _uniffiStatus)
	}))
}

// Encode the POS configuration to a URL-safe base64 string
func (_self *PosConfig) Encode() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*PosConfig")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_posconfig_encode(
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

// Get whether to show the description/note field
func (_self *PosConfig) ShowDescription() *bool {
	_pointer := _self.ffiObject.incrementPointer("*PosConfig")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_posconfig_show_description(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get whether to show the gear/settings button
func (_self *PosConfig) ShowGear() *bool {
	_pointer := _self.ffiObject.incrementPointer("*PosConfig")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_posconfig_show_gear(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *PosConfig) String() string {
	_pointer := _self.ffiObject.incrementPointer("*PosConfig")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_posconfig_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *PosConfig) Eq(other *PosConfig) bool {
	_pointer := _self.ffiObject.incrementPointer("*PosConfig")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_posconfig_uniffi_trait_eq_eq(
			_pointer, FfiConverterPosConfigINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *PosConfig) Ne(other *PosConfig) bool {
	_pointer := _self.ffiObject.incrementPointer("*PosConfig")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_posconfig_uniffi_trait_eq_ne(
			_pointer, FfiConverterPosConfigINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (object *PosConfig) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterPosConfig struct{}

var FfiConverterPosConfigINSTANCE = FfiConverterPosConfig{}

func (c FfiConverterPosConfig) Lift(pointer unsafe.Pointer) *PosConfig {
	result := &PosConfig{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_posconfig(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_posconfig(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*PosConfig).Destroy)
	return result
}

func (c FfiConverterPosConfig) Read(reader io.Reader) *PosConfig {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterPosConfig) Lower(value *PosConfig) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*PosConfig")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterPosConfig) Write(writer io.Writer, value *PosConfig) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerPosConfig struct{}

func (_ FfiDestroyerPosConfig) Destroy(value *PosConfig) {
	value.Destroy()
}

// Wrapper over [`lwk_common::Precision`]
type PrecisionInterface interface {
	// See [`lwk_common::Precision::sats_to_string`]
	SatsToString(sats int64) string
	// See [`lwk_common::Precision::string_to_sats`]
	StringToSats(val string) (int64, error)
}

// Wrapper over [`lwk_common::Precision`]
type Precision struct {
	ffiObject FfiObject
}

// See [`lwk_common::Precision::new`]
func NewPrecision(precision uint8) (*Precision, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_precision_new(FfiConverterUint8INSTANCE.Lower(precision), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Precision
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPrecisionINSTANCE.Lift(_uniffiRV), nil
	}
}

// See [`lwk_common::Precision::sats_to_string`]
func (_self *Precision) SatsToString(sats int64) string {
	_pointer := _self.ffiObject.incrementPointer("*Precision")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_precision_sats_to_string(
				_pointer, FfiConverterInt64INSTANCE.Lower(sats), _uniffiStatus),
		}
	}))
}

// See [`lwk_common::Precision::string_to_sats`]
func (_self *Precision) StringToSats(val string) (int64, error) {
	_pointer := _self.ffiObject.incrementPointer("*Precision")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) C.int64_t {
		return C.uniffi_lwk_fn_method_precision_string_to_sats(
			_pointer, FfiConverterStringINSTANCE.Lower(val), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue int64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterInt64INSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *Precision) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterPrecision struct{}

var FfiConverterPrecisionINSTANCE = FfiConverterPrecision{}

func (c FfiConverterPrecision) Lift(pointer unsafe.Pointer) *Precision {
	result := &Precision{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_precision(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_precision(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Precision).Destroy)
	return result
}

func (c FfiConverterPrecision) Read(reader io.Reader) *Precision {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterPrecision) Lower(value *Precision) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Precision")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterPrecision) Write(writer io.Writer, value *Precision) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerPrecision struct{}

func (_ FfiDestroyerPrecision) Destroy(value *Precision) {
	value.Destroy()
}

type PreparePayResponseInterface interface {
	Advance() (PaymentState, error)
	// The fee of the swap provider
	//
	// It is equal to the invoice amount multiplied by the boltz fee rate.
	// For example for paying an invoice of 1000 satoshi with a 0.1% rate would be 1 satoshi.
	BoltzFee() (*uint64, error)
	CompletePay() (bool, error)
	// The fee of the swap provider and the network fee
	//
	// It is equal to the amount requested onchain minus the amount of the bolt11 invoice
	Fee() (*uint64, error)
	// The txid of the user lockup transaction of the swap.
	LockupTxid() (*string, error)
	// Serialize the prepare pay response data to a json string
	//
	// This can be used to restore the prepare pay response after a crash
	Serialize() (string, error)
	// Optionally set the lockup transaction txid.
	//
	// This can be useful when the app creates and broadcasts the lockup transaction and wants to
	// persist the txid immediately before websocket updates arrive from Boltz. It helps avoid a
	// race where a fast retry flow could submit the lockup transaction twice.
	SetLockupTxid(txid string) error
	SwapId() (string, error)
	Uri() (string, error)
	UriAddress() (*Address, error)
	UriAmount() (uint64, error)
}
type PreparePayResponse struct {
	ffiObject FfiObject
}

func (_self *PreparePayResponse) Advance() (PaymentState, error) {
	_pointer := _self.ffiObject.incrementPointer("*PreparePayResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_preparepayresponse_advance(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue PaymentState
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPaymentStateINSTANCE.Lift(_uniffiRV), nil
	}
}

// The fee of the swap provider
//
// It is equal to the invoice amount multiplied by the boltz fee rate.
// For example for paying an invoice of 1000 satoshi with a 0.1% rate would be 1 satoshi.
func (_self *PreparePayResponse) BoltzFee() (*uint64, error) {
	_pointer := _self.ffiObject.incrementPointer("*PreparePayResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_preparepayresponse_boltz_fee(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *uint64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalUint64INSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *PreparePayResponse) CompletePay() (bool, error) {
	_pointer := _self.ffiObject.incrementPointer("*PreparePayResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_preparepayresponse_complete_pay(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue bool
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBoolINSTANCE.Lift(_uniffiRV), nil
	}
}

// The fee of the swap provider and the network fee
//
// It is equal to the amount requested onchain minus the amount of the bolt11 invoice
func (_self *PreparePayResponse) Fee() (*uint64, error) {
	_pointer := _self.ffiObject.incrementPointer("*PreparePayResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_preparepayresponse_fee(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *uint64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalUint64INSTANCE.Lift(_uniffiRV), nil
	}
}

// The txid of the user lockup transaction of the swap.
func (_self *PreparePayResponse) LockupTxid() (*string, error) {
	_pointer := _self.ffiObject.incrementPointer("*PreparePayResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_preparepayresponse_lockup_txid(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// Serialize the prepare pay response data to a json string
//
// This can be used to restore the prepare pay response after a crash
func (_self *PreparePayResponse) Serialize() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*PreparePayResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_preparepayresponse_serialize(
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

// Optionally set the lockup transaction txid.
//
// This can be useful when the app creates and broadcasts the lockup transaction and wants to
// persist the txid immediately before websocket updates arrive from Boltz. It helps avoid a
// race where a fast retry flow could submit the lockup transaction twice.
func (_self *PreparePayResponse) SetLockupTxid(txid string) error {
	_pointer := _self.ffiObject.incrementPointer("*PreparePayResponse")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_preparepayresponse_set_lockup_txid(
			_pointer, FfiConverterStringINSTANCE.Lower(txid), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *PreparePayResponse) SwapId() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*PreparePayResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_preparepayresponse_swap_id(
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

func (_self *PreparePayResponse) Uri() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*PreparePayResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_preparepayresponse_uri(
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

func (_self *PreparePayResponse) UriAddress() (*Address, error) {
	_pointer := _self.ffiObject.incrementPointer("*PreparePayResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_preparepayresponse_uri_address(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Address
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAddressINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *PreparePayResponse) UriAmount() (uint64, error) {
	_pointer := _self.ffiObject.incrementPointer("*PreparePayResponse")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_preparepayresponse_uri_amount(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue uint64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUint64INSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *PreparePayResponse) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterPreparePayResponse struct{}

var FfiConverterPreparePayResponseINSTANCE = FfiConverterPreparePayResponse{}

func (c FfiConverterPreparePayResponse) Lift(pointer unsafe.Pointer) *PreparePayResponse {
	result := &PreparePayResponse{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_preparepayresponse(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_preparepayresponse(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*PreparePayResponse).Destroy)
	return result
}

func (c FfiConverterPreparePayResponse) Read(reader io.Reader) *PreparePayResponse {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterPreparePayResponse) Lower(value *PreparePayResponse) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*PreparePayResponse")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterPreparePayResponse) Write(writer io.Writer, value *PreparePayResponse) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerPreparePayResponse struct{}

func (_ FfiDestroyerPreparePayResponse) Destroy(value *PreparePayResponse) {
	value.Destroy()
}

// A Partially Signed Elements Transaction
type PsetInterface interface {
	// Attempt to combine with another `Pset`.
	Combine(other *Pset) (*Pset, error)
	// Extract the Transaction from a Pset by filling in
	// the available signature information in place.
	ExtractTx() (*Transaction, error)
	// Finalize and extract the PSET
	Finalize() (*Transaction, error)
	// Return a copy of the inputs of this PSET
	Inputs() []*PsetInput
	// Return a copy of the outputs of this PSET
	Outputs() []*PsetOutput
	// Get the unique id of the PSET as defined by [BIP-370](https://github.com/bitcoin/bips/blob/master/bip-0370.mediawiki#unique-identification)
	//
	// The unique id is the txid of the PSET with sequence numbers of inputs set to 0
	UniqueId() (*Txid, error)
}

// A Partially Signed Elements Transaction
type Pset struct {
	ffiObject FfiObject
}

// Construct a Watch-Only wallet object
func NewPset(base64 string) (*Pset, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_pset_new(FfiConverterStringINSTANCE.Lower(base64), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Pset
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPsetINSTANCE.Lift(_uniffiRV), nil
	}
}

// Attempt to combine with another `Pset`.
func (_self *Pset) Combine(other *Pset) (*Pset, error) {
	_pointer := _self.ffiObject.incrementPointer("*Pset")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_pset_combine(
			_pointer, FfiConverterPsetINSTANCE.Lower(other), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Pset
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPsetINSTANCE.Lift(_uniffiRV), nil
	}
}

// Extract the Transaction from a Pset by filling in
// the available signature information in place.
func (_self *Pset) ExtractTx() (*Transaction, error) {
	_pointer := _self.ffiObject.incrementPointer("*Pset")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_pset_extract_tx(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Transaction
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTransactionINSTANCE.Lift(_uniffiRV), nil
	}
}

// Finalize and extract the PSET
func (_self *Pset) Finalize() (*Transaction, error) {
	_pointer := _self.ffiObject.incrementPointer("*Pset")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_pset_finalize(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Transaction
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTransactionINSTANCE.Lift(_uniffiRV), nil
	}
}

// Return a copy of the inputs of this PSET
func (_self *Pset) Inputs() []*PsetInput {
	_pointer := _self.ffiObject.incrementPointer("*Pset")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequencePsetInputINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_pset_inputs(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return a copy of the outputs of this PSET
func (_self *Pset) Outputs() []*PsetOutput {
	_pointer := _self.ffiObject.incrementPointer("*Pset")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequencePsetOutputINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_pset_outputs(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the unique id of the PSET as defined by [BIP-370](https://github.com/bitcoin/bips/blob/master/bip-0370.mediawiki#unique-identification)
//
// The unique id is the txid of the PSET with sequence numbers of inputs set to 0
func (_self *Pset) UniqueId() (*Txid, error) {
	_pointer := _self.ffiObject.incrementPointer("*Pset")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_pset_unique_id(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Txid
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxidINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Pset) String() string {
	_pointer := _self.ffiObject.incrementPointer("*Pset")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_pset_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *Pset) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterPset struct{}

var FfiConverterPsetINSTANCE = FfiConverterPset{}

func (c FfiConverterPset) Lift(pointer unsafe.Pointer) *Pset {
	result := &Pset{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_pset(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_pset(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Pset).Destroy)
	return result
}

func (c FfiConverterPset) Read(reader io.Reader) *Pset {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterPset) Lower(value *Pset) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Pset")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterPset) Write(writer io.Writer, value *Pset) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerPset struct{}

func (_ FfiDestroyerPset) Destroy(value *Pset) {
	value.Destroy()
}

type PsetBalanceInterface interface {
	Balances() map[AssetId]int64
	Fee() uint64
	Recipients() []*Recipient
}
type PsetBalance struct {
	ffiObject FfiObject
}

func (_self *PsetBalance) Balances() map[AssetId]int64 {
	_pointer := _self.ffiObject.incrementPointer("*PsetBalance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterMapTypeAssetIdInt64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetbalance_balances(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *PsetBalance) Fee() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*PsetBalance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_psetbalance_fee(
			_pointer, _uniffiStatus)
	}))
}

func (_self *PsetBalance) Recipients() []*Recipient {
	_pointer := _self.ffiObject.incrementPointer("*PsetBalance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceRecipientINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetbalance_recipients(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *PsetBalance) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterPsetBalance struct{}

var FfiConverterPsetBalanceINSTANCE = FfiConverterPsetBalance{}

func (c FfiConverterPsetBalance) Lift(pointer unsafe.Pointer) *PsetBalance {
	result := &PsetBalance{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_psetbalance(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_psetbalance(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*PsetBalance).Destroy)
	return result
}

func (c FfiConverterPsetBalance) Read(reader io.Reader) *PsetBalance {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterPsetBalance) Lower(value *PsetBalance) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*PsetBalance")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterPsetBalance) Write(writer io.Writer, value *PsetBalance) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerPsetBalance struct{}

func (_ FfiDestroyerPsetBalance) Destroy(value *PsetBalance) {
	value.Destroy()
}

// The details of a Partially Signed Elements Transaction:
//
// - the net balance from the point of view of the wallet
// - the available and missing signatures for each input
// - for issuances and reissuances transactions contains the issuance or reissuance details
type PsetDetailsInterface interface {
	// Return the balance of the PSET from the point of view of the wallet
	// that generated this via `psetDetails()`
	Balance() *PsetBalance
	// Set of fingerprints for which the PSET has a signature
	FingerprintsHas() []string
	// Set of fingerprints for which the PSET is missing a signature
	FingerprintsMissing() []string
	// Return an element for every input that could possibly be a issuance or a reissuance
	InputsIssuances() []*Issuance
	// For each input its existing or missing signatures
	Signatures() []*PsetSignatures
}

// The details of a Partially Signed Elements Transaction:
//
// - the net balance from the point of view of the wallet
// - the available and missing signatures for each input
// - for issuances and reissuances transactions contains the issuance or reissuance details
type PsetDetails struct {
	ffiObject FfiObject
}

// Return the balance of the PSET from the point of view of the wallet
// that generated this via `psetDetails()`
func (_self *PsetDetails) Balance() *PsetBalance {
	_pointer := _self.ffiObject.incrementPointer("*PsetDetails")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterPsetBalanceINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_psetdetails_balance(
			_pointer, _uniffiStatus)
	}))
}

// Set of fingerprints for which the PSET has a signature
func (_self *PsetDetails) FingerprintsHas() []string {
	_pointer := _self.ffiObject.incrementPointer("*PsetDetails")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetdetails_fingerprints_has(
				_pointer, _uniffiStatus),
		}
	}))
}

// Set of fingerprints for which the PSET is missing a signature
func (_self *PsetDetails) FingerprintsMissing() []string {
	_pointer := _self.ffiObject.incrementPointer("*PsetDetails")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetdetails_fingerprints_missing(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return an element for every input that could possibly be a issuance or a reissuance
func (_self *PsetDetails) InputsIssuances() []*Issuance {
	_pointer := _self.ffiObject.incrementPointer("*PsetDetails")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceIssuanceINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetdetails_inputs_issuances(
				_pointer, _uniffiStatus),
		}
	}))
}

// For each input its existing or missing signatures
func (_self *PsetDetails) Signatures() []*PsetSignatures {
	_pointer := _self.ffiObject.incrementPointer("*PsetDetails")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequencePsetSignaturesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetdetails_signatures(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *PsetDetails) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterPsetDetails struct{}

var FfiConverterPsetDetailsINSTANCE = FfiConverterPsetDetails{}

func (c FfiConverterPsetDetails) Lift(pointer unsafe.Pointer) *PsetDetails {
	result := &PsetDetails{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_psetdetails(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_psetdetails(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*PsetDetails).Destroy)
	return result
}

func (c FfiConverterPsetDetails) Read(reader io.Reader) *PsetDetails {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterPsetDetails) Lower(value *PsetDetails) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*PsetDetails")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterPsetDetails) Write(writer io.Writer, value *PsetDetails) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerPsetDetails struct{}

func (_ FfiDestroyerPsetDetails) Destroy(value *PsetDetails) {
	value.Destroy()
}

// PSET input (read-only)
type PsetInputInterface interface {
	// If the input has a (re)issuance, the issuance object.
	Issuance() **Issuance
	// If the input has an issuance, the asset id.
	IssuanceAsset() *AssetId
	// If the input has an issuance, returns (asset_id, token_id).
	// Returns `None` if the input has no issuance.
	IssuanceIds() *[]AssetId
	// If the input has an issuance, the token id.
	IssuanceToken() *AssetId
	// Prevout scriptpubkey of the input.
	PreviousScriptPubkey() **Script
	// Prevout TXID of the input.
	PreviousTxid() *Txid
	// Prevout vout of the input.
	PreviousVout() uint32
	// Redeem script of the input.
	RedeemScript() **Script
	// Input sighash.
	Sighash() uint32
}

// PSET input (read-only)
type PsetInput struct {
	ffiObject FfiObject
}

// If the input has a (re)issuance, the issuance object.
func (_self *PsetInput) Issuance() **Issuance {
	_pointer := _self.ffiObject.incrementPointer("*PsetInput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalIssuanceINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetinput_issuance(
				_pointer, _uniffiStatus),
		}
	}))
}

// If the input has an issuance, the asset id.
func (_self *PsetInput) IssuanceAsset() *AssetId {
	_pointer := _self.ffiObject.incrementPointer("*PsetInput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalTypeAssetIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetinput_issuance_asset(
				_pointer, _uniffiStatus),
		}
	}))
}

// If the input has an issuance, returns (asset_id, token_id).
// Returns `None` if the input has no issuance.
func (_self *PsetInput) IssuanceIds() *[]AssetId {
	_pointer := _self.ffiObject.incrementPointer("*PsetInput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalSequenceTypeAssetIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetinput_issuance_ids(
				_pointer, _uniffiStatus),
		}
	}))
}

// If the input has an issuance, the token id.
func (_self *PsetInput) IssuanceToken() *AssetId {
	_pointer := _self.ffiObject.incrementPointer("*PsetInput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalTypeAssetIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetinput_issuance_token(
				_pointer, _uniffiStatus),
		}
	}))
}

// Prevout scriptpubkey of the input.
func (_self *PsetInput) PreviousScriptPubkey() **Script {
	_pointer := _self.ffiObject.incrementPointer("*PsetInput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalScriptINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetinput_previous_script_pubkey(
				_pointer, _uniffiStatus),
		}
	}))
}

// Prevout TXID of the input.
func (_self *PsetInput) PreviousTxid() *Txid {
	_pointer := _self.ffiObject.incrementPointer("*PsetInput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTxidINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_psetinput_previous_txid(
			_pointer, _uniffiStatus)
	}))
}

// Prevout vout of the input.
func (_self *PsetInput) PreviousVout() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*PsetInput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_psetinput_previous_vout(
			_pointer, _uniffiStatus)
	}))
}

// Redeem script of the input.
func (_self *PsetInput) RedeemScript() **Script {
	_pointer := _self.ffiObject.incrementPointer("*PsetInput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalScriptINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetinput_redeem_script(
				_pointer, _uniffiStatus),
		}
	}))
}

// Input sighash.
func (_self *PsetInput) Sighash() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*PsetInput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_psetinput_sighash(
			_pointer, _uniffiStatus)
	}))
}
func (object *PsetInput) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterPsetInput struct{}

var FfiConverterPsetInputINSTANCE = FfiConverterPsetInput{}

func (c FfiConverterPsetInput) Lift(pointer unsafe.Pointer) *PsetInput {
	result := &PsetInput{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_psetinput(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_psetinput(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*PsetInput).Destroy)
	return result
}

func (c FfiConverterPsetInput) Read(reader io.Reader) *PsetInput {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterPsetInput) Lower(value *PsetInput) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*PsetInput")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterPsetInput) Write(writer io.Writer, value *PsetInput) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerPsetInput struct{}

func (_ FfiDestroyerPsetInput) Destroy(value *PsetInput) {
	value.Destroy()
}

// PSET output (read-only)
type PsetOutputInterface interface {
	// Get the explicit amount, if set.
	Amount() *uint64
	// Get the explicit asset ID, if set.
	Asset() *AssetId
	// Get the blinder index, if set.
	BlinderIndex() *uint32
	// Get the script pubkey.
	ScriptPubkey() *Script
}

// PSET output (read-only)
type PsetOutput struct {
	ffiObject FfiObject
}

// Get the explicit amount, if set.
func (_self *PsetOutput) Amount() *uint64 {
	_pointer := _self.ffiObject.incrementPointer("*PsetOutput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetoutput_amount(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the explicit asset ID, if set.
func (_self *PsetOutput) Asset() *AssetId {
	_pointer := _self.ffiObject.incrementPointer("*PsetOutput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalTypeAssetIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetoutput_asset(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the blinder index, if set.
func (_self *PsetOutput) BlinderIndex() *uint32 {
	_pointer := _self.ffiObject.incrementPointer("*PsetOutput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetoutput_blinder_index(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the script pubkey.
func (_self *PsetOutput) ScriptPubkey() *Script {
	_pointer := _self.ffiObject.incrementPointer("*PsetOutput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterScriptINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_psetoutput_script_pubkey(
			_pointer, _uniffiStatus)
	}))
}
func (object *PsetOutput) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterPsetOutput struct{}

var FfiConverterPsetOutputINSTANCE = FfiConverterPsetOutput{}

func (c FfiConverterPsetOutput) Lift(pointer unsafe.Pointer) *PsetOutput {
	result := &PsetOutput{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_psetoutput(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_psetoutput(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*PsetOutput).Destroy)
	return result
}

func (c FfiConverterPsetOutput) Read(reader io.Reader) *PsetOutput {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterPsetOutput) Lower(value *PsetOutput) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*PsetOutput")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterPsetOutput) Write(writer io.Writer, value *PsetOutput) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerPsetOutput struct{}

func (_ FfiDestroyerPsetOutput) Destroy(value *PsetOutput) {
	value.Destroy()
}

type PsetSignaturesInterface interface {
	HasSignature() map[string]string
	MissingSignature() map[string]string
}
type PsetSignatures struct {
	ffiObject FfiObject
}

func (_self *PsetSignatures) HasSignature() map[string]string {
	_pointer := _self.ffiObject.incrementPointer("*PsetSignatures")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterMapStringStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetsignatures_has_signature(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *PsetSignatures) MissingSignature() map[string]string {
	_pointer := _self.ffiObject.incrementPointer("*PsetSignatures")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterMapStringStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_psetsignatures_missing_signature(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *PsetSignatures) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterPsetSignatures struct{}

var FfiConverterPsetSignaturesINSTANCE = FfiConverterPsetSignatures{}

func (c FfiConverterPsetSignatures) Lift(pointer unsafe.Pointer) *PsetSignatures {
	result := &PsetSignatures{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_psetsignatures(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_psetsignatures(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*PsetSignatures).Destroy)
	return result
}

func (c FfiConverterPsetSignatures) Read(reader io.Reader) *PsetSignatures {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterPsetSignatures) Lower(value *PsetSignatures) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*PsetSignatures")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterPsetSignatures) Write(writer io.Writer, value *PsetSignatures) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerPsetSignatures struct{}

func (_ FfiDestroyerPsetSignatures) Destroy(value *PsetSignatures) {
	value.Destroy()
}

// Builder for creating swap quotes
type QuoteBuilderInterface interface {
	// Build the quote, calculating fees and receive amount
	Build() (Quote, error)
	// Set the destination asset for the swap
	Receive(asset SwapAsset) error
	// Set the source asset for the swap
	Send(asset SwapAsset) error
}

// Builder for creating swap quotes
type QuoteBuilder struct {
	ffiObject FfiObject
}

// Build the quote, calculating fees and receive amount
func (_self *QuoteBuilder) Build() (Quote, error) {
	_pointer := _self.ffiObject.incrementPointer("*QuoteBuilder")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_quotebuilder_build(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue Quote
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterQuoteINSTANCE.Lift(_uniffiRV), nil
	}
}

// Set the destination asset for the swap
func (_self *QuoteBuilder) Receive(asset SwapAsset) error {
	_pointer := _self.ffiObject.incrementPointer("*QuoteBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_quotebuilder_receive(
			_pointer, FfiConverterSwapAssetINSTANCE.Lower(asset), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Set the source asset for the swap
func (_self *QuoteBuilder) Send(asset SwapAsset) error {
	_pointer := _self.ffiObject.incrementPointer("*QuoteBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_quotebuilder_send(
			_pointer, FfiConverterSwapAssetINSTANCE.Lower(asset), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}
func (object *QuoteBuilder) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterQuoteBuilder struct{}

var FfiConverterQuoteBuilderINSTANCE = FfiConverterQuoteBuilder{}

func (c FfiConverterQuoteBuilder) Lift(pointer unsafe.Pointer) *QuoteBuilder {
	result := &QuoteBuilder{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_quotebuilder(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_quotebuilder(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*QuoteBuilder).Destroy)
	return result
}

func (c FfiConverterQuoteBuilder) Read(reader io.Reader) *QuoteBuilder {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterQuoteBuilder) Lower(value *QuoteBuilder) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*QuoteBuilder")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterQuoteBuilder) Write(writer io.Writer, value *QuoteBuilder) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerQuoteBuilder struct{}

func (_ FfiDestroyerQuoteBuilder) Destroy(value *QuoteBuilder) {
	value.Destroy()
}

type RecipientInterface interface {
	Address() **Address
	Asset() *AssetId
	Value() *uint64
	Vout() uint32
}
type Recipient struct {
	ffiObject FfiObject
}

func (_self *Recipient) Address() **Address {
	_pointer := _self.ffiObject.incrementPointer("*Recipient")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalAddressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_recipient_address(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Recipient) Asset() *AssetId {
	_pointer := _self.ffiObject.incrementPointer("*Recipient")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalTypeAssetIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_recipient_asset(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Recipient) Value() *uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Recipient")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_recipient_value(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Recipient) Vout() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*Recipient")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_recipient_vout(
			_pointer, _uniffiStatus)
	}))
}
func (object *Recipient) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterRecipient struct{}

var FfiConverterRecipientINSTANCE = FfiConverterRecipient{}

func (c FfiConverterRecipient) Lift(pointer unsafe.Pointer) *Recipient {
	result := &Recipient{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_recipient(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_recipient(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Recipient).Destroy)
	return result
}

func (c FfiConverterRecipient) Read(reader io.Reader) *Recipient {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterRecipient) Lower(value *Recipient) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Recipient")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterRecipient) Write(writer io.Writer, value *Recipient) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerRecipient struct{}

func (_ FfiDestroyerRecipient) Destroy(value *Recipient) {
	value.Destroy()
}

// A Liquid script
type ScriptInterface interface {
	// Return the consensus encoded bytes of the script.
	Bytes() []byte
	// Whether a script pubkey is provably unspendable (like a burn script)
	IsProvablyUnspendable() bool
	// Returns SHA256 of the script's consensus bytes.
	//
	// Returns an equivalent value to the `jet::input_script_hash(index)`/`jet::output_script_hash(index)`.
	JetSha256Hex() Hex
	// Return the string representation of the script showing op codes and their arguments.
	// For example: "OP_0 OP_PUSHBYTES_32 d2e99f0c38089c08e5e1080ff6658c6075afaa7699d384333d956c470881afde"
	ToAsm() string
}

// A Liquid script
type Script struct {
	ffiObject FfiObject
}

// Construct a Script object from its hex representation.
// To create the hex representation of a script use `to_string()`.
func NewScript(hex Hex) (*Script, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_script_new(FfiConverterTypeHexINSTANCE.Lower(hex), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Script
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterScriptINSTANCE.Lift(_uniffiRV), nil
	}
}

// Create an empty script (for fee outputs).
func ScriptEmpty() *Script {
	return FfiConverterScriptINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_script_empty(_uniffiStatus)
	}))
}

// Create an OP_RETURN script (for burn outputs).
func ScriptNewOpReturn(data []byte) *Script {
	return FfiConverterScriptINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_script_new_op_return(FfiConverterBytesINSTANCE.Lower(data), _uniffiStatus)
	}))
}

// Return the consensus encoded bytes of the script.
func (_self *Script) Bytes() []byte {
	_pointer := _self.ffiObject.incrementPointer("*Script")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_script_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

// Whether a script pubkey is provably unspendable (like a burn script)
func (_self *Script) IsProvablyUnspendable() bool {
	_pointer := _self.ffiObject.incrementPointer("*Script")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_script_is_provably_unspendable(
			_pointer, _uniffiStatus)
	}))
}

// Returns SHA256 of the script's consensus bytes.
//
// Returns an equivalent value to the `jet::input_script_hash(index)`/`jet::output_script_hash(index)`.
func (_self *Script) JetSha256Hex() Hex {
	_pointer := _self.ffiObject.incrementPointer("*Script")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeHexINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_script_jet_sha256_hex(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the string representation of the script showing op codes and their arguments.
// For example: "OP_0 OP_PUSHBYTES_32 d2e99f0c38089c08e5e1080ff6658c6075afaa7699d384333d956c470881afde"
func (_self *Script) ToAsm() string {
	_pointer := _self.ffiObject.incrementPointer("*Script")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_script_to_asm(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Script) String() string {
	_pointer := _self.ffiObject.incrementPointer("*Script")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_script_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *Script) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterScript struct{}

var FfiConverterScriptINSTANCE = FfiConverterScript{}

func (c FfiConverterScript) Lift(pointer unsafe.Pointer) *Script {
	result := &Script{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_script(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_script(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Script).Destroy)
	return result
}

func (c FfiConverterScript) Read(reader io.Reader) *Script {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterScript) Lower(value *Script) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Script")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterScript) Write(writer io.Writer, value *Script) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerScript struct{}

func (_ FfiDestroyerScript) Destroy(value *Script) {
	value.Destroy()
}

// A secret key
type SecretKeyInterface interface {
	// Returns the bytes of the secret key, the bytes can be used to create a `SecretKey` with `from_bytes()`
	Bytes() []byte
	// Sign the given `pset`
	Sign(pset *Pset) (*Pset, error)
}

// A secret key
type SecretKey struct {
	ffiObject FfiObject
}

// Creates a `SecretKey` from a byte array
//
// The bytes can be used to create a `SecretKey` with `from_bytes()`
func SecretKeyFromBytes(bytes []byte) (*SecretKey, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_secretkey_from_bytes(FfiConverterBytesINSTANCE.Lower(bytes), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *SecretKey
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSecretKeyINSTANCE.Lift(_uniffiRV), nil
	}
}

// Creates a `SecretKey` from a WIF (Wallet Import Format) string
func SecretKeyFromWif(wif string) (*SecretKey, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_secretkey_from_wif(FfiConverterStringINSTANCE.Lower(wif), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *SecretKey
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSecretKeyINSTANCE.Lift(_uniffiRV), nil
	}
}

// Returns the bytes of the secret key, the bytes can be used to create a `SecretKey` with `from_bytes()`
func (_self *SecretKey) Bytes() []byte {
	_pointer := _self.ffiObject.incrementPointer("*SecretKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_secretkey_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

// Sign the given `pset`
func (_self *SecretKey) Sign(pset *Pset) (*Pset, error) {
	_pointer := _self.ffiObject.incrementPointer("*SecretKey")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_secretkey_sign(
			_pointer, FfiConverterPsetINSTANCE.Lower(pset), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Pset
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPsetINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *SecretKey) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterSecretKey struct{}

var FfiConverterSecretKeyINSTANCE = FfiConverterSecretKey{}

func (c FfiConverterSecretKey) Lift(pointer unsafe.Pointer) *SecretKey {
	result := &SecretKey{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_secretkey(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_secretkey(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*SecretKey).Destroy)
	return result
}

func (c FfiConverterSecretKey) Read(reader io.Reader) *SecretKey {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterSecretKey) Lower(value *SecretKey) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*SecretKey")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterSecretKey) Write(writer io.Writer, value *SecretKey) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerSecretKey struct{}

func (_ FfiDestroyerSecretKey) Destroy(value *SecretKey) {
	value.Destroy()
}

// A Software signer, wrapper over [`lwk_signer::SwSigner`]
type SignerInterface interface {
	// AMP0 account xpub
	Amp0AccountXpub(account uint32) (string, error)
	// AMP0 sign login challenge
	Amp0SignChallenge(challenge string) (string, error)
	// AMP0 signer data for login
	Amp0SignerData() (*Amp0SignerData, error)
	// Derive a BIP85 mnemonic from this signer
	//
	// # Arguments
	// * `index` - The index for the derived mnemonic (0-based)
	// * `word_count` - The number of words in the derived mnemonic (12 or 24)
	//
	// # Returns
	// * `Ok(Mnemonic)` - The derived BIP85 mnemonic
	// * `Err(LwkError)` - If BIP85 derivation fails
	//
	// # Example
	// ```python
	// signer = Signer.new(mnemonic, network)
	// derived_mnemonic = signer.derive_bip85_mnemonic(0, 12)
	// ```
	DeriveBip85Mnemonic(index uint32, wordCount uint32) (*Mnemonic, error)
	// Return the signer fingerprint
	Fingerprint() (string, error)
	// Return keyorigin and xpub, like "[73c5da0a/84h/1h/0h]tpub..."
	KeyoriginXpub(bip *Bip) (string, error)
	// Get the mnemonic of the signer
	Mnemonic() (*Mnemonic, error)
	// Sign the given `pset`
	//
	// Note from an API perspective it would be better to consume the `pset` parameter so it would
	// be clear the signed PSET is the returned one, but it's not possible with uniffi bindings
	Sign(pset *Pset) (*Pset, error)
	// Generate a singlesig descriptor with the given parameters
	SinglesigDesc(scriptVariant Singlesig, blindingVariant DescriptorBlindingKey) (*WolletDescriptor, error)
	// Return the witness public key hash, slip77 descriptor of this signer
	WpkhSlip77Descriptor() (*WolletDescriptor, error)
}

// A Software signer, wrapper over [`lwk_signer::SwSigner`]
type Signer struct {
	ffiObject FfiObject
}

// Construct a software signer
func NewSigner(mnemonic *Mnemonic, network *Network) (*Signer, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_signer_new(FfiConverterMnemonicINSTANCE.Lower(mnemonic), FfiConverterNetworkINSTANCE.Lower(network), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Signer
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSignerINSTANCE.Lift(_uniffiRV), nil
	}
}

// Generate a new random software signer
func SignerRandom(network *Network) (*Signer, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_signer_random(FfiConverterNetworkINSTANCE.Lower(network), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Signer
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSignerINSTANCE.Lift(_uniffiRV), nil
	}
}

// AMP0 account xpub
func (_self *Signer) Amp0AccountXpub(account uint32) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_signer_amp0_account_xpub(
				_pointer, FfiConverterUint32INSTANCE.Lower(account), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// AMP0 sign login challenge
func (_self *Signer) Amp0SignChallenge(challenge string) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_signer_amp0_sign_challenge(
				_pointer, FfiConverterStringINSTANCE.Lower(challenge), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// AMP0 signer data for login
func (_self *Signer) Amp0SignerData() (*Amp0SignerData, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_signer_amp0_signer_data(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Amp0SignerData
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAmp0SignerDataINSTANCE.Lift(_uniffiRV), nil
	}
}

// Derive a BIP85 mnemonic from this signer
//
// # Arguments
// * `index` - The index for the derived mnemonic (0-based)
// * `word_count` - The number of words in the derived mnemonic (12 or 24)
//
// # Returns
// * `Ok(Mnemonic)` - The derived BIP85 mnemonic
// * `Err(LwkError)` - If BIP85 derivation fails
//
// # Example
// ```python
// signer = Signer.new(mnemonic, network)
// derived_mnemonic = signer.derive_bip85_mnemonic(0, 12)
// ```
func (_self *Signer) DeriveBip85Mnemonic(index uint32, wordCount uint32) (*Mnemonic, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_signer_derive_bip85_mnemonic(
			_pointer, FfiConverterUint32INSTANCE.Lower(index), FfiConverterUint32INSTANCE.Lower(wordCount), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Mnemonic
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterMnemonicINSTANCE.Lift(_uniffiRV), nil
	}
}

// Return the signer fingerprint
func (_self *Signer) Fingerprint() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_signer_fingerprint(
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

// Return keyorigin and xpub, like "[73c5da0a/84h/1h/0h]tpub..."
func (_self *Signer) KeyoriginXpub(bip *Bip) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_signer_keyorigin_xpub(
				_pointer, FfiConverterBipINSTANCE.Lower(bip), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the mnemonic of the signer
func (_self *Signer) Mnemonic() (*Mnemonic, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_signer_mnemonic(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Mnemonic
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterMnemonicINSTANCE.Lift(_uniffiRV), nil
	}
}

// Sign the given `pset`
//
// Note from an API perspective it would be better to consume the `pset` parameter so it would
// be clear the signed PSET is the returned one, but it's not possible with uniffi bindings
func (_self *Signer) Sign(pset *Pset) (*Pset, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_signer_sign(
			_pointer, FfiConverterPsetINSTANCE.Lower(pset), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Pset
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPsetINSTANCE.Lift(_uniffiRV), nil
	}
}

// Generate a singlesig descriptor with the given parameters
func (_self *Signer) SinglesigDesc(scriptVariant Singlesig, blindingVariant DescriptorBlindingKey) (*WolletDescriptor, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_signer_singlesig_desc(
			_pointer, FfiConverterSinglesigINSTANCE.Lower(scriptVariant), FfiConverterDescriptorBlindingKeyINSTANCE.Lower(blindingVariant), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *WolletDescriptor
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWolletDescriptorINSTANCE.Lift(_uniffiRV), nil
	}
}

// Return the witness public key hash, slip77 descriptor of this signer
func (_self *Signer) WpkhSlip77Descriptor() (*WolletDescriptor, error) {
	_pointer := _self.ffiObject.incrementPointer("*Signer")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_signer_wpkh_slip77_descriptor(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *WolletDescriptor
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWolletDescriptorINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *Signer) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterSigner struct{}

var FfiConverterSignerINSTANCE = FfiConverterSigner{}

func (c FfiConverterSigner) Lift(pointer unsafe.Pointer) *Signer {
	result := &Signer{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_signer(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_signer(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Signer).Destroy)
	return result
}

func (c FfiConverterSigner) Read(reader io.Reader) *Signer {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterSigner) Lower(value *Signer) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Signer")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterSigner) Write(writer io.Writer, value *Signer) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerSigner struct{}

func (_ FfiDestroyerSigner) Destroy(value *Signer) {
	value.Destroy()
}

type SwapListInterface interface {
}
type SwapList struct {
	ffiObject FfiObject
}

func (_self *SwapList) String() string {
	_pointer := _self.ffiObject.incrementPointer("*SwapList")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_swaplist_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *SwapList) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterSwapList struct{}

var FfiConverterSwapListINSTANCE = FfiConverterSwapList{}

func (c FfiConverterSwapList) Lift(pointer unsafe.Pointer) *SwapList {
	result := &SwapList{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_swaplist(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_swaplist(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*SwapList).Destroy)
	return result
}

func (c FfiConverterSwapList) Read(reader io.Reader) *SwapList {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterSwapList) Lower(value *SwapList) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*SwapList")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterSwapList) Write(writer io.Writer, value *SwapList) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerSwapList struct{}

func (_ FfiDestroyerSwapList) Destroy(value *SwapList) {
	value.Destroy()
}

// A Liquid transaction
type TransactionInterface interface {
	// Return the consensus encoded bytes of the transaction.
	//
	// Deprecated: use `to_bytes()` instead.
	Bytes() []byte
	// Returns the "discount virtual size" of this transaction.
	DiscountVsize() uint32
	// Return the fee of the transaction in the given asset.
	// At the moment the only asset that can be used as fee is the policy asset (LBTC for mainnet).
	Fee(policyAsset AssetId) uint64
	// Return a copy of the inputs of the transaction.
	Inputs() []*TxIn
	// Return a copy of the outputs of the transaction.
	Outputs() []*TxOut
	// Return the consensus encoded bytes of the transaction.
	ToBytes() []byte
	// Return the transaction identifier.
	Txid() *Txid
	// Verify that the transaction has correctly calculated blinding factors and they CT
	// verification equation holds.
	//
	// This is *NOT* a complete Transaction verification check
	// It does *NOT* check whether input witness/script satisfies the script pubkey, or
	// inputs are double-spent and other consensus checks.
	//
	// This method only checks if the `Transaction` verification equation for Confidential
	// transactions holds. i.e Sum of inputs = Sum of outputs + fees.
	//
	// And the corresponding surjection/rangeproofs are correct.
	// For checking of surjection proofs and amounts, spent_utxos parameter
	// should contain information about the prevouts. Note that the order of
	// spent_utxos should be consistent with transaction inputs.
	VerifyTxAmtProofs(utxos []*TxOut) error
}

// A Liquid transaction
type Transaction struct {
	ffiObject FfiObject
}

// Construct a Transaction object from its hex representation.
// To create the hex representation of a transaction use `to_string()`.
//
// Deprecated: use `from_string()` instead.
func NewTransaction(hex Hex) (*Transaction, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_transaction_new(FfiConverterTypeHexINSTANCE.Lower(hex), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Transaction
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTransactionINSTANCE.Lift(_uniffiRV), nil
	}
}

// Construct a Transaction object from its bytes.
func TransactionFromBytes(bytes []byte) (*Transaction, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_transaction_from_bytes(FfiConverterBytesINSTANCE.Lower(bytes), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Transaction
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTransactionINSTANCE.Lift(_uniffiRV), nil
	}
}

// Construct a Transaction object from its canonical string representation.
// To create the string representation of a transaction use `to_string()`.
func TransactionFromString(s string) (*Transaction, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_transaction_from_string(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Transaction
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTransactionINSTANCE.Lift(_uniffiRV), nil
	}
}

// Return the consensus encoded bytes of the transaction.
//
// Deprecated: use `to_bytes()` instead.
func (_self *Transaction) Bytes() []byte {
	_pointer := _self.ffiObject.incrementPointer("*Transaction")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_transaction_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns the "discount virtual size" of this transaction.
func (_self *Transaction) DiscountVsize() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*Transaction")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_transaction_discount_vsize(
			_pointer, _uniffiStatus)
	}))
}

// Return the fee of the transaction in the given asset.
// At the moment the only asset that can be used as fee is the policy asset (LBTC for mainnet).
func (_self *Transaction) Fee(policyAsset AssetId) uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Transaction")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_transaction_fee(
			_pointer, FfiConverterTypeAssetIdINSTANCE.Lower(policyAsset), _uniffiStatus)
	}))
}

// Return a copy of the inputs of the transaction.
func (_self *Transaction) Inputs() []*TxIn {
	_pointer := _self.ffiObject.incrementPointer("*Transaction")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceTxInINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_transaction_inputs(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return a copy of the outputs of the transaction.
func (_self *Transaction) Outputs() []*TxOut {
	_pointer := _self.ffiObject.incrementPointer("*Transaction")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceTxOutINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_transaction_outputs(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the consensus encoded bytes of the transaction.
func (_self *Transaction) ToBytes() []byte {
	_pointer := _self.ffiObject.incrementPointer("*Transaction")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_transaction_to_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the transaction identifier.
func (_self *Transaction) Txid() *Txid {
	_pointer := _self.ffiObject.incrementPointer("*Transaction")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTxidINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_transaction_txid(
			_pointer, _uniffiStatus)
	}))
}

// Verify that the transaction has correctly calculated blinding factors and they CT
// verification equation holds.
//
// This is *NOT* a complete Transaction verification check
// It does *NOT* check whether input witness/script satisfies the script pubkey, or
// inputs are double-spent and other consensus checks.
//
// This method only checks if the `Transaction` verification equation for Confidential
// transactions holds. i.e Sum of inputs = Sum of outputs + fees.
//
// And the corresponding surjection/rangeproofs are correct.
// For checking of surjection proofs and amounts, spent_utxos parameter
// should contain information about the prevouts. Note that the order of
// spent_utxos should be consistent with transaction inputs.
func (_self *Transaction) VerifyTxAmtProofs(utxos []*TxOut) error {
	_pointer := _self.ffiObject.incrementPointer("*Transaction")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_transaction_verify_tx_amt_proofs(
			_pointer, FfiConverterSequenceTxOutINSTANCE.Lower(utxos), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *Transaction) String() string {
	_pointer := _self.ffiObject.incrementPointer("*Transaction")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_transaction_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *Transaction) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterTransaction struct{}

var FfiConverterTransactionINSTANCE = FfiConverterTransaction{}

func (c FfiConverterTransaction) Lift(pointer unsafe.Pointer) *Transaction {
	result := &Transaction{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_transaction(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_transaction(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Transaction).Destroy)
	return result
}

func (c FfiConverterTransaction) Read(reader io.Reader) *Transaction {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterTransaction) Lower(value *Transaction) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Transaction")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterTransaction) Write(writer io.Writer, value *Transaction) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerTransaction struct{}

func (_ FfiDestroyerTransaction) Destroy(value *Transaction) {
	value.Destroy()
}

// Wrapper over [`lwk_wollet::TxBuilder`]
type TxBuilderInterface interface {
	// Burn satoshi units of the given asset
	AddBurn(satoshi uint64, asset AssetId) error
	// Add explicit recipient
	AddExplicitRecipient(address *Address, satoshi uint64, asset AssetId) error
	// Adds external UTXOs
	//
	// Note: unblinded UTXOs with the same scriptpubkeys as the wallet, are considered external.
	AddExternalUtxos(utxos []*ExternalUtxo) error
	// Add input rangeproofs
	AddInputRangeproofs(addRangeproofs bool) error
	// Add a recipient receiving L-BTC
	AddLbtcRecipient(address *Address, satoshi uint64) error
	// Add a recipient receiving the given asset
	AddRecipient(address *Address, satoshi uint64, asset AssetId) error
	// Sets the address to drain excess L-BTC to
	DrainLbtcTo(address *Address) error
	// Select all available L-BTC inputs
	DrainLbtcWallet() error
	// Fee rate in sats/kvb
	// Multiply sats/vb value by 1000 i.e. 1.0 sat/byte = 1000.0 sat/kvb
	FeeRate(rate *float32) error
	// Build the transaction
	Finish(wollet *Wollet) (*Pset, error)
	// Build the transaction
	FinishForAmp0(wollet *Wollet) (*Amp0Pset, error)
	// Issue an asset
	//
	// There will be `asset_sats` units of this asset that will be received by
	// `asset_receiver` if it's set, otherwise to an address of the wallet generating the issuance.
	//
	// There will be `token_sats` reissuance tokens that allow token holder to reissue the created
	// asset. Reissuance token will be received by `token_receiver` if it's some, or to an
	// address of the wallet generating the issuance if none.
	//
	// If a `contract` is provided, it's metadata will be committed in the generated asset id.
	//
	// Can't be used if `reissue_asset` has been called
	IssueAsset(assetSats uint64, assetReceiver **Address, tokenSats uint64, tokenReceiver **Address, contract **Contract) error
	// Set data to create a PSET from which you
	// can create a LiquiDEX proposal
	LiquidexMake(utxo *OutPoint, address *Address, amount uint64, asset AssetId) error
	// Set data to take LiquiDEX proposals
	LiquidexTake(proposals []*ValidatedLiquidexProposal) error
	// Reissue an asset
	//
	// reissue the asset defined by `asset_to_reissue`, provided the reissuance token is owned
	// by the wallet generating te reissuance.
	//
	// Generated transaction will create `satoshi_to_reissue` new asset units, and they will be
	// sent to the provided `asset_receiver` address if some, or to an address from the wallet
	// generating the reissuance transaction if none.
	//
	// If the issuance transaction does not involve this wallet,
	// pass the issuance transaction in `issuance_tx`.
	ReissueAsset(assetToReissue AssetId, satoshiToReissue uint64, assetReceiver **Address, issuanceTx **Transaction) error
	// Switch to manual coin selection by giving a list of internal UTXOs to use.
	//
	// All passed UTXOs are added to the transaction.
	// No other wallet UTXO is added to the transaction, caller is supposed to add enough UTXOs to
	// cover for all recipients and fees.
	//
	// This method never fails, any error will be raised in [`TxBuilder::finish`].
	//
	// Possible errors:
	// * OutPoint doesn't belong to the wallet
	// * Insufficient funds (remember to include L-BTC utxos for fees)
	SetWalletUtxos(utxos []*OutPoint) error
}

// Wrapper over [`lwk_wollet::TxBuilder`]
type TxBuilder struct {
	ffiObject FfiObject
}

// Construct a transaction builder
func NewTxBuilder(network *Network) *TxBuilder {
	return FfiConverterTxBuilderINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_txbuilder_new(FfiConverterNetworkINSTANCE.Lower(network), _uniffiStatus)
	}))
}

// Burn satoshi units of the given asset
func (_self *TxBuilder) AddBurn(satoshi uint64, asset AssetId) error {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_txbuilder_add_burn(
			_pointer, FfiConverterUint64INSTANCE.Lower(satoshi), FfiConverterTypeAssetIdINSTANCE.Lower(asset), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Add explicit recipient
func (_self *TxBuilder) AddExplicitRecipient(address *Address, satoshi uint64, asset AssetId) error {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_txbuilder_add_explicit_recipient(
			_pointer, FfiConverterAddressINSTANCE.Lower(address), FfiConverterUint64INSTANCE.Lower(satoshi), FfiConverterTypeAssetIdINSTANCE.Lower(asset), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Adds external UTXOs
//
// Note: unblinded UTXOs with the same scriptpubkeys as the wallet, are considered external.
func (_self *TxBuilder) AddExternalUtxos(utxos []*ExternalUtxo) error {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_txbuilder_add_external_utxos(
			_pointer, FfiConverterSequenceExternalUtxoINSTANCE.Lower(utxos), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Add input rangeproofs
func (_self *TxBuilder) AddInputRangeproofs(addRangeproofs bool) error {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_txbuilder_add_input_rangeproofs(
			_pointer, FfiConverterBoolINSTANCE.Lower(addRangeproofs), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Add a recipient receiving L-BTC
func (_self *TxBuilder) AddLbtcRecipient(address *Address, satoshi uint64) error {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_txbuilder_add_lbtc_recipient(
			_pointer, FfiConverterAddressINSTANCE.Lower(address), FfiConverterUint64INSTANCE.Lower(satoshi), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Add a recipient receiving the given asset
func (_self *TxBuilder) AddRecipient(address *Address, satoshi uint64, asset AssetId) error {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_txbuilder_add_recipient(
			_pointer, FfiConverterAddressINSTANCE.Lower(address), FfiConverterUint64INSTANCE.Lower(satoshi), FfiConverterTypeAssetIdINSTANCE.Lower(asset), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Sets the address to drain excess L-BTC to
func (_self *TxBuilder) DrainLbtcTo(address *Address) error {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_txbuilder_drain_lbtc_to(
			_pointer, FfiConverterAddressINSTANCE.Lower(address), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Select all available L-BTC inputs
func (_self *TxBuilder) DrainLbtcWallet() error {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_txbuilder_drain_lbtc_wallet(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Fee rate in sats/kvb
// Multiply sats/vb value by 1000 i.e. 1.0 sat/byte = 1000.0 sat/kvb
func (_self *TxBuilder) FeeRate(rate *float32) error {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_txbuilder_fee_rate(
			_pointer, FfiConverterOptionalFloat32INSTANCE.Lower(rate), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Build the transaction
func (_self *TxBuilder) Finish(wollet *Wollet) (*Pset, error) {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_txbuilder_finish(
			_pointer, FfiConverterWolletINSTANCE.Lower(wollet), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Pset
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPsetINSTANCE.Lift(_uniffiRV), nil
	}
}

// Build the transaction
func (_self *TxBuilder) FinishForAmp0(wollet *Wollet) (*Amp0Pset, error) {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_txbuilder_finish_for_amp0(
			_pointer, FfiConverterWolletINSTANCE.Lower(wollet), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Amp0Pset
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAmp0PsetINSTANCE.Lift(_uniffiRV), nil
	}
}

// Issue an asset
//
// There will be `asset_sats` units of this asset that will be received by
// `asset_receiver` if it's set, otherwise to an address of the wallet generating the issuance.
//
// There will be `token_sats` reissuance tokens that allow token holder to reissue the created
// asset. Reissuance token will be received by `token_receiver` if it's some, or to an
// address of the wallet generating the issuance if none.
//
// If a `contract` is provided, it's metadata will be committed in the generated asset id.
//
// Can't be used if `reissue_asset` has been called
func (_self *TxBuilder) IssueAsset(assetSats uint64, assetReceiver **Address, tokenSats uint64, tokenReceiver **Address, contract **Contract) error {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_txbuilder_issue_asset(
			_pointer, FfiConverterUint64INSTANCE.Lower(assetSats), FfiConverterOptionalAddressINSTANCE.Lower(assetReceiver), FfiConverterUint64INSTANCE.Lower(tokenSats), FfiConverterOptionalAddressINSTANCE.Lower(tokenReceiver), FfiConverterOptionalContractINSTANCE.Lower(contract), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Set data to create a PSET from which you
// can create a LiquiDEX proposal
func (_self *TxBuilder) LiquidexMake(utxo *OutPoint, address *Address, amount uint64, asset AssetId) error {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_txbuilder_liquidex_make(
			_pointer, FfiConverterOutPointINSTANCE.Lower(utxo), FfiConverterAddressINSTANCE.Lower(address), FfiConverterUint64INSTANCE.Lower(amount), FfiConverterTypeAssetIdINSTANCE.Lower(asset), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Set data to take LiquiDEX proposals
func (_self *TxBuilder) LiquidexTake(proposals []*ValidatedLiquidexProposal) error {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_txbuilder_liquidex_take(
			_pointer, FfiConverterSequenceValidatedLiquidexProposalINSTANCE.Lower(proposals), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Reissue an asset
//
// reissue the asset defined by `asset_to_reissue`, provided the reissuance token is owned
// by the wallet generating te reissuance.
//
// Generated transaction will create `satoshi_to_reissue` new asset units, and they will be
// sent to the provided `asset_receiver` address if some, or to an address from the wallet
// generating the reissuance transaction if none.
//
// If the issuance transaction does not involve this wallet,
// pass the issuance transaction in `issuance_tx`.
func (_self *TxBuilder) ReissueAsset(assetToReissue AssetId, satoshiToReissue uint64, assetReceiver **Address, issuanceTx **Transaction) error {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_txbuilder_reissue_asset(
			_pointer, FfiConverterTypeAssetIdINSTANCE.Lower(assetToReissue), FfiConverterUint64INSTANCE.Lower(satoshiToReissue), FfiConverterOptionalAddressINSTANCE.Lower(assetReceiver), FfiConverterOptionalTransactionINSTANCE.Lower(issuanceTx), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Switch to manual coin selection by giving a list of internal UTXOs to use.
//
// All passed UTXOs are added to the transaction.
// No other wallet UTXO is added to the transaction, caller is supposed to add enough UTXOs to
// cover for all recipients and fees.
//
// This method never fails, any error will be raised in [`TxBuilder::finish`].
//
// Possible errors:
// * OutPoint doesn't belong to the wallet
// * Insufficient funds (remember to include L-BTC utxos for fees)
func (_self *TxBuilder) SetWalletUtxos(utxos []*OutPoint) error {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_txbuilder_set_wallet_utxos(
			_pointer, FfiConverterSequenceOutPointINSTANCE.Lower(utxos), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *TxBuilder) String() string {
	_pointer := _self.ffiObject.incrementPointer("*TxBuilder")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_txbuilder_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *TxBuilder) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterTxBuilder struct{}

var FfiConverterTxBuilderINSTANCE = FfiConverterTxBuilder{}

func (c FfiConverterTxBuilder) Lift(pointer unsafe.Pointer) *TxBuilder {
	result := &TxBuilder{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_txbuilder(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_txbuilder(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*TxBuilder).Destroy)
	return result
}

func (c FfiConverterTxBuilder) Read(reader io.Reader) *TxBuilder {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterTxBuilder) Lower(value *TxBuilder) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*TxBuilder")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterTxBuilder) Write(writer io.Writer, value *TxBuilder) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerTxBuilder struct{}

func (_ FfiDestroyerTxBuilder) Destroy(value *TxBuilder) {
	value.Destroy()
}

// A transaction input.
type TxInInterface interface {
	// Outpoint
	Outpoint() *OutPoint
	// Get the sequence number for this input.
	Sequence() uint32
}

// A transaction input.
type TxIn struct {
	ffiObject FfiObject
}

// Outpoint
func (_self *TxIn) Outpoint() *OutPoint {
	_pointer := _self.ffiObject.incrementPointer("*TxIn")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOutPointINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_txin_outpoint(
			_pointer, _uniffiStatus)
	}))
}

// Get the sequence number for this input.
func (_self *TxIn) Sequence() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*TxIn")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_txin_sequence(
			_pointer, _uniffiStatus)
	}))
}
func (object *TxIn) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterTxIn struct{}

var FfiConverterTxInINSTANCE = FfiConverterTxIn{}

func (c FfiConverterTxIn) Lift(pointer unsafe.Pointer) *TxIn {
	result := &TxIn{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_txin(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_txin(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*TxIn).Destroy)
	return result
}

func (c FfiConverterTxIn) Read(reader io.Reader) *TxIn {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterTxIn) Lower(value *TxIn) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*TxIn")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterTxIn) Write(writer io.Writer, value *TxIn) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerTxIn struct{}

func (_ FfiDestroyerTxIn) Destroy(value *TxIn) {
	value.Destroy()
}

// A transaction output.
type TxOutInterface interface {
	// If explicit returns the asset, if confidential [None]
	Asset() *AssetId
	// Whether or not this output is a fee output
	IsFee() bool
	// Returns if at least some part of this output are blinded
	IsPartiallyBlinded() bool
	// Scriptpubkey
	ScriptPubkey() *Script
	// Unblind the output
	Unblind(secretKey *SecretKey) (*TxOutSecrets, error)
	// Unconfidential address
	UnconfidentialAddress(network *Network) **Address
	// If explicit returns the value, if confidential [None]
	Value() *uint64
}

// A transaction output.
type TxOut struct {
	ffiObject FfiObject
}

// Create a TxOut with explicit asset and value from script pubkey and asset ID.
//
// This is useful for constructing UTXOs for Simplicity transaction signing.
func TxOutFromExplicit(scriptPubkey *Script, assetId AssetId, satoshi uint64) *TxOut {
	return FfiConverterTxOutINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_txout_from_explicit(FfiConverterScriptINSTANCE.Lower(scriptPubkey), FfiConverterTypeAssetIdINSTANCE.Lower(assetId), FfiConverterUint64INSTANCE.Lower(satoshi), _uniffiStatus)
	}))
}

// If explicit returns the asset, if confidential [None]
func (_self *TxOut) Asset() *AssetId {
	_pointer := _self.ffiObject.incrementPointer("*TxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalTypeAssetIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_txout_asset(
				_pointer, _uniffiStatus),
		}
	}))
}

// Whether or not this output is a fee output
func (_self *TxOut) IsFee() bool {
	_pointer := _self.ffiObject.incrementPointer("*TxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_txout_is_fee(
			_pointer, _uniffiStatus)
	}))
}

// Returns if at least some part of this output are blinded
func (_self *TxOut) IsPartiallyBlinded() bool {
	_pointer := _self.ffiObject.incrementPointer("*TxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_txout_is_partially_blinded(
			_pointer, _uniffiStatus)
	}))
}

// Scriptpubkey
func (_self *TxOut) ScriptPubkey() *Script {
	_pointer := _self.ffiObject.incrementPointer("*TxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterScriptINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_txout_script_pubkey(
			_pointer, _uniffiStatus)
	}))
}

// Unblind the output
func (_self *TxOut) Unblind(secretKey *SecretKey) (*TxOutSecrets, error) {
	_pointer := _self.ffiObject.incrementPointer("*TxOut")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_txout_unblind(
			_pointer, FfiConverterSecretKeyINSTANCE.Lower(secretKey), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *TxOutSecrets
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxOutSecretsINSTANCE.Lift(_uniffiRV), nil
	}
}

// Unconfidential address
func (_self *TxOut) UnconfidentialAddress(network *Network) **Address {
	_pointer := _self.ffiObject.incrementPointer("*TxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalAddressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_txout_unconfidential_address(
				_pointer, FfiConverterNetworkINSTANCE.Lower(network), _uniffiStatus),
		}
	}))
}

// If explicit returns the value, if confidential [None]
func (_self *TxOut) Value() *uint64 {
	_pointer := _self.ffiObject.incrementPointer("*TxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_txout_value(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *TxOut) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterTxOut struct{}

var FfiConverterTxOutINSTANCE = FfiConverterTxOut{}

func (c FfiConverterTxOut) Lift(pointer unsafe.Pointer) *TxOut {
	result := &TxOut{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_txout(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_txout(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*TxOut).Destroy)
	return result
}

func (c FfiConverterTxOut) Read(reader io.Reader) *TxOut {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterTxOut) Lower(value *TxOut) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*TxOut")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterTxOut) Write(writer io.Writer, value *TxOut) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerTxOut struct{}

func (_ FfiDestroyerTxOut) Destroy(value *TxOut) {
	value.Destroy()
}

// Contains unblinded information such as the asset and the value of a transaction output
type TxOutSecretsInterface interface {
	// Return the asset identifier of the output.
	Asset() AssetId
	// Return the asset blinding factor as a hex string.
	//
	// Deprecated: use `asset_blinding_factor()` instead.
	AssetBf() Hex
	// Get the asset commitment
	//
	// If the output is explicit, returns the empty string
	AssetCommitment() Hex
	// Return true if the output is explicit (no blinding factors).
	IsExplicit() bool
	// Return the value of the output.
	Value() uint64
	// Return the value blinding factor as a hex string.
	//
	// Deprecated: use `value_blinding_factor()` instead.
	ValueBf() Hex
	// Get the value commitment
	//
	// If the output is explicit, returns the empty string
	ValueCommitment() Hex
}

// Contains unblinded information such as the asset and the value of a transaction output
type TxOutSecrets struct {
	ffiObject FfiObject
}

// Return the asset identifier of the output.
func (_self *TxOutSecrets) Asset() AssetId {
	_pointer := _self.ffiObject.incrementPointer("*TxOutSecrets")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeAssetIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_txoutsecrets_asset(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the asset blinding factor as a hex string.
//
// Deprecated: use `asset_blinding_factor()` instead.
func (_self *TxOutSecrets) AssetBf() Hex {
	_pointer := _self.ffiObject.incrementPointer("*TxOutSecrets")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeHexINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_txoutsecrets_asset_bf(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the asset commitment
//
// If the output is explicit, returns the empty string
func (_self *TxOutSecrets) AssetCommitment() Hex {
	_pointer := _self.ffiObject.incrementPointer("*TxOutSecrets")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeHexINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_txoutsecrets_asset_commitment(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return true if the output is explicit (no blinding factors).
func (_self *TxOutSecrets) IsExplicit() bool {
	_pointer := _self.ffiObject.incrementPointer("*TxOutSecrets")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_txoutsecrets_is_explicit(
			_pointer, _uniffiStatus)
	}))
}

// Return the value of the output.
func (_self *TxOutSecrets) Value() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*TxOutSecrets")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_txoutsecrets_value(
			_pointer, _uniffiStatus)
	}))
}

// Return the value blinding factor as a hex string.
//
// Deprecated: use `value_blinding_factor()` instead.
func (_self *TxOutSecrets) ValueBf() Hex {
	_pointer := _self.ffiObject.incrementPointer("*TxOutSecrets")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeHexINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_txoutsecrets_value_bf(
				_pointer, _uniffiStatus),
		}
	}))
}

// Get the value commitment
//
// If the output is explicit, returns the empty string
func (_self *TxOutSecrets) ValueCommitment() Hex {
	_pointer := _self.ffiObject.incrementPointer("*TxOutSecrets")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeHexINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_txoutsecrets_value_commitment(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *TxOutSecrets) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterTxOutSecrets struct{}

var FfiConverterTxOutSecretsINSTANCE = FfiConverterTxOutSecrets{}

func (c FfiConverterTxOutSecrets) Lift(pointer unsafe.Pointer) *TxOutSecrets {
	result := &TxOutSecrets{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_txoutsecrets(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_txoutsecrets(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*TxOutSecrets).Destroy)
	return result
}

func (c FfiConverterTxOutSecrets) Read(reader io.Reader) *TxOutSecrets {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterTxOutSecrets) Lower(value *TxOutSecrets) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*TxOutSecrets")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterTxOutSecrets) Write(writer io.Writer, value *TxOutSecrets) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerTxOutSecrets struct{}

func (_ FfiDestroyerTxOutSecrets) Destroy(value *TxOutSecrets) {
	value.Destroy()
}

// A transaction identifier.
type TxidInterface interface {
	// Return the bytes of the transaction identifier.
	Bytes() []byte
}

// A transaction identifier.
type Txid struct {
	ffiObject FfiObject
}

// Construct a Txid object
func NewTxid(hex Hex) (*Txid, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_txid_new(FfiConverterTypeHexINSTANCE.Lower(hex), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Txid
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxidINSTANCE.Lift(_uniffiRV), nil
	}
}

// Return the bytes of the transaction identifier.
func (_self *Txid) Bytes() []byte {
	_pointer := _self.ffiObject.incrementPointer("*Txid")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_txid_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Txid) String() string {
	_pointer := _self.ffiObject.incrementPointer("*Txid")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_txid_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *Txid) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterTxid struct{}

var FfiConverterTxidINSTANCE = FfiConverterTxid{}

func (c FfiConverterTxid) Lift(pointer unsafe.Pointer) *Txid {
	result := &Txid{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_txid(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_txid(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Txid).Destroy)
	return result
}

func (c FfiConverterTxid) Read(reader io.Reader) *Txid {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterTxid) Lower(value *Txid) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Txid")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterTxid) Write(writer io.Writer, value *Txid) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerTxid struct{}

func (_ FfiDestroyerTxid) Destroy(value *Txid) {
	value.Destroy()
}

// LiquiDEX swap proposal
//
// A LiquiDEX swap proposal is a transaction with one input and one output created by the "maker".
// The transaction "swaps" the input for the output, meaning that the "maker" sends the input and
// receives the output.
// However the transaction is incomplete (unbalanced and without a fee output), thus it cannot be
// broadcast.
// The "taker" can "complete" the transaction (using `liquidex_take()`) by
// adding more inputs and more outputs to balance the amounts, meaning that the "taker" sends the
// output and receives the input.
type UnvalidatedLiquidexProposalInterface interface {
	// Validate the proposal output but not the input wich require fetching the previous transaction
	InsecureValidate() (*ValidatedLiquidexProposal, error)
	// Return the transaction id of the previous transaction needed for validation
	NeededTx() (*Txid, error)
	// Validate the proposal input and output, returning a validated proposal.
	Validate(previousTx *Transaction) (*ValidatedLiquidexProposal, error)
}

// LiquiDEX swap proposal
//
// A LiquiDEX swap proposal is a transaction with one input and one output created by the "maker".
// The transaction "swaps" the input for the output, meaning that the "maker" sends the input and
// receives the output.
// However the transaction is incomplete (unbalanced and without a fee output), thus it cannot be
// broadcast.
// The "taker" can "complete" the transaction (using `liquidex_take()`) by
// adding more inputs and more outputs to balance the amounts, meaning that the "taker" sends the
// output and receives the input.
type UnvalidatedLiquidexProposal struct {
	ffiObject FfiObject
}

// Create a LiquiDEX proposal from its json string representation
func NewUnvalidatedLiquidexProposal(s string) (*UnvalidatedLiquidexProposal, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_unvalidatedliquidexproposal_new(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *UnvalidatedLiquidexProposal
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUnvalidatedLiquidexProposalINSTANCE.Lift(_uniffiRV), nil
	}
}

// Create a LiquiDEX proposal from a PSET
func UnvalidatedLiquidexProposalFromPset(pset *Pset) (*UnvalidatedLiquidexProposal, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_unvalidatedliquidexproposal_from_pset(FfiConverterPsetINSTANCE.Lower(pset), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *UnvalidatedLiquidexProposal
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUnvalidatedLiquidexProposalINSTANCE.Lift(_uniffiRV), nil
	}
}

// Validate the proposal output but not the input wich require fetching the previous transaction
func (_self *UnvalidatedLiquidexProposal) InsecureValidate() (*ValidatedLiquidexProposal, error) {
	_pointer := _self.ffiObject.incrementPointer("*UnvalidatedLiquidexProposal")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_unvalidatedliquidexproposal_insecure_validate(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ValidatedLiquidexProposal
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterValidatedLiquidexProposalINSTANCE.Lift(_uniffiRV), nil
	}
}

// Return the transaction id of the previous transaction needed for validation
func (_self *UnvalidatedLiquidexProposal) NeededTx() (*Txid, error) {
	_pointer := _self.ffiObject.incrementPointer("*UnvalidatedLiquidexProposal")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_unvalidatedliquidexproposal_needed_tx(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Txid
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTxidINSTANCE.Lift(_uniffiRV), nil
	}
}

// Validate the proposal input and output, returning a validated proposal.
func (_self *UnvalidatedLiquidexProposal) Validate(previousTx *Transaction) (*ValidatedLiquidexProposal, error) {
	_pointer := _self.ffiObject.incrementPointer("*UnvalidatedLiquidexProposal")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_unvalidatedliquidexproposal_validate(
			_pointer, FfiConverterTransactionINSTANCE.Lower(previousTx), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ValidatedLiquidexProposal
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterValidatedLiquidexProposalINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *UnvalidatedLiquidexProposal) String() string {
	_pointer := _self.ffiObject.incrementPointer("*UnvalidatedLiquidexProposal")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_unvalidatedliquidexproposal_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *UnvalidatedLiquidexProposal) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterUnvalidatedLiquidexProposal struct{}

var FfiConverterUnvalidatedLiquidexProposalINSTANCE = FfiConverterUnvalidatedLiquidexProposal{}

func (c FfiConverterUnvalidatedLiquidexProposal) Lift(pointer unsafe.Pointer) *UnvalidatedLiquidexProposal {
	result := &UnvalidatedLiquidexProposal{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_unvalidatedliquidexproposal(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_unvalidatedliquidexproposal(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*UnvalidatedLiquidexProposal).Destroy)
	return result
}

func (c FfiConverterUnvalidatedLiquidexProposal) Read(reader io.Reader) *UnvalidatedLiquidexProposal {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterUnvalidatedLiquidexProposal) Lower(value *UnvalidatedLiquidexProposal) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*UnvalidatedLiquidexProposal")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterUnvalidatedLiquidexProposal) Write(writer io.Writer, value *UnvalidatedLiquidexProposal) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerUnvalidatedLiquidexProposal struct{}

func (_ FfiDestroyerUnvalidatedLiquidexProposal) Destroy(value *UnvalidatedLiquidexProposal) {
	value.Destroy()
}

// Wrapper over [`lwk_wollet::Update`]
type UpdateInterface interface {
	// Whether the update only changes the tip (does not affect transactions)
	OnlyTip() bool
	// Serialize an `Update` to a byte array, can be deserialized back with `new()`
	Serialize() ([]byte, error)
}

// Wrapper over [`lwk_wollet::Update`]
type Update struct {
	ffiObject FfiObject
}

// Creates an `Update` from a byte array created with `serialize()`
func NewUpdate(bytes []byte) (*Update, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_update_new(FfiConverterBytesINSTANCE.Lower(bytes), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Update
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUpdateINSTANCE.Lift(_uniffiRV), nil
	}
}

// Whether the update only changes the tip (does not affect transactions)
func (_self *Update) OnlyTip() bool {
	_pointer := _self.ffiObject.incrementPointer("*Update")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_update_only_tip(
			_pointer, _uniffiStatus)
	}))
}

// Serialize an `Update` to a byte array, can be deserialized back with `new()`
func (_self *Update) Serialize() ([]byte, error) {
	_pointer := _self.ffiObject.incrementPointer("*Update")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_update_serialize(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *Update) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterUpdate struct{}

var FfiConverterUpdateINSTANCE = FfiConverterUpdate{}

func (c FfiConverterUpdate) Lift(pointer unsafe.Pointer) *Update {
	result := &Update{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_update(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_update(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Update).Destroy)
	return result
}

func (c FfiConverterUpdate) Read(reader io.Reader) *Update {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterUpdate) Lower(value *Update) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Update")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterUpdate) Write(writer io.Writer, value *Update) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerUpdate struct{}

func (_ FfiDestroyerUpdate) Destroy(value *Update) {
	value.Destroy()
}

// Created by validating `UnvalidatedLiquidexProposal` via `validate()` or `insecure_validate()`
type ValidatedLiquidexProposalInterface interface {
	// The asset value and amount in the input of this validated proposal.
	Input() *AssetAmount
	// The asset value and amount in the output of this validated proposal.
	Output() *AssetAmount
}

// Created by validating `UnvalidatedLiquidexProposal` via `validate()` or `insecure_validate()`
type ValidatedLiquidexProposal struct {
	ffiObject FfiObject
}

// The asset value and amount in the input of this validated proposal.
func (_self *ValidatedLiquidexProposal) Input() *AssetAmount {
	_pointer := _self.ffiObject.incrementPointer("*ValidatedLiquidexProposal")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterAssetAmountINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_validatedliquidexproposal_input(
			_pointer, _uniffiStatus)
	}))
}

// The asset value and amount in the output of this validated proposal.
func (_self *ValidatedLiquidexProposal) Output() *AssetAmount {
	_pointer := _self.ffiObject.incrementPointer("*ValidatedLiquidexProposal")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterAssetAmountINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_validatedliquidexproposal_output(
			_pointer, _uniffiStatus)
	}))
}

func (_self *ValidatedLiquidexProposal) String() string {
	_pointer := _self.ffiObject.incrementPointer("*ValidatedLiquidexProposal")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_validatedliquidexproposal_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *ValidatedLiquidexProposal) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterValidatedLiquidexProposal struct{}

var FfiConverterValidatedLiquidexProposalINSTANCE = FfiConverterValidatedLiquidexProposal{}

func (c FfiConverterValidatedLiquidexProposal) Lift(pointer unsafe.Pointer) *ValidatedLiquidexProposal {
	result := &ValidatedLiquidexProposal{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_validatedliquidexproposal(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_validatedliquidexproposal(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ValidatedLiquidexProposal).Destroy)
	return result
}

func (c FfiConverterValidatedLiquidexProposal) Read(reader io.Reader) *ValidatedLiquidexProposal {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterValidatedLiquidexProposal) Lower(value *ValidatedLiquidexProposal) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*ValidatedLiquidexProposal")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterValidatedLiquidexProposal) Write(writer io.Writer, value *ValidatedLiquidexProposal) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerValidatedLiquidexProposal struct{}

func (_ FfiDestroyerValidatedLiquidexProposal) Destroy(value *ValidatedLiquidexProposal) {
	value.Destroy()
}

// A blinding factor for value commitments.
type ValueBlindingFactorInterface interface {
	// Returns the bytes (32 bytes).
	ToBytes() []byte
}

// A blinding factor for value commitments.
type ValueBlindingFactor struct {
	ffiObject FfiObject
}

// Create from bytes.
func ValueBlindingFactorFromBytes(bytes []byte) (*ValueBlindingFactor, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_valueblindingfactor_from_bytes(FfiConverterBytesINSTANCE.Lower(bytes), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ValueBlindingFactor
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterValueBlindingFactorINSTANCE.Lift(_uniffiRV), nil
	}
}

// Creates from a hex string.
func ValueBlindingFactorFromString(s string) (*ValueBlindingFactor, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_valueblindingfactor_from_string(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ValueBlindingFactor
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterValueBlindingFactorINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get a unblinded/zero value blinding factor
func ValueBlindingFactorZero() *ValueBlindingFactor {
	return FfiConverterValueBlindingFactorINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_valueblindingfactor_zero(_uniffiStatus)
	}))
}

// Returns the bytes (32 bytes).
func (_self *ValueBlindingFactor) ToBytes() []byte {
	_pointer := _self.ffiObject.incrementPointer("*ValueBlindingFactor")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_valueblindingfactor_to_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *ValueBlindingFactor) String() string {
	_pointer := _self.ffiObject.incrementPointer("*ValueBlindingFactor")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_valueblindingfactor_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *ValueBlindingFactor) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterValueBlindingFactor struct{}

var FfiConverterValueBlindingFactorINSTANCE = FfiConverterValueBlindingFactor{}

func (c FfiConverterValueBlindingFactor) Lift(pointer unsafe.Pointer) *ValueBlindingFactor {
	result := &ValueBlindingFactor{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_valueblindingfactor(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_valueblindingfactor(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ValueBlindingFactor).Destroy)
	return result
}

func (c FfiConverterValueBlindingFactor) Read(reader io.Reader) *ValueBlindingFactor {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterValueBlindingFactor) Lower(value *ValueBlindingFactor) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*ValueBlindingFactor")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterValueBlindingFactor) Write(writer io.Writer, value *ValueBlindingFactor) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerValueBlindingFactor struct{}

func (_ FfiDestroyerValueBlindingFactor) Destroy(value *ValueBlindingFactor) {
	value.Destroy()
}

// Value returned by asking transactions to the wallet. Contains details about a transaction
// from the perspective of the wallet, for example the net-balance of the transaction for the
// wallet.
type WalletTxInterface interface {
	// Return the net balance of the transaction for the wallet.
	Balance() map[AssetId]int64
	// Return the fee of the transaction.
	Fee() uint64
	// Return the height of the block containing the transaction if it's confirmed.
	Height() *uint32
	// Return a list with the same number of elements as the inputs of the transaction.
	// The element in the list is a `WalletTxOut` (the output spent to create the input)
	// if it belongs to the wallet, while it is None for inputs owned by others
	Inputs() []**WalletTxOut
	// Return a list with the same number of elements as the outputs of the transaction.
	// The element in the list is a `WalletTxOut` if it belongs to the wallet,
	// while it is None for inputs owned by others
	Outputs() []**WalletTxOut
	// Return the timestamp of the block containing the transaction if it's confirmed.
	Timestamp() *uint32
	// Return a copy of the transaction.
	Tx() *Transaction
	// Return the transaction identifier.
	Txid() *Txid
	// Return the type of the transaction. Can be "issuance", "reissuance", "burn", "redeposit",
	// "incoming", "outgoing" or "unknown".
	Type() string
	// Return the URL to view the transaction on the explorer. Including the information needed to
	// unblind the transaction in the explorer UI.
	UnblindedUrl(explorerUrl string) string
}

// Value returned by asking transactions to the wallet. Contains details about a transaction
// from the perspective of the wallet, for example the net-balance of the transaction for the
// wallet.
type WalletTx struct {
	ffiObject FfiObject
}

// Return the net balance of the transaction for the wallet.
func (_self *WalletTx) Balance() map[AssetId]int64 {
	_pointer := _self.ffiObject.incrementPointer("*WalletTx")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterMapTypeAssetIdInt64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wallettx_balance(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the fee of the transaction.
func (_self *WalletTx) Fee() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*WalletTx")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_wallettx_fee(
			_pointer, _uniffiStatus)
	}))
}

// Return the height of the block containing the transaction if it's confirmed.
func (_self *WalletTx) Height() *uint32 {
	_pointer := _self.ffiObject.incrementPointer("*WalletTx")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wallettx_height(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return a list with the same number of elements as the inputs of the transaction.
// The element in the list is a `WalletTxOut` (the output spent to create the input)
// if it belongs to the wallet, while it is None for inputs owned by others
func (_self *WalletTx) Inputs() []**WalletTxOut {
	_pointer := _self.ffiObject.incrementPointer("*WalletTx")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceOptionalWalletTxOutINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wallettx_inputs(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return a list with the same number of elements as the outputs of the transaction.
// The element in the list is a `WalletTxOut` if it belongs to the wallet,
// while it is None for inputs owned by others
func (_self *WalletTx) Outputs() []**WalletTxOut {
	_pointer := _self.ffiObject.incrementPointer("*WalletTx")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceOptionalWalletTxOutINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wallettx_outputs(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the timestamp of the block containing the transaction if it's confirmed.
func (_self *WalletTx) Timestamp() *uint32 {
	_pointer := _self.ffiObject.incrementPointer("*WalletTx")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wallettx_timestamp(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return a copy of the transaction.
func (_self *WalletTx) Tx() *Transaction {
	_pointer := _self.ffiObject.incrementPointer("*WalletTx")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTransactionINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wallettx_tx(
			_pointer, _uniffiStatus)
	}))
}

// Return the transaction identifier.
func (_self *WalletTx) Txid() *Txid {
	_pointer := _self.ffiObject.incrementPointer("*WalletTx")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTxidINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wallettx_txid(
			_pointer, _uniffiStatus)
	}))
}

// Return the type of the transaction. Can be "issuance", "reissuance", "burn", "redeposit",
// "incoming", "outgoing" or "unknown".
func (_self *WalletTx) Type() string {
	_pointer := _self.ffiObject.incrementPointer("*WalletTx")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wallettx_type_(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the URL to view the transaction on the explorer. Including the information needed to
// unblind the transaction in the explorer UI.
func (_self *WalletTx) UnblindedUrl(explorerUrl string) string {
	_pointer := _self.ffiObject.incrementPointer("*WalletTx")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wallettx_unblinded_url(
				_pointer, FfiConverterStringINSTANCE.Lower(explorerUrl), _uniffiStatus),
		}
	}))
}
func (object *WalletTx) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterWalletTx struct{}

var FfiConverterWalletTxINSTANCE = FfiConverterWalletTx{}

func (c FfiConverterWalletTx) Lift(pointer unsafe.Pointer) *WalletTx {
	result := &WalletTx{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_wallettx(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_wallettx(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*WalletTx).Destroy)
	return result
}

func (c FfiConverterWalletTx) Read(reader io.Reader) *WalletTx {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterWalletTx) Lower(value *WalletTx) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*WalletTx")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterWalletTx) Write(writer io.Writer, value *WalletTx) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerWalletTx struct{}

func (_ FfiDestroyerWalletTx) Destroy(value *WalletTx) {
	value.Destroy()
}

// Details of a wallet transaction output used in `WalletTx`
type WalletTxOutInterface interface {
	// Return the address of this `WalletTxOut`.
	Address() *Address
	// Return the chain of this `WalletTxOut`. Can be "Chain::External" or "Chain::Internal" (change).
	ExtInt() Chain
	// Return the height of the block containing this output if it's confirmed.
	Height() *uint32
	// Return the outpoint (txid and vout) of this `WalletTxOut`.
	Outpoint() *OutPoint
	// Return the script pubkey of the address of this `WalletTxOut`.
	ScriptPubkey() *Script
	// Return the unblinded values of this `WalletTxOut`.
	Unblinded() *TxOutSecrets
	// Return the wildcard index used to derive the address of this `WalletTxOut`.
	WildcardIndex() uint32
}

// Details of a wallet transaction output used in `WalletTx`
type WalletTxOut struct {
	ffiObject FfiObject
}

// Return the address of this `WalletTxOut`.
func (_self *WalletTxOut) Address() *Address {
	_pointer := _self.ffiObject.incrementPointer("*WalletTxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterAddressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wallettxout_address(
			_pointer, _uniffiStatus)
	}))
}

// Return the chain of this `WalletTxOut`. Can be "Chain::External" or "Chain::Internal" (change).
func (_self *WalletTxOut) ExtInt() Chain {
	_pointer := _self.ffiObject.incrementPointer("*WalletTxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterChainINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wallettxout_ext_int(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the height of the block containing this output if it's confirmed.
func (_self *WalletTxOut) Height() *uint32 {
	_pointer := _self.ffiObject.incrementPointer("*WalletTxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wallettxout_height(
				_pointer, _uniffiStatus),
		}
	}))
}

// Return the outpoint (txid and vout) of this `WalletTxOut`.
func (_self *WalletTxOut) Outpoint() *OutPoint {
	_pointer := _self.ffiObject.incrementPointer("*WalletTxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOutPointINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wallettxout_outpoint(
			_pointer, _uniffiStatus)
	}))
}

// Return the script pubkey of the address of this `WalletTxOut`.
func (_self *WalletTxOut) ScriptPubkey() *Script {
	_pointer := _self.ffiObject.incrementPointer("*WalletTxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterScriptINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wallettxout_script_pubkey(
			_pointer, _uniffiStatus)
	}))
}

// Return the unblinded values of this `WalletTxOut`.
func (_self *WalletTxOut) Unblinded() *TxOutSecrets {
	_pointer := _self.ffiObject.incrementPointer("*WalletTxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTxOutSecretsINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wallettxout_unblinded(
			_pointer, _uniffiStatus)
	}))
}

// Return the wildcard index used to derive the address of this `WalletTxOut`.
func (_self *WalletTxOut) WildcardIndex() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*WalletTxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_wallettxout_wildcard_index(
			_pointer, _uniffiStatus)
	}))
}
func (object *WalletTxOut) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterWalletTxOut struct{}

var FfiConverterWalletTxOutINSTANCE = FfiConverterWalletTxOut{}

func (c FfiConverterWalletTxOut) Lift(pointer unsafe.Pointer) *WalletTxOut {
	result := &WalletTxOut{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_wallettxout(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_wallettxout(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*WalletTxOut).Destroy)
	return result
}

func (c FfiConverterWalletTxOut) Read(reader io.Reader) *WalletTxOut {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterWalletTxOut) Lower(value *WalletTxOut) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*WalletTxOut")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterWalletTxOut) Write(writer io.Writer, value *WalletTxOut) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerWalletTxOut struct{}

func (_ FfiDestroyerWalletTxOut) Destroy(value *WalletTxOut) {
	value.Destroy()
}

type WebHookInterface interface {
}
type WebHook struct {
	ffiObject FfiObject
}

func NewWebHook(url string, status []string) *WebHook {
	return FfiConverterWebHookINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_webhook_new(FfiConverterStringINSTANCE.Lower(url), FfiConverterSequenceStringINSTANCE.Lower(status), _uniffiStatus)
	}))
}

func (object *WebHook) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterWebHook struct{}

var FfiConverterWebHookINSTANCE = FfiConverterWebHook{}

func (c FfiConverterWebHook) Lift(pointer unsafe.Pointer) *WebHook {
	result := &WebHook{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_webhook(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_webhook(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*WebHook).Destroy)
	return result
}

func (c FfiConverterWebHook) Read(reader io.Reader) *WebHook {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterWebHook) Lower(value *WebHook) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*WebHook")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterWebHook) Write(writer io.Writer, value *WebHook) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerWebHook struct{}

func (_ FfiDestroyerWebHook) Destroy(value *WebHook) {
	value.Destroy()
}

// A Watch-Only wallet, wrapper over [`lwk_wollet::Wollet`]
type WolletInterface interface {
	// Add wallet details to the PSET
	AddDetails(pset *Pset) (*Pset, error)
	// Get a wallet address
	//
	// If Some return the address at the given index,
	// otherwise the last unused address.
	Address(index *uint32) (*AddressResult, error)
	// Apply a transaction to the wallet state
	//
	// Wallet transactions are normally obtained using `full_scan()`
	// and applying the resulting `Update` with `apply_update()`. However a
	// full scan involves network calls and it can take a significant amount of time.
	//
	// If the caller does not want to wait for a full scan containing the transaction, it can
	// apply the transaction to the wallet state using this function.
	//
	// Note: if this transaction is *not* returned by a next full scan, after `apply_update()` it will disappear from the
	// transactions list, will not be included in balance computations, and by the remaining
	// wollet methods.
	//
	// Calling this method, might cause `apply_update()` to fail with a
	// `Error::UpdateOnDifferentStatus`, make sure to either avoid it or handle the error properly.
	ApplyTransaction(tx *Transaction) error
	// Apply an update containing blockchain data
	//
	// To update the wallet you need to first obtain the blockchain data relevant for the wallet.
	// This can be done using `full_scan()`, which
	// returns an `Update` that contains new transaction and other data relevant for the
	// wallet.
	// The update must then be applied to the `Wollet` so that wollet methods such as
	// `balance()` or `transactions()` include the new data.
	//
	// However getting blockchain data involves network calls, so between the full scan start and
	// when the update is applied it might elapse a significant amount of time.
	// In that interval, applying any update, or any transaction using `apply_transaction()`,
	// will cause this function to return a `Error::UpdateOnDifferentStatus`.
	// Callers should either avoid applying updates and transactions, or they can catch the error and wait for a new full scan to be completed and applied.
	ApplyUpdate(update *Update) error
	// Get the wallet balance
	Balance() (map[AssetId]uint64, error)
	// Get a copy of the wallet descriptor
	Descriptor() (*WolletDescriptor, error)
	// Return the [ELIP152](https://github.com/ElementsProject/ELIPs/blob/main/elip-0152.mediawiki) deterministic wallet identifier.
	Dwid() (string, error)
	// Extract the wallet UTXOs that a PSET is creating
	ExtractWalletUtxos(pset *Pset) ([]*ExternalUtxo, error)
	// Finalize a PSET, returning a new PSET with the finalized inputs
	Finalize(pset *Pset) (*Pset, error)
	// Whether the wallet is AMP0
	IsAmp0() (bool, error)
	// Whether the wallet is segwit
	IsSegwit() (bool, error)
	// Max weight to satisfy for inputs belonging to this wallet
	MaxWeightToSatisfy() (uint32, error)
	// Get the PSET details with respect to the wallet
	PsetDetails(pset *Pset) (*PsetDetails, error)
	// Get all the wallet transaction
	Transaction(txid *Txid) (*WalletTx, error)
	// Get all the wallet transactions
	Transactions() ([]*WalletTx, error)
	// Get the wallet transactions with pagination
	TransactionsPaginated(offset uint32, limit uint32) ([]*WalletTx, error)
	// Get all the transaction outputs of the wallet, both spent and unspent
	Txos() ([]*WalletTxOut, error)
	// Get the utxo with unspent transaction outputs of the wallet
	// Return utxos unblinded with a specific blinding key
	UnblindUtxosWith(blindingPrivkey *SecretKey) ([]*ExternalUtxo, error)
	// Get the unspent transaction outputs of the wallet
	Utxos() ([]*WalletTxOut, error)
	// Note this a test method but we are not feature gating in test because we need it in
	// destination language examples
	WaitForTx(txid *Txid, client *ElectrumClient) (*WalletTx, error)
}

// A Watch-Only wallet, wrapper over [`lwk_wollet::Wollet`]
type Wollet struct {
	ffiObject FfiObject
}

// Construct a Watch-Only wallet object
func NewWollet(network *Network, descriptor *WolletDescriptor, datadir *string) (*Wollet, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_wollet_new(FfiConverterNetworkINSTANCE.Lower(network), FfiConverterWolletDescriptorINSTANCE.Lower(descriptor), FfiConverterOptionalStringINSTANCE.Lower(datadir), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Wollet
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWolletINSTANCE.Lift(_uniffiRV), nil
	}
}

// Construct a Watch-Only wallet object with a caller provided store
func WolletWithCustomStore(network *Network, descriptor *WolletDescriptor, store *ForeignStoreLink) (*Wollet, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_wollet_with_custom_store(FfiConverterNetworkINSTANCE.Lower(network), FfiConverterWolletDescriptorINSTANCE.Lower(descriptor), FfiConverterForeignStoreLinkINSTANCE.Lower(store), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Wollet
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWolletINSTANCE.Lift(_uniffiRV), nil
	}
}

// Add wallet details to the PSET
func (_self *Wollet) AddDetails(pset *Pset) (*Pset, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wollet_add_details(
			_pointer, FfiConverterPsetINSTANCE.Lower(pset), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Pset
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPsetINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get a wallet address
//
// If Some return the address at the given index,
// otherwise the last unused address.
func (_self *Wollet) Address(index *uint32) (*AddressResult, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wollet_address(
			_pointer, FfiConverterOptionalUint32INSTANCE.Lower(index), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *AddressResult
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAddressResultINSTANCE.Lift(_uniffiRV), nil
	}
}

// Apply a transaction to the wallet state
//
// Wallet transactions are normally obtained using `full_scan()`
// and applying the resulting `Update` with `apply_update()`. However a
// full scan involves network calls and it can take a significant amount of time.
//
// If the caller does not want to wait for a full scan containing the transaction, it can
// apply the transaction to the wallet state using this function.
//
// Note: if this transaction is *not* returned by a next full scan, after `apply_update()` it will disappear from the
// transactions list, will not be included in balance computations, and by the remaining
// wollet methods.
//
// Calling this method, might cause `apply_update()` to fail with a
// `Error::UpdateOnDifferentStatus`, make sure to either avoid it or handle the error properly.
func (_self *Wollet) ApplyTransaction(tx *Transaction) error {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_wollet_apply_transaction(
			_pointer, FfiConverterTransactionINSTANCE.Lower(tx), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Apply an update containing blockchain data
//
// To update the wallet you need to first obtain the blockchain data relevant for the wallet.
// This can be done using `full_scan()`, which
// returns an `Update` that contains new transaction and other data relevant for the
// wallet.
// The update must then be applied to the `Wollet` so that wollet methods such as
// `balance()` or `transactions()` include the new data.
//
// However getting blockchain data involves network calls, so between the full scan start and
// when the update is applied it might elapse a significant amount of time.
// In that interval, applying any update, or any transaction using `apply_transaction()`,
// will cause this function to return a `Error::UpdateOnDifferentStatus`.
// Callers should either avoid applying updates and transactions, or they can catch the error and wait for a new full scan to be completed and applied.
func (_self *Wollet) ApplyUpdate(update *Update) error {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_wollet_apply_update(
			_pointer, FfiConverterUpdateINSTANCE.Lower(update), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Get the wallet balance
func (_self *Wollet) Balance() (map[AssetId]uint64, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wollet_balance(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue map[AssetId]uint64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterMapTypeAssetIdUint64INSTANCE.Lift(_uniffiRV), nil
	}
}

// Get a copy of the wallet descriptor
func (_self *Wollet) Descriptor() (*WolletDescriptor, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wollet_descriptor(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *WolletDescriptor
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWolletDescriptorINSTANCE.Lift(_uniffiRV), nil
	}
}

// Return the [ELIP152](https://github.com/ElementsProject/ELIPs/blob/main/elip-0152.mediawiki) deterministic wallet identifier.
func (_self *Wollet) Dwid() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wollet_dwid(
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

// Extract the wallet UTXOs that a PSET is creating
func (_self *Wollet) ExtractWalletUtxos(pset *Pset) ([]*ExternalUtxo, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wollet_extract_wallet_utxos(
				_pointer, FfiConverterPsetINSTANCE.Lower(pset), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []*ExternalUtxo
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceExternalUtxoINSTANCE.Lift(_uniffiRV), nil
	}
}

// Finalize a PSET, returning a new PSET with the finalized inputs
func (_self *Wollet) Finalize(pset *Pset) (*Pset, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wollet_finalize(
			_pointer, FfiConverterPsetINSTANCE.Lower(pset), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Pset
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPsetINSTANCE.Lift(_uniffiRV), nil
	}
}

// Whether the wallet is AMP0
func (_self *Wollet) IsAmp0() (bool, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_wollet_is_amp0(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue bool
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBoolINSTANCE.Lift(_uniffiRV), nil
	}
}

// Whether the wallet is segwit
func (_self *Wollet) IsSegwit() (bool, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_wollet_is_segwit(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue bool
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBoolINSTANCE.Lift(_uniffiRV), nil
	}
}

// Max weight to satisfy for inputs belonging to this wallet
func (_self *Wollet) MaxWeightToSatisfy() (uint32, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_wollet_max_weight_to_satisfy(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue uint32
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUint32INSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the PSET details with respect to the wallet
func (_self *Wollet) PsetDetails(pset *Pset) (*PsetDetails, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wollet_pset_details(
			_pointer, FfiConverterPsetINSTANCE.Lower(pset), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *PsetDetails
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPsetDetailsINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get all the wallet transaction
func (_self *Wollet) Transaction(txid *Txid) (*WalletTx, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wollet_transaction(
			_pointer, FfiConverterTxidINSTANCE.Lower(txid), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *WalletTx
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWalletTxINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get all the wallet transactions
func (_self *Wollet) Transactions() ([]*WalletTx, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wollet_transactions(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []*WalletTx
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceWalletTxINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the wallet transactions with pagination
func (_self *Wollet) TransactionsPaginated(offset uint32, limit uint32) ([]*WalletTx, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wollet_transactions_paginated(
				_pointer, FfiConverterUint32INSTANCE.Lower(offset), FfiConverterUint32INSTANCE.Lower(limit), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []*WalletTx
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceWalletTxINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get all the transaction outputs of the wallet, both spent and unspent
func (_self *Wollet) Txos() ([]*WalletTxOut, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wollet_txos(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []*WalletTxOut
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceWalletTxOutINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the utxo with unspent transaction outputs of the wallet
// Return utxos unblinded with a specific blinding key
func (_self *Wollet) UnblindUtxosWith(blindingPrivkey *SecretKey) ([]*ExternalUtxo, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wollet_unblind_utxos_with(
				_pointer, FfiConverterSecretKeyINSTANCE.Lower(blindingPrivkey), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []*ExternalUtxo
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceExternalUtxoINSTANCE.Lift(_uniffiRV), nil
	}
}

// Get the unspent transaction outputs of the wallet
func (_self *Wollet) Utxos() ([]*WalletTxOut, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wollet_utxos(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []*WalletTxOut
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceWalletTxOutINSTANCE.Lift(_uniffiRV), nil
	}
}

// Note this a test method but we are not feature gating in test because we need it in
// destination language examples
func (_self *Wollet) WaitForTx(txid *Txid, client *ElectrumClient) (*WalletTx, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wollet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wollet_wait_for_tx(
			_pointer, FfiConverterTxidINSTANCE.Lower(txid), FfiConverterElectrumClientINSTANCE.Lower(client), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *WalletTx
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWalletTxINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *Wollet) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterWollet struct{}

var FfiConverterWolletINSTANCE = FfiConverterWollet{}

func (c FfiConverterWollet) Lift(pointer unsafe.Pointer) *Wollet {
	result := &Wollet{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_wollet(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_wollet(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Wollet).Destroy)
	return result
}

func (c FfiConverterWollet) Read(reader io.Reader) *Wollet {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterWollet) Lower(value *Wollet) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Wollet")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterWollet) Write(writer io.Writer, value *Wollet) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerWollet struct{}

func (_ FfiDestroyerWollet) Destroy(value *Wollet) {
	value.Destroy()
}

// A builder for constructing a [`Wollet`].
type WolletBuilderInterface interface {
	// Build the wallet from this builder.
	Build() (*Wollet, error)
	// Persist wallet updates in the legacy encrypted filesystem store.
	WithLegacyFsStore(datadir string) error
	// Set the threshold used to merge persisted updates during build.
	//
	// `None` disables merging (default behavior).
	WithMergeThreshold(mergeThreshold *uint32) error
	// Use a caller-provided store implementation.
	WithStore(store *ForeignStoreLink) error
}

// A builder for constructing a [`Wollet`].
type WolletBuilder struct {
	ffiObject FfiObject
}

// Create a builder for a watch-only wallet.
func NewWolletBuilder(network *Network, descriptor *WolletDescriptor) *WolletBuilder {
	return FfiConverterWolletBuilderINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_wolletbuilder_new(FfiConverterNetworkINSTANCE.Lower(network), FfiConverterWolletDescriptorINSTANCE.Lower(descriptor), _uniffiStatus)
	}))
}

// Build the wallet from this builder.
func (_self *WolletBuilder) Build() (*Wollet, error) {
	_pointer := _self.ffiObject.incrementPointer("*WolletBuilder")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wolletbuilder_build(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Wollet
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWolletINSTANCE.Lift(_uniffiRV), nil
	}
}

// Persist wallet updates in the legacy encrypted filesystem store.
func (_self *WolletBuilder) WithLegacyFsStore(datadir string) error {
	_pointer := _self.ffiObject.incrementPointer("*WolletBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_wolletbuilder_with_legacy_fs_store(
			_pointer, FfiConverterStringINSTANCE.Lower(datadir), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Set the threshold used to merge persisted updates during build.
//
// `None` disables merging (default behavior).
func (_self *WolletBuilder) WithMergeThreshold(mergeThreshold *uint32) error {
	_pointer := _self.ffiObject.incrementPointer("*WolletBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_wolletbuilder_with_merge_threshold(
			_pointer, FfiConverterOptionalUint32INSTANCE.Lower(mergeThreshold), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Use a caller-provided store implementation.
func (_self *WolletBuilder) WithStore(store *ForeignStoreLink) error {
	_pointer := _self.ffiObject.incrementPointer("*WolletBuilder")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_wolletbuilder_with_store(
			_pointer, FfiConverterForeignStoreLinkINSTANCE.Lower(store), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}
func (object *WolletBuilder) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterWolletBuilder struct{}

var FfiConverterWolletBuilderINSTANCE = FfiConverterWolletBuilder{}

func (c FfiConverterWolletBuilder) Lift(pointer unsafe.Pointer) *WolletBuilder {
	result := &WolletBuilder{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_wolletbuilder(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_wolletbuilder(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*WolletBuilder).Destroy)
	return result
}

func (c FfiConverterWolletBuilder) Read(reader io.Reader) *WolletBuilder {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterWolletBuilder) Lower(value *WolletBuilder) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*WolletBuilder")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterWolletBuilder) Write(writer io.Writer, value *WolletBuilder) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerWolletBuilder struct{}

func (_ FfiDestroyerWolletBuilder) Destroy(value *WolletBuilder) {
	value.Destroy()
}

// The output descriptors, wrapper over [`lwk_wollet::WolletDescriptor`]
type WolletDescriptorInterface interface {
	// Derive the private blinding key
	DeriveBlindingKey(scriptPubkey *Script) **SecretKey
	// Whether the descriptor is AMP0
	IsAmp0() bool
	// Whether the descriptor is on the mainnet
	IsMainnet() bool
	// Derive a scriptpubkey
	ScriptPubkey(extInt Chain, index uint32) (*Script, error)
	// Return the descriptor encoded so that can be part of an URL
	UrlEncodedDescriptor() (string, error)
}

// The output descriptors, wrapper over [`lwk_wollet::WolletDescriptor`]
type WolletDescriptor struct {
	ffiObject FfiObject
}

// Create a new descriptor from its string representation.
func NewWolletDescriptor(descriptor string) (*WolletDescriptor, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_wolletdescriptor_new(FfiConverterStringINSTANCE.Lower(descriptor), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *WolletDescriptor
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWolletDescriptorINSTANCE.Lift(_uniffiRV), nil
	}
}

// Derive the private blinding key
func (_self *WolletDescriptor) DeriveBlindingKey(scriptPubkey *Script) **SecretKey {
	_pointer := _self.ffiObject.incrementPointer("*WolletDescriptor")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalSecretKeyINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wolletdescriptor_derive_blinding_key(
				_pointer, FfiConverterScriptINSTANCE.Lower(scriptPubkey), _uniffiStatus),
		}
	}))
}

// Whether the descriptor is AMP0
func (_self *WolletDescriptor) IsAmp0() bool {
	_pointer := _self.ffiObject.incrementPointer("*WolletDescriptor")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_wolletdescriptor_is_amp0(
			_pointer, _uniffiStatus)
	}))
}

// Whether the descriptor is on the mainnet
func (_self *WolletDescriptor) IsMainnet() bool {
	_pointer := _self.ffiObject.incrementPointer("*WolletDescriptor")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_wolletdescriptor_is_mainnet(
			_pointer, _uniffiStatus)
	}))
}

// Derive a scriptpubkey
func (_self *WolletDescriptor) ScriptPubkey(extInt Chain, index uint32) (*Script, error) {
	_pointer := _self.ffiObject.incrementPointer("*WolletDescriptor")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wolletdescriptor_script_pubkey(
			_pointer, FfiConverterChainINSTANCE.Lower(extInt), FfiConverterUint32INSTANCE.Lower(index), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Script
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterScriptINSTANCE.Lift(_uniffiRV), nil
	}
}

// Return the descriptor encoded so that can be part of an URL
func (_self *WolletDescriptor) UrlEncodedDescriptor() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*WolletDescriptor")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wolletdescriptor_url_encoded_descriptor(
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

func (_self *WolletDescriptor) String() string {
	_pointer := _self.ffiObject.incrementPointer("*WolletDescriptor")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_wolletdescriptor_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *WolletDescriptor) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterWolletDescriptor struct{}

var FfiConverterWolletDescriptorINSTANCE = FfiConverterWolletDescriptor{}

func (c FfiConverterWolletDescriptor) Lift(pointer unsafe.Pointer) *WolletDescriptor {
	result := &WolletDescriptor{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_wolletdescriptor(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_wolletdescriptor(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*WolletDescriptor).Destroy)
	return result
}

func (c FfiConverterWolletDescriptor) Read(reader io.Reader) *WolletDescriptor {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterWolletDescriptor) Lower(value *WolletDescriptor) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*WolletDescriptor")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterWolletDescriptor) Write(writer io.Writer, value *WolletDescriptor) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerWolletDescriptor struct{}

func (_ FfiDestroyerWolletDescriptor) Destroy(value *WolletDescriptor) {
	value.Destroy()
}

// A builder for the `BoltzSession`
type BoltzSessionBuilder struct {
	Network                  *Network
	Client                   *AnyClient
	Timeout                  *uint64
	Mnemonic                 **Mnemonic
	Logging                  *Logging
	Polling                  bool
	TimeoutAdvance           *uint64
	NextIndexToUse           *uint32
	ReferralId               *string
	BitcoinElectrumClientUrl *string
	RandomPreimages          bool
	// Optional store for persisting swap data
	//
	// When set, swap data will be automatically persisted to the store after creation
	// and on each state change. This enables automatic restoration of pending swaps.
	Store **ForeignStoreLink
}

func (r *BoltzSessionBuilder) Destroy() {
	FfiDestroyerNetwork{}.Destroy(r.Network)
	FfiDestroyerAnyClient{}.Destroy(r.Client)
	FfiDestroyerOptionalUint64{}.Destroy(r.Timeout)
	FfiDestroyerOptionalMnemonic{}.Destroy(r.Mnemonic)
	FfiDestroyerOptionalLogging{}.Destroy(r.Logging)
	FfiDestroyerBool{}.Destroy(r.Polling)
	FfiDestroyerOptionalUint64{}.Destroy(r.TimeoutAdvance)
	FfiDestroyerOptionalUint32{}.Destroy(r.NextIndexToUse)
	FfiDestroyerOptionalString{}.Destroy(r.ReferralId)
	FfiDestroyerOptionalString{}.Destroy(r.BitcoinElectrumClientUrl)
	FfiDestroyerBool{}.Destroy(r.RandomPreimages)
	FfiDestroyerOptionalForeignStoreLink{}.Destroy(r.Store)
}

type FfiConverterBoltzSessionBuilder struct{}

var FfiConverterBoltzSessionBuilderINSTANCE = FfiConverterBoltzSessionBuilder{}

func (c FfiConverterBoltzSessionBuilder) Lift(rb RustBufferI) BoltzSessionBuilder {
	return LiftFromRustBuffer[BoltzSessionBuilder](c, rb)
}

func (c FfiConverterBoltzSessionBuilder) Read(reader io.Reader) BoltzSessionBuilder {
	return BoltzSessionBuilder{
		FfiConverterNetworkINSTANCE.Read(reader),
		FfiConverterAnyClientINSTANCE.Read(reader),
		FfiConverterOptionalUint64INSTANCE.Read(reader),
		FfiConverterOptionalMnemonicINSTANCE.Read(reader),
		FfiConverterOptionalLoggingINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterOptionalUint64INSTANCE.Read(reader),
		FfiConverterOptionalUint32INSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterOptionalForeignStoreLinkINSTANCE.Read(reader),
	}
}

func (c FfiConverterBoltzSessionBuilder) Lower(value BoltzSessionBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[BoltzSessionBuilder](c, value)
}

func (c FfiConverterBoltzSessionBuilder) LowerExternal(value BoltzSessionBuilder) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[BoltzSessionBuilder](c, value))
}

func (c FfiConverterBoltzSessionBuilder) Write(writer io.Writer, value BoltzSessionBuilder) {
	FfiConverterNetworkINSTANCE.Write(writer, value.Network)
	FfiConverterAnyClientINSTANCE.Write(writer, value.Client)
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.Timeout)
	FfiConverterOptionalMnemonicINSTANCE.Write(writer, value.Mnemonic)
	FfiConverterOptionalLoggingINSTANCE.Write(writer, value.Logging)
	FfiConverterBoolINSTANCE.Write(writer, value.Polling)
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.TimeoutAdvance)
	FfiConverterOptionalUint32INSTANCE.Write(writer, value.NextIndexToUse)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.ReferralId)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.BitcoinElectrumClientUrl)
	FfiConverterBoolINSTANCE.Write(writer, value.RandomPreimages)
	FfiConverterOptionalForeignStoreLinkINSTANCE.Write(writer, value.Store)
}

type FfiDestroyerBoltzSessionBuilder struct{}

func (_ FfiDestroyerBoltzSessionBuilder) Destroy(value BoltzSessionBuilder) {
	value.Destroy()
}

// A builder for the `EsploraClient`
type EsploraClientBuilder struct {
	BaseUrl     string
	Network     *Network
	Waterfalls  bool
	Concurrency *uint32
	Timeout     *uint8
	UtxoOnly    bool
}

func (r *EsploraClientBuilder) Destroy() {
	FfiDestroyerString{}.Destroy(r.BaseUrl)
	FfiDestroyerNetwork{}.Destroy(r.Network)
	FfiDestroyerBool{}.Destroy(r.Waterfalls)
	FfiDestroyerOptionalUint32{}.Destroy(r.Concurrency)
	FfiDestroyerOptionalUint8{}.Destroy(r.Timeout)
	FfiDestroyerBool{}.Destroy(r.UtxoOnly)
}

type FfiConverterEsploraClientBuilder struct{}

var FfiConverterEsploraClientBuilderINSTANCE = FfiConverterEsploraClientBuilder{}

func (c FfiConverterEsploraClientBuilder) Lift(rb RustBufferI) EsploraClientBuilder {
	return LiftFromRustBuffer[EsploraClientBuilder](c, rb)
}

func (c FfiConverterEsploraClientBuilder) Read(reader io.Reader) EsploraClientBuilder {
	return EsploraClientBuilder{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterNetworkINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterOptionalUint32INSTANCE.Read(reader),
		FfiConverterOptionalUint8INSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterEsploraClientBuilder) Lower(value EsploraClientBuilder) C.RustBuffer {
	return LowerIntoRustBuffer[EsploraClientBuilder](c, value)
}

func (c FfiConverterEsploraClientBuilder) LowerExternal(value EsploraClientBuilder) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[EsploraClientBuilder](c, value))
}

func (c FfiConverterEsploraClientBuilder) Write(writer io.Writer, value EsploraClientBuilder) {
	FfiConverterStringINSTANCE.Write(writer, value.BaseUrl)
	FfiConverterNetworkINSTANCE.Write(writer, value.Network)
	FfiConverterBoolINSTANCE.Write(writer, value.Waterfalls)
	FfiConverterOptionalUint32INSTANCE.Write(writer, value.Concurrency)
	FfiConverterOptionalUint8INSTANCE.Write(writer, value.Timeout)
	FfiConverterBoolINSTANCE.Write(writer, value.UtxoOnly)
}

type FfiDestroyerEsploraClientBuilder struct{}

func (_ FfiDestroyerEsploraClientBuilder) Destroy(value EsploraClientBuilder) {
	value.Destroy()
}

// Liquid BIP21 payment details
type LiquidBip21 struct {
	// The Liquid address
	Address *Address
	// The asset identifier
	Asset AssetId
	// The amount in satoshis
	Satoshi *uint64
}

func (r *LiquidBip21) Destroy() {
	FfiDestroyerAddress{}.Destroy(r.Address)
	FfiDestroyerTypeAssetId{}.Destroy(r.Asset)
	FfiDestroyerOptionalUint64{}.Destroy(r.Satoshi)
}

type FfiConverterLiquidBip21 struct{}

var FfiConverterLiquidBip21INSTANCE = FfiConverterLiquidBip21{}

func (c FfiConverterLiquidBip21) Lift(rb RustBufferI) LiquidBip21 {
	return LiftFromRustBuffer[LiquidBip21](c, rb)
}

func (c FfiConverterLiquidBip21) Read(reader io.Reader) LiquidBip21 {
	return LiquidBip21{
		FfiConverterAddressINSTANCE.Read(reader),
		FfiConverterTypeAssetIdINSTANCE.Read(reader),
		FfiConverterOptionalUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterLiquidBip21) Lower(value LiquidBip21) C.RustBuffer {
	return LowerIntoRustBuffer[LiquidBip21](c, value)
}

func (c FfiConverterLiquidBip21) LowerExternal(value LiquidBip21) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[LiquidBip21](c, value))
}

func (c FfiConverterLiquidBip21) Write(writer io.Writer, value LiquidBip21) {
	FfiConverterAddressINSTANCE.Write(writer, value.Address)
	FfiConverterTypeAssetIdINSTANCE.Write(writer, value.Asset)
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.Satoshi)
}

type FfiDestroyerLiquidBip21 struct{}

func (_ FfiDestroyerLiquidBip21) Destroy(value LiquidBip21) {
	value.Destroy()
}

// Quote result containing fee breakdown for a swap
type Quote struct {
	// Amount the user sends (before fees)
	SendAmount uint64
	// Amount the user will receive after fees
	ReceiveAmount uint64
	// Network/miner fee in satoshis
	NetworkFee uint64
	// Boltz service fee in satoshis
	BoltzFee uint64
	// Minimum amount for this swap pair
	Min uint64
	// Maximum amount for this swap pair
	Max uint64
}

func (r *Quote) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.SendAmount)
	FfiDestroyerUint64{}.Destroy(r.ReceiveAmount)
	FfiDestroyerUint64{}.Destroy(r.NetworkFee)
	FfiDestroyerUint64{}.Destroy(r.BoltzFee)
	FfiDestroyerUint64{}.Destroy(r.Min)
	FfiDestroyerUint64{}.Destroy(r.Max)
}

type FfiConverterQuote struct{}

var FfiConverterQuoteINSTANCE = FfiConverterQuote{}

func (c FfiConverterQuote) Lift(rb RustBufferI) Quote {
	return LiftFromRustBuffer[Quote](c, rb)
}

func (c FfiConverterQuote) Read(reader io.Reader) Quote {
	return Quote{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterQuote) Lower(value Quote) C.RustBuffer {
	return LowerIntoRustBuffer[Quote](c, value)
}

func (c FfiConverterQuote) LowerExternal(value Quote) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[Quote](c, value))
}

func (c FfiConverterQuote) Write(writer io.Writer, value Quote) {
	FfiConverterUint64INSTANCE.Write(writer, value.SendAmount)
	FfiConverterUint64INSTANCE.Write(writer, value.ReceiveAmount)
	FfiConverterUint64INSTANCE.Write(writer, value.NetworkFee)
	FfiConverterUint64INSTANCE.Write(writer, value.BoltzFee)
	FfiConverterUint64INSTANCE.Write(writer, value.Min)
	FfiConverterUint64INSTANCE.Write(writer, value.Max)
}

type FfiDestroyerQuote struct{}

func (_ FfiDestroyerQuote) Destroy(value Quote) {
	value.Destroy()
}

// see [`lwk_wollet::Chain`]
type Chain uint

const (
	// External address, shown when asked for a payment.
	// Wallet having a single descriptor are considered External
	ChainExternal Chain = 1
	// Internal address, used for the change
	ChainInternal Chain = 2
)

type FfiConverterChain struct{}

var FfiConverterChainINSTANCE = FfiConverterChain{}

func (c FfiConverterChain) Lift(rb RustBufferI) Chain {
	return LiftFromRustBuffer[Chain](c, rb)
}

func (c FfiConverterChain) Lower(value Chain) C.RustBuffer {
	return LowerIntoRustBuffer[Chain](c, value)
}

func (c FfiConverterChain) LowerExternal(value Chain) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[Chain](c, value))
}
func (FfiConverterChain) Read(reader io.Reader) Chain {
	id := readInt32(reader)
	return Chain(id)
}

func (FfiConverterChain) Write(writer io.Writer, value Chain) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerChain struct{}

func (_ FfiDestroyerChain) Destroy(value Chain) {
}

type DescriptorBlindingKey uint

const (
	DescriptorBlindingKeySlip77     DescriptorBlindingKey = 1
	DescriptorBlindingKeySlip77Rand DescriptorBlindingKey = 2
	DescriptorBlindingKeyElip151    DescriptorBlindingKey = 3
)

type FfiConverterDescriptorBlindingKey struct{}

var FfiConverterDescriptorBlindingKeyINSTANCE = FfiConverterDescriptorBlindingKey{}

func (c FfiConverterDescriptorBlindingKey) Lift(rb RustBufferI) DescriptorBlindingKey {
	return LiftFromRustBuffer[DescriptorBlindingKey](c, rb)
}

func (c FfiConverterDescriptorBlindingKey) Lower(value DescriptorBlindingKey) C.RustBuffer {
	return LowerIntoRustBuffer[DescriptorBlindingKey](c, value)
}

func (c FfiConverterDescriptorBlindingKey) LowerExternal(value DescriptorBlindingKey) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[DescriptorBlindingKey](c, value))
}
func (FfiConverterDescriptorBlindingKey) Read(reader io.Reader) DescriptorBlindingKey {
	id := readInt32(reader)
	return DescriptorBlindingKey(id)
}

func (FfiConverterDescriptorBlindingKey) Write(writer io.Writer, value DescriptorBlindingKey) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerDescriptorBlindingKey struct{}

func (_ FfiDestroyerDescriptorBlindingKey) Destroy(value DescriptorBlindingKey) {
}

// Log level for logging messages
type LogLevel uint

const (
	// Debug level
	LogLevelDebug LogLevel = 1
	// Info level
	LogLevelInfo LogLevel = 2
	// Warning level
	LogLevelWarn LogLevel = 3
	// Error level
	LogLevelError LogLevel = 4
)

type FfiConverterLogLevel struct{}

var FfiConverterLogLevelINSTANCE = FfiConverterLogLevel{}

func (c FfiConverterLogLevel) Lift(rb RustBufferI) LogLevel {
	return LiftFromRustBuffer[LogLevel](c, rb)
}

func (c FfiConverterLogLevel) Lower(value LogLevel) C.RustBuffer {
	return LowerIntoRustBuffer[LogLevel](c, value)
}

func (c FfiConverterLogLevel) LowerExternal(value LogLevel) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[LogLevel](c, value))
}
func (FfiConverterLogLevel) Read(reader io.Reader) LogLevel {
	id := readInt32(reader)
	return LogLevel(id)
}

func (FfiConverterLogLevel) Write(writer io.Writer, value LogLevel) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerLogLevel struct{}

func (_ FfiDestroyerLogLevel) Destroy(value LogLevel) {
}

// Possible errors emitted
type LwkError struct {
	err error
}

// Convience method to turn *LwkError into error
// Avoiding treating nil pointer as non nil error interface
func (err *LwkError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err LwkError) Error() string {
	return fmt.Sprintf("LwkError: %s", err.err.Error())
}

func (err LwkError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrLwkErrorGeneric = fmt.Errorf("LwkErrorGeneric")
var ErrLwkErrorPoisonError = fmt.Errorf("LwkErrorPoisonError")
var ErrLwkErrorMagicRoutingHint = fmt.Errorf("LwkErrorMagicRoutingHint")
var ErrLwkErrorSwapExpired = fmt.Errorf("LwkErrorSwapExpired")
var ErrLwkErrorNoBoltzUpdate = fmt.Errorf("LwkErrorNoBoltzUpdate")
var ErrLwkErrorObjectConsumed = fmt.Errorf("LwkErrorObjectConsumed")
var ErrLwkErrorBoltzBackendHttpError = fmt.Errorf("LwkErrorBoltzBackendHttpError")

// Variant structs
type LwkErrorGeneric struct {
	Msg string
}

func NewLwkErrorGeneric(
	msg string,
) *LwkError {
	return &LwkError{err: &LwkErrorGeneric{
		Msg: msg}}
}

func (e LwkErrorGeneric) destroy() {
	FfiDestroyerString{}.Destroy(e.Msg)
}

func (err LwkErrorGeneric) Error() string {
	return fmt.Sprint("Generic",
		": ",

		"Msg=",
		err.Msg,
	)
}

func (self LwkErrorGeneric) Is(target error) bool {
	return target == ErrLwkErrorGeneric
}

type LwkErrorPoisonError struct {
	Msg string
}

func NewLwkErrorPoisonError(
	msg string,
) *LwkError {
	return &LwkError{err: &LwkErrorPoisonError{
		Msg: msg}}
}

func (e LwkErrorPoisonError) destroy() {
	FfiDestroyerString{}.Destroy(e.Msg)
}

func (err LwkErrorPoisonError) Error() string {
	return fmt.Sprint("PoisonError",
		": ",

		"Msg=",
		err.Msg,
	)
}

func (self LwkErrorPoisonError) Is(target error) bool {
	return target == ErrLwkErrorPoisonError
}

type LwkErrorMagicRoutingHint struct {
	Address string
	Amount  uint64
	Uri     string
}

func NewLwkErrorMagicRoutingHint(
	address string,
	amount uint64,
	uri string,
) *LwkError {
	return &LwkError{err: &LwkErrorMagicRoutingHint{
		Address: address,
		Amount:  amount,
		Uri:     uri}}
}

func (e LwkErrorMagicRoutingHint) destroy() {
	FfiDestroyerString{}.Destroy(e.Address)
	FfiDestroyerUint64{}.Destroy(e.Amount)
	FfiDestroyerString{}.Destroy(e.Uri)
}

func (err LwkErrorMagicRoutingHint) Error() string {
	return fmt.Sprint("MagicRoutingHint",
		": ",

		"Address=",
		err.Address,
		", ",
		"Amount=",
		err.Amount,
		", ",
		"Uri=",
		err.Uri,
	)
}

func (self LwkErrorMagicRoutingHint) Is(target error) bool {
	return target == ErrLwkErrorMagicRoutingHint
}

type LwkErrorSwapExpired struct {
	SwapId string
	Status string
}

func NewLwkErrorSwapExpired(
	swapId string,
	status string,
) *LwkError {
	return &LwkError{err: &LwkErrorSwapExpired{
		SwapId: swapId,
		Status: status}}
}

func (e LwkErrorSwapExpired) destroy() {
	FfiDestroyerString{}.Destroy(e.SwapId)
	FfiDestroyerString{}.Destroy(e.Status)
}

func (err LwkErrorSwapExpired) Error() string {
	return fmt.Sprint("SwapExpired",
		": ",

		"SwapId=",
		err.SwapId,
		", ",
		"Status=",
		err.Status,
	)
}

func (self LwkErrorSwapExpired) Is(target error) bool {
	return target == ErrLwkErrorSwapExpired
}

type LwkErrorNoBoltzUpdate struct {
}

func NewLwkErrorNoBoltzUpdate() *LwkError {
	return &LwkError{err: &LwkErrorNoBoltzUpdate{}}
}

func (e LwkErrorNoBoltzUpdate) destroy() {
}

func (err LwkErrorNoBoltzUpdate) Error() string {
	return fmt.Sprint("NoBoltzUpdate")
}

func (self LwkErrorNoBoltzUpdate) Is(target error) bool {
	return target == ErrLwkErrorNoBoltzUpdate
}

type LwkErrorObjectConsumed struct {
}

func NewLwkErrorObjectConsumed() *LwkError {
	return &LwkError{err: &LwkErrorObjectConsumed{}}
}

func (e LwkErrorObjectConsumed) destroy() {
}

func (err LwkErrorObjectConsumed) Error() string {
	return fmt.Sprint("ObjectConsumed")
}

func (self LwkErrorObjectConsumed) Is(target error) bool {
	return target == ErrLwkErrorObjectConsumed
}

type LwkErrorBoltzBackendHttpError struct {
	Status uint16
	Error_ *string
}

func NewLwkErrorBoltzBackendHttpError(
	status uint16,
	error *string,
) *LwkError {
	return &LwkError{err: &LwkErrorBoltzBackendHttpError{
		Status: status,
		Error_: error}}
}

func (e LwkErrorBoltzBackendHttpError) destroy() {
	FfiDestroyerUint16{}.Destroy(e.Status)
	FfiDestroyerOptionalString{}.Destroy(e.Error_)
}

func (err LwkErrorBoltzBackendHttpError) Error() string {
	return fmt.Sprint("BoltzBackendHttpError",
		": ",

		"Status=",
		err.Status,
		", ",
		"Error_=",
		err.Error_,
	)
}

func (self LwkErrorBoltzBackendHttpError) Is(target error) bool {
	return target == ErrLwkErrorBoltzBackendHttpError
}

type FfiConverterLwkError struct{}

var FfiConverterLwkErrorINSTANCE = FfiConverterLwkError{}

func (c FfiConverterLwkError) Lift(eb RustBufferI) *LwkError {
	return LiftFromRustBuffer[*LwkError](c, eb)
}

func (c FfiConverterLwkError) Lower(value *LwkError) C.RustBuffer {
	return LowerIntoRustBuffer[*LwkError](c, value)
}

func (c FfiConverterLwkError) LowerExternal(value *LwkError) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[*LwkError](c, value))
}

func (c FfiConverterLwkError) Read(reader io.Reader) *LwkError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &LwkError{&LwkErrorGeneric{
			Msg: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 2:
		return &LwkError{&LwkErrorPoisonError{
			Msg: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 3:
		return &LwkError{&LwkErrorMagicRoutingHint{
			Address: FfiConverterStringINSTANCE.Read(reader),
			Amount:  FfiConverterUint64INSTANCE.Read(reader),
			Uri:     FfiConverterStringINSTANCE.Read(reader),
		}}
	case 4:
		return &LwkError{&LwkErrorSwapExpired{
			SwapId: FfiConverterStringINSTANCE.Read(reader),
			Status: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 5:
		return &LwkError{&LwkErrorNoBoltzUpdate{}}
	case 6:
		return &LwkError{&LwkErrorObjectConsumed{}}
	case 7:
		return &LwkError{&LwkErrorBoltzBackendHttpError{
			Status: FfiConverterUint16INSTANCE.Read(reader),
			Error_: FfiConverterOptionalStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterLwkError.Read()", errorID))
	}
}

func (c FfiConverterLwkError) Write(writer io.Writer, value *LwkError) {
	switch variantValue := value.err.(type) {
	case *LwkErrorGeneric:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Msg)
	case *LwkErrorPoisonError:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Msg)
	case *LwkErrorMagicRoutingHint:
		writeInt32(writer, 3)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Address)
		FfiConverterUint64INSTANCE.Write(writer, variantValue.Amount)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Uri)
	case *LwkErrorSwapExpired:
		writeInt32(writer, 4)
		FfiConverterStringINSTANCE.Write(writer, variantValue.SwapId)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Status)
	case *LwkErrorNoBoltzUpdate:
		writeInt32(writer, 5)
	case *LwkErrorObjectConsumed:
		writeInt32(writer, 6)
	case *LwkErrorBoltzBackendHttpError:
		writeInt32(writer, 7)
		FfiConverterUint16INSTANCE.Write(writer, variantValue.Status)
		FfiConverterOptionalStringINSTANCE.Write(writer, variantValue.Error_)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterLwkError.Write", value))
	}
}

type FfiDestroyerLwkError struct{}

func (_ FfiDestroyerLwkError) Destroy(value *LwkError) {
	switch variantValue := value.err.(type) {
	case LwkErrorGeneric:
		variantValue.destroy()
	case LwkErrorPoisonError:
		variantValue.destroy()
	case LwkErrorMagicRoutingHint:
		variantValue.destroy()
	case LwkErrorSwapExpired:
		variantValue.destroy()
	case LwkErrorNoBoltzUpdate:
		variantValue.destroy()
	case LwkErrorObjectConsumed:
		variantValue.destroy()
	case LwkErrorBoltzBackendHttpError:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerLwkError.Destroy", value))
	}
}

// The kind/type of a payment category without the associated data
type PaymentKind uint

const (
	// A Bitcoin address
	PaymentKindBitcoinAddress PaymentKind = 1
	// A Liquid address
	PaymentKindLiquidAddress PaymentKind = 2
	// A Lightning BOLT11 invoice
	PaymentKindLightningInvoice PaymentKind = 3
	// A Lightning BOLT12 offer
	PaymentKindLightningOffer PaymentKind = 4
	// An LNURL
	PaymentKindLnUrl PaymentKind = 5
	// A BIP353 payment instruction (₿user@domain)
	PaymentKindBip353 PaymentKind = 6
	// A BIP21 URI
	PaymentKindBip21 PaymentKind = 7
	// A BIP321 URI (BIP21 without address but with payment method)
	PaymentKindBip321 PaymentKind = 8
	// A Liquid BIP21 URI with amount and asset
	PaymentKindLiquidBip21 PaymentKind = 9
)

type FfiConverterPaymentKind struct{}

var FfiConverterPaymentKindINSTANCE = FfiConverterPaymentKind{}

func (c FfiConverterPaymentKind) Lift(rb RustBufferI) PaymentKind {
	return LiftFromRustBuffer[PaymentKind](c, rb)
}

func (c FfiConverterPaymentKind) Lower(value PaymentKind) C.RustBuffer {
	return LowerIntoRustBuffer[PaymentKind](c, value)
}

func (c FfiConverterPaymentKind) LowerExternal(value PaymentKind) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[PaymentKind](c, value))
}
func (FfiConverterPaymentKind) Read(reader io.Reader) PaymentKind {
	id := readInt32(reader)
	return PaymentKind(id)
}

func (FfiConverterPaymentKind) Write(writer io.Writer, value PaymentKind) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerPaymentKind struct{}

func (_ FfiDestroyerPaymentKind) Destroy(value PaymentKind) {
}

type PaymentState uint

const (
	PaymentStateContinue PaymentState = 1
	PaymentStateSuccess  PaymentState = 2
	PaymentStateFailed   PaymentState = 3
)

type FfiConverterPaymentState struct{}

var FfiConverterPaymentStateINSTANCE = FfiConverterPaymentState{}

func (c FfiConverterPaymentState) Lift(rb RustBufferI) PaymentState {
	return LiftFromRustBuffer[PaymentState](c, rb)
}

func (c FfiConverterPaymentState) Lower(value PaymentState) C.RustBuffer {
	return LowerIntoRustBuffer[PaymentState](c, value)
}

func (c FfiConverterPaymentState) LowerExternal(value PaymentState) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[PaymentState](c, value))
}
func (FfiConverterPaymentState) Read(reader io.Reader) PaymentState {
	id := readInt32(reader)
	return PaymentState(id)
}

func (FfiConverterPaymentState) Write(writer io.Writer, value PaymentState) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerPaymentState struct{}

func (_ FfiDestroyerPaymentState) Destroy(value PaymentState) {
}

type Singlesig uint

const (
	SinglesigWpkh   Singlesig = 1
	SinglesigShWpkh Singlesig = 2
)

type FfiConverterSinglesig struct{}

var FfiConverterSinglesigINSTANCE = FfiConverterSinglesig{}

func (c FfiConverterSinglesig) Lift(rb RustBufferI) Singlesig {
	return LiftFromRustBuffer[Singlesig](c, rb)
}

func (c FfiConverterSinglesig) Lower(value Singlesig) C.RustBuffer {
	return LowerIntoRustBuffer[Singlesig](c, value)
}

func (c FfiConverterSinglesig) LowerExternal(value Singlesig) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[Singlesig](c, value))
}
func (FfiConverterSinglesig) Read(reader io.Reader) Singlesig {
	id := readInt32(reader)
	return Singlesig(id)
}

func (FfiConverterSinglesig) Write(writer io.Writer, value Singlesig) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerSinglesig struct{}

func (_ FfiDestroyerSinglesig) Destroy(value Singlesig) {
}

// Asset type for swap quotes
type SwapAsset uint

const (
	// Lightning Bitcoin (for reverse/submarine swaps)
	SwapAssetLightning SwapAsset = 1
	// Onchain Bitcoin (for chain swaps)
	SwapAssetOnchain SwapAsset = 2
	// Liquid Bitcoin (onchain)
	SwapAssetLiquid SwapAsset = 3
)

type FfiConverterSwapAsset struct{}

var FfiConverterSwapAssetINSTANCE = FfiConverterSwapAsset{}

func (c FfiConverterSwapAsset) Lift(rb RustBufferI) SwapAsset {
	return LiftFromRustBuffer[SwapAsset](c, rb)
}

func (c FfiConverterSwapAsset) Lower(value SwapAsset) C.RustBuffer {
	return LowerIntoRustBuffer[SwapAsset](c, value)
}

func (c FfiConverterSwapAsset) LowerExternal(value SwapAsset) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[SwapAsset](c, value))
}
func (FfiConverterSwapAsset) Read(reader io.Reader) SwapAsset {
	id := readInt32(reader)
	return SwapAsset(id)
}

func (FfiConverterSwapAsset) Write(writer io.Writer, value SwapAsset) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerSwapAsset struct{}

func (_ FfiDestroyerSwapAsset) Destroy(value SwapAsset) {
}

type FfiConverterOptionalUint8 struct{}

var FfiConverterOptionalUint8INSTANCE = FfiConverterOptionalUint8{}

func (c FfiConverterOptionalUint8) Lift(rb RustBufferI) *uint8 {
	return LiftFromRustBuffer[*uint8](c, rb)
}

func (_ FfiConverterOptionalUint8) Read(reader io.Reader) *uint8 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint8INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint8) Lower(value *uint8) C.RustBuffer {
	return LowerIntoRustBuffer[*uint8](c, value)
}

func (c FfiConverterOptionalUint8) LowerExternal(value *uint8) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[*uint8](c, value))
}

func (_ FfiConverterOptionalUint8) Write(writer io.Writer, value *uint8) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint8INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint8 struct{}

func (_ FfiDestroyerOptionalUint8) Destroy(value *uint8) {
	if value != nil {
		FfiDestroyerUint8{}.Destroy(*value)
	}
}

type FfiConverterOptionalUint32 struct{}

var FfiConverterOptionalUint32INSTANCE = FfiConverterOptionalUint32{}

func (c FfiConverterOptionalUint32) Lift(rb RustBufferI) *uint32 {
	return LiftFromRustBuffer[*uint32](c, rb)
}

func (_ FfiConverterOptionalUint32) Read(reader io.Reader) *uint32 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint32INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint32) Lower(value *uint32) C.RustBuffer {
	return LowerIntoRustBuffer[*uint32](c, value)
}

func (c FfiConverterOptionalUint32) LowerExternal(value *uint32) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[*uint32](c, value))
}

func (_ FfiConverterOptionalUint32) Write(writer io.Writer, value *uint32) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint32INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint32 struct{}

func (_ FfiDestroyerOptionalUint32) Destroy(value *uint32) {
	if value != nil {
		FfiDestroyerUint32{}.Destroy(*value)
	}
}

type FfiConverterOptionalUint64 struct{}

var FfiConverterOptionalUint64INSTANCE = FfiConverterOptionalUint64{}

func (c FfiConverterOptionalUint64) Lift(rb RustBufferI) *uint64 {
	return LiftFromRustBuffer[*uint64](c, rb)
}

func (_ FfiConverterOptionalUint64) Read(reader io.Reader) *uint64 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint64INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint64) Lower(value *uint64) C.RustBuffer {
	return LowerIntoRustBuffer[*uint64](c, value)
}

func (c FfiConverterOptionalUint64) LowerExternal(value *uint64) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[*uint64](c, value))
}

func (_ FfiConverterOptionalUint64) Write(writer io.Writer, value *uint64) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint64INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint64 struct{}

func (_ FfiDestroyerOptionalUint64) Destroy(value *uint64) {
	if value != nil {
		FfiDestroyerUint64{}.Destroy(*value)
	}
}

type FfiConverterOptionalFloat32 struct{}

var FfiConverterOptionalFloat32INSTANCE = FfiConverterOptionalFloat32{}

func (c FfiConverterOptionalFloat32) Lift(rb RustBufferI) *float32 {
	return LiftFromRustBuffer[*float32](c, rb)
}

func (_ FfiConverterOptionalFloat32) Read(reader io.Reader) *float32 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterFloat32INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalFloat32) Lower(value *float32) C.RustBuffer {
	return LowerIntoRustBuffer[*float32](c, value)
}

func (c FfiConverterOptionalFloat32) LowerExternal(value *float32) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[*float32](c, value))
}

func (_ FfiConverterOptionalFloat32) Write(writer io.Writer, value *float32) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterFloat32INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalFloat32 struct{}

func (_ FfiDestroyerOptionalFloat32) Destroy(value *float32) {
	if value != nil {
		FfiDestroyerFloat32{}.Destroy(*value)
	}
}

type FfiConverterOptionalBool struct{}

var FfiConverterOptionalBoolINSTANCE = FfiConverterOptionalBool{}

func (c FfiConverterOptionalBool) Lift(rb RustBufferI) *bool {
	return LiftFromRustBuffer[*bool](c, rb)
}

func (_ FfiConverterOptionalBool) Read(reader io.Reader) *bool {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterBoolINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalBool) Lower(value *bool) C.RustBuffer {
	return LowerIntoRustBuffer[*bool](c, value)
}

func (c FfiConverterOptionalBool) LowerExternal(value *bool) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[*bool](c, value))
}

func (_ FfiConverterOptionalBool) Write(writer io.Writer, value *bool) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterBoolINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalBool struct{}

func (_ FfiDestroyerOptionalBool) Destroy(value *bool) {
	if value != nil {
		FfiDestroyerBool{}.Destroy(*value)
	}
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

func (c FfiConverterOptionalString) LowerExternal(value *string) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[*string](c, value))
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

type FfiConverterOptionalBytes struct{}

var FfiConverterOptionalBytesINSTANCE = FfiConverterOptionalBytes{}

func (c FfiConverterOptionalBytes) Lift(rb RustBufferI) *[]byte {
	return LiftFromRustBuffer[*[]byte](c, rb)
}

func (_ FfiConverterOptionalBytes) Read(reader io.Reader) *[]byte {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterBytesINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalBytes) Lower(value *[]byte) C.RustBuffer {
	return LowerIntoRustBuffer[*[]byte](c, value)
}

func (c FfiConverterOptionalBytes) LowerExternal(value *[]byte) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[*[]byte](c, value))
}

func (_ FfiConverterOptionalBytes) Write(writer io.Writer, value *[]byte) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterBytesINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalBytes struct{}

func (_ FfiDestroyerOptionalBytes) Destroy(value *[]byte) {
	if value != nil {
		FfiDestroyerBytes{}.Destroy(*value)
	}
}

type FfiConverterOptionalAddress struct{}

var FfiConverterOptionalAddressINSTANCE = FfiConverterOptionalAddress{}

func (c FfiConverterOptionalAddress) Lift(rb RustBufferI) **Address {
	return LiftFromRustBuffer[**Address](c, rb)
}

func (_ FfiConverterOptionalAddress) Read(reader io.Reader) **Address {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterAddressINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalAddress) Lower(value **Address) C.RustBuffer {
	return LowerIntoRustBuffer[**Address](c, value)
}

func (c FfiConverterOptionalAddress) LowerExternal(value **Address) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**Address](c, value))
}

func (_ FfiConverterOptionalAddress) Write(writer io.Writer, value **Address) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterAddressINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalAddress struct{}

func (_ FfiDestroyerOptionalAddress) Destroy(value **Address) {
	if value != nil {
		FfiDestroyerAddress{}.Destroy(*value)
	}
}

type FfiConverterOptionalBip21 struct{}

var FfiConverterOptionalBip21INSTANCE = FfiConverterOptionalBip21{}

func (c FfiConverterOptionalBip21) Lift(rb RustBufferI) **Bip21 {
	return LiftFromRustBuffer[**Bip21](c, rb)
}

func (_ FfiConverterOptionalBip21) Read(reader io.Reader) **Bip21 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterBip21INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalBip21) Lower(value **Bip21) C.RustBuffer {
	return LowerIntoRustBuffer[**Bip21](c, value)
}

func (c FfiConverterOptionalBip21) LowerExternal(value **Bip21) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**Bip21](c, value))
}

func (_ FfiConverterOptionalBip21) Write(writer io.Writer, value **Bip21) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterBip21INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalBip21 struct{}

func (_ FfiDestroyerOptionalBip21) Destroy(value **Bip21) {
	if value != nil {
		FfiDestroyerBip21{}.Destroy(*value)
	}
}

type FfiConverterOptionalBip321 struct{}

var FfiConverterOptionalBip321INSTANCE = FfiConverterOptionalBip321{}

func (c FfiConverterOptionalBip321) Lift(rb RustBufferI) **Bip321 {
	return LiftFromRustBuffer[**Bip321](c, rb)
}

func (_ FfiConverterOptionalBip321) Read(reader io.Reader) **Bip321 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterBip321INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalBip321) Lower(value **Bip321) C.RustBuffer {
	return LowerIntoRustBuffer[**Bip321](c, value)
}

func (c FfiConverterOptionalBip321) LowerExternal(value **Bip321) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**Bip321](c, value))
}

func (_ FfiConverterOptionalBip321) Write(writer io.Writer, value **Bip321) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterBip321INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalBip321 struct{}

func (_ FfiDestroyerOptionalBip321) Destroy(value **Bip321) {
	if value != nil {
		FfiDestroyerBip321{}.Destroy(*value)
	}
}

type FfiConverterOptionalBitcoinAddress struct{}

var FfiConverterOptionalBitcoinAddressINSTANCE = FfiConverterOptionalBitcoinAddress{}

func (c FfiConverterOptionalBitcoinAddress) Lift(rb RustBufferI) **BitcoinAddress {
	return LiftFromRustBuffer[**BitcoinAddress](c, rb)
}

func (_ FfiConverterOptionalBitcoinAddress) Read(reader io.Reader) **BitcoinAddress {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterBitcoinAddressINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalBitcoinAddress) Lower(value **BitcoinAddress) C.RustBuffer {
	return LowerIntoRustBuffer[**BitcoinAddress](c, value)
}

func (c FfiConverterOptionalBitcoinAddress) LowerExternal(value **BitcoinAddress) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**BitcoinAddress](c, value))
}

func (_ FfiConverterOptionalBitcoinAddress) Write(writer io.Writer, value **BitcoinAddress) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterBitcoinAddressINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalBitcoinAddress struct{}

func (_ FfiDestroyerOptionalBitcoinAddress) Destroy(value **BitcoinAddress) {
	if value != nil {
		FfiDestroyerBitcoinAddress{}.Destroy(*value)
	}
}

type FfiConverterOptionalBolt11Invoice struct{}

var FfiConverterOptionalBolt11InvoiceINSTANCE = FfiConverterOptionalBolt11Invoice{}

func (c FfiConverterOptionalBolt11Invoice) Lift(rb RustBufferI) **Bolt11Invoice {
	return LiftFromRustBuffer[**Bolt11Invoice](c, rb)
}

func (_ FfiConverterOptionalBolt11Invoice) Read(reader io.Reader) **Bolt11Invoice {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterBolt11InvoiceINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalBolt11Invoice) Lower(value **Bolt11Invoice) C.RustBuffer {
	return LowerIntoRustBuffer[**Bolt11Invoice](c, value)
}

func (c FfiConverterOptionalBolt11Invoice) LowerExternal(value **Bolt11Invoice) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**Bolt11Invoice](c, value))
}

func (_ FfiConverterOptionalBolt11Invoice) Write(writer io.Writer, value **Bolt11Invoice) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterBolt11InvoiceINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalBolt11Invoice struct{}

func (_ FfiDestroyerOptionalBolt11Invoice) Destroy(value **Bolt11Invoice) {
	if value != nil {
		FfiDestroyerBolt11Invoice{}.Destroy(*value)
	}
}

type FfiConverterOptionalContract struct{}

var FfiConverterOptionalContractINSTANCE = FfiConverterOptionalContract{}

func (c FfiConverterOptionalContract) Lift(rb RustBufferI) **Contract {
	return LiftFromRustBuffer[**Contract](c, rb)
}

func (_ FfiConverterOptionalContract) Read(reader io.Reader) **Contract {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterContractINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalContract) Lower(value **Contract) C.RustBuffer {
	return LowerIntoRustBuffer[**Contract](c, value)
}

func (c FfiConverterOptionalContract) LowerExternal(value **Contract) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**Contract](c, value))
}

func (_ FfiConverterOptionalContract) Write(writer io.Writer, value **Contract) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterContractINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalContract struct{}

func (_ FfiDestroyerOptionalContract) Destroy(value **Contract) {
	if value != nil {
		FfiDestroyerContract{}.Destroy(*value)
	}
}

type FfiConverterOptionalForeignStoreLink struct{}

var FfiConverterOptionalForeignStoreLinkINSTANCE = FfiConverterOptionalForeignStoreLink{}

func (c FfiConverterOptionalForeignStoreLink) Lift(rb RustBufferI) **ForeignStoreLink {
	return LiftFromRustBuffer[**ForeignStoreLink](c, rb)
}

func (_ FfiConverterOptionalForeignStoreLink) Read(reader io.Reader) **ForeignStoreLink {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterForeignStoreLinkINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalForeignStoreLink) Lower(value **ForeignStoreLink) C.RustBuffer {
	return LowerIntoRustBuffer[**ForeignStoreLink](c, value)
}

func (c FfiConverterOptionalForeignStoreLink) LowerExternal(value **ForeignStoreLink) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**ForeignStoreLink](c, value))
}

func (_ FfiConverterOptionalForeignStoreLink) Write(writer io.Writer, value **ForeignStoreLink) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterForeignStoreLinkINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalForeignStoreLink struct{}

func (_ FfiDestroyerOptionalForeignStoreLink) Destroy(value **ForeignStoreLink) {
	if value != nil {
		FfiDestroyerForeignStoreLink{}.Destroy(*value)
	}
}

type FfiConverterOptionalIssuance struct{}

var FfiConverterOptionalIssuanceINSTANCE = FfiConverterOptionalIssuance{}

func (c FfiConverterOptionalIssuance) Lift(rb RustBufferI) **Issuance {
	return LiftFromRustBuffer[**Issuance](c, rb)
}

func (_ FfiConverterOptionalIssuance) Read(reader io.Reader) **Issuance {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterIssuanceINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalIssuance) Lower(value **Issuance) C.RustBuffer {
	return LowerIntoRustBuffer[**Issuance](c, value)
}

func (c FfiConverterOptionalIssuance) LowerExternal(value **Issuance) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**Issuance](c, value))
}

func (_ FfiConverterOptionalIssuance) Write(writer io.Writer, value **Issuance) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterIssuanceINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalIssuance struct{}

func (_ FfiDestroyerOptionalIssuance) Destroy(value **Issuance) {
	if value != nil {
		FfiDestroyerIssuance{}.Destroy(*value)
	}
}

type FfiConverterOptionalLightningPayment struct{}

var FfiConverterOptionalLightningPaymentINSTANCE = FfiConverterOptionalLightningPayment{}

func (c FfiConverterOptionalLightningPayment) Lift(rb RustBufferI) **LightningPayment {
	return LiftFromRustBuffer[**LightningPayment](c, rb)
}

func (_ FfiConverterOptionalLightningPayment) Read(reader io.Reader) **LightningPayment {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterLightningPaymentINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalLightningPayment) Lower(value **LightningPayment) C.RustBuffer {
	return LowerIntoRustBuffer[**LightningPayment](c, value)
}

func (c FfiConverterOptionalLightningPayment) LowerExternal(value **LightningPayment) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**LightningPayment](c, value))
}

func (_ FfiConverterOptionalLightningPayment) Write(writer io.Writer, value **LightningPayment) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterLightningPaymentINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalLightningPayment struct{}

func (_ FfiDestroyerOptionalLightningPayment) Destroy(value **LightningPayment) {
	if value != nil {
		FfiDestroyerLightningPayment{}.Destroy(*value)
	}
}

type FfiConverterOptionalLogging struct{}

var FfiConverterOptionalLoggingINSTANCE = FfiConverterOptionalLogging{}

func (c FfiConverterOptionalLogging) Lift(rb RustBufferI) *Logging {
	return LiftFromRustBuffer[*Logging](c, rb)
}

func (_ FfiConverterOptionalLogging) Read(reader io.Reader) *Logging {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterLoggingINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalLogging) Lower(value *Logging) C.RustBuffer {
	return LowerIntoRustBuffer[*Logging](c, value)
}

func (c FfiConverterOptionalLogging) LowerExternal(value *Logging) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[*Logging](c, value))
}

func (_ FfiConverterOptionalLogging) Write(writer io.Writer, value *Logging) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterLoggingINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalLogging struct{}

func (_ FfiDestroyerOptionalLogging) Destroy(value *Logging) {
	if value != nil {
		FfiDestroyerLogging{}.Destroy(*value)
	}
}

type FfiConverterOptionalMnemonic struct{}

var FfiConverterOptionalMnemonicINSTANCE = FfiConverterOptionalMnemonic{}

func (c FfiConverterOptionalMnemonic) Lift(rb RustBufferI) **Mnemonic {
	return LiftFromRustBuffer[**Mnemonic](c, rb)
}

func (_ FfiConverterOptionalMnemonic) Read(reader io.Reader) **Mnemonic {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterMnemonicINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalMnemonic) Lower(value **Mnemonic) C.RustBuffer {
	return LowerIntoRustBuffer[**Mnemonic](c, value)
}

func (c FfiConverterOptionalMnemonic) LowerExternal(value **Mnemonic) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**Mnemonic](c, value))
}

func (_ FfiConverterOptionalMnemonic) Write(writer io.Writer, value **Mnemonic) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterMnemonicINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalMnemonic struct{}

func (_ FfiDestroyerOptionalMnemonic) Destroy(value **Mnemonic) {
	if value != nil {
		FfiDestroyerMnemonic{}.Destroy(*value)
	}
}

type FfiConverterOptionalScript struct{}

var FfiConverterOptionalScriptINSTANCE = FfiConverterOptionalScript{}

func (c FfiConverterOptionalScript) Lift(rb RustBufferI) **Script {
	return LiftFromRustBuffer[**Script](c, rb)
}

func (_ FfiConverterOptionalScript) Read(reader io.Reader) **Script {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterScriptINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalScript) Lower(value **Script) C.RustBuffer {
	return LowerIntoRustBuffer[**Script](c, value)
}

func (c FfiConverterOptionalScript) LowerExternal(value **Script) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**Script](c, value))
}

func (_ FfiConverterOptionalScript) Write(writer io.Writer, value **Script) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterScriptINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalScript struct{}

func (_ FfiDestroyerOptionalScript) Destroy(value **Script) {
	if value != nil {
		FfiDestroyerScript{}.Destroy(*value)
	}
}

type FfiConverterOptionalSecretKey struct{}

var FfiConverterOptionalSecretKeyINSTANCE = FfiConverterOptionalSecretKey{}

func (c FfiConverterOptionalSecretKey) Lift(rb RustBufferI) **SecretKey {
	return LiftFromRustBuffer[**SecretKey](c, rb)
}

func (_ FfiConverterOptionalSecretKey) Read(reader io.Reader) **SecretKey {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterSecretKeyINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSecretKey) Lower(value **SecretKey) C.RustBuffer {
	return LowerIntoRustBuffer[**SecretKey](c, value)
}

func (c FfiConverterOptionalSecretKey) LowerExternal(value **SecretKey) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**SecretKey](c, value))
}

func (_ FfiConverterOptionalSecretKey) Write(writer io.Writer, value **SecretKey) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterSecretKeyINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSecretKey struct{}

func (_ FfiDestroyerOptionalSecretKey) Destroy(value **SecretKey) {
	if value != nil {
		FfiDestroyerSecretKey{}.Destroy(*value)
	}
}

type FfiConverterOptionalTransaction struct{}

var FfiConverterOptionalTransactionINSTANCE = FfiConverterOptionalTransaction{}

func (c FfiConverterOptionalTransaction) Lift(rb RustBufferI) **Transaction {
	return LiftFromRustBuffer[**Transaction](c, rb)
}

func (_ FfiConverterOptionalTransaction) Read(reader io.Reader) **Transaction {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterTransactionINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalTransaction) Lower(value **Transaction) C.RustBuffer {
	return LowerIntoRustBuffer[**Transaction](c, value)
}

func (c FfiConverterOptionalTransaction) LowerExternal(value **Transaction) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**Transaction](c, value))
}

func (_ FfiConverterOptionalTransaction) Write(writer io.Writer, value **Transaction) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterTransactionINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalTransaction struct{}

func (_ FfiDestroyerOptionalTransaction) Destroy(value **Transaction) {
	if value != nil {
		FfiDestroyerTransaction{}.Destroy(*value)
	}
}

type FfiConverterOptionalTxid struct{}

var FfiConverterOptionalTxidINSTANCE = FfiConverterOptionalTxid{}

func (c FfiConverterOptionalTxid) Lift(rb RustBufferI) **Txid {
	return LiftFromRustBuffer[**Txid](c, rb)
}

func (_ FfiConverterOptionalTxid) Read(reader io.Reader) **Txid {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterTxidINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalTxid) Lower(value **Txid) C.RustBuffer {
	return LowerIntoRustBuffer[**Txid](c, value)
}

func (c FfiConverterOptionalTxid) LowerExternal(value **Txid) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**Txid](c, value))
}

func (_ FfiConverterOptionalTxid) Write(writer io.Writer, value **Txid) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterTxidINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalTxid struct{}

func (_ FfiDestroyerOptionalTxid) Destroy(value **Txid) {
	if value != nil {
		FfiDestroyerTxid{}.Destroy(*value)
	}
}

type FfiConverterOptionalUpdate struct{}

var FfiConverterOptionalUpdateINSTANCE = FfiConverterOptionalUpdate{}

func (c FfiConverterOptionalUpdate) Lift(rb RustBufferI) **Update {
	return LiftFromRustBuffer[**Update](c, rb)
}

func (_ FfiConverterOptionalUpdate) Read(reader io.Reader) **Update {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUpdateINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUpdate) Lower(value **Update) C.RustBuffer {
	return LowerIntoRustBuffer[**Update](c, value)
}

func (c FfiConverterOptionalUpdate) LowerExternal(value **Update) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**Update](c, value))
}

func (_ FfiConverterOptionalUpdate) Write(writer io.Writer, value **Update) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUpdateINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUpdate struct{}

func (_ FfiDestroyerOptionalUpdate) Destroy(value **Update) {
	if value != nil {
		FfiDestroyerUpdate{}.Destroy(*value)
	}
}

type FfiConverterOptionalWalletTxOut struct{}

var FfiConverterOptionalWalletTxOutINSTANCE = FfiConverterOptionalWalletTxOut{}

func (c FfiConverterOptionalWalletTxOut) Lift(rb RustBufferI) **WalletTxOut {
	return LiftFromRustBuffer[**WalletTxOut](c, rb)
}

func (_ FfiConverterOptionalWalletTxOut) Read(reader io.Reader) **WalletTxOut {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterWalletTxOutINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalWalletTxOut) Lower(value **WalletTxOut) C.RustBuffer {
	return LowerIntoRustBuffer[**WalletTxOut](c, value)
}

func (c FfiConverterOptionalWalletTxOut) LowerExternal(value **WalletTxOut) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**WalletTxOut](c, value))
}

func (_ FfiConverterOptionalWalletTxOut) Write(writer io.Writer, value **WalletTxOut) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterWalletTxOutINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalWalletTxOut struct{}

func (_ FfiDestroyerOptionalWalletTxOut) Destroy(value **WalletTxOut) {
	if value != nil {
		FfiDestroyerWalletTxOut{}.Destroy(*value)
	}
}

type FfiConverterOptionalWebHook struct{}

var FfiConverterOptionalWebHookINSTANCE = FfiConverterOptionalWebHook{}

func (c FfiConverterOptionalWebHook) Lift(rb RustBufferI) **WebHook {
	return LiftFromRustBuffer[**WebHook](c, rb)
}

func (_ FfiConverterOptionalWebHook) Read(reader io.Reader) **WebHook {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterWebHookINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalWebHook) Lower(value **WebHook) C.RustBuffer {
	return LowerIntoRustBuffer[**WebHook](c, value)
}

func (c FfiConverterOptionalWebHook) LowerExternal(value **WebHook) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[**WebHook](c, value))
}

func (_ FfiConverterOptionalWebHook) Write(writer io.Writer, value **WebHook) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterWebHookINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalWebHook struct{}

func (_ FfiDestroyerOptionalWebHook) Destroy(value **WebHook) {
	if value != nil {
		FfiDestroyerWebHook{}.Destroy(*value)
	}
}

type FfiConverterOptionalLiquidBip21 struct{}

var FfiConverterOptionalLiquidBip21INSTANCE = FfiConverterOptionalLiquidBip21{}

func (c FfiConverterOptionalLiquidBip21) Lift(rb RustBufferI) *LiquidBip21 {
	return LiftFromRustBuffer[*LiquidBip21](c, rb)
}

func (_ FfiConverterOptionalLiquidBip21) Read(reader io.Reader) *LiquidBip21 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterLiquidBip21INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalLiquidBip21) Lower(value *LiquidBip21) C.RustBuffer {
	return LowerIntoRustBuffer[*LiquidBip21](c, value)
}

func (c FfiConverterOptionalLiquidBip21) LowerExternal(value *LiquidBip21) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[*LiquidBip21](c, value))
}

func (_ FfiConverterOptionalLiquidBip21) Write(writer io.Writer, value *LiquidBip21) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterLiquidBip21INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalLiquidBip21 struct{}

func (_ FfiDestroyerOptionalLiquidBip21) Destroy(value *LiquidBip21) {
	if value != nil {
		FfiDestroyerLiquidBip21{}.Destroy(*value)
	}
}

type FfiConverterOptionalSequenceTypeAssetId struct{}

var FfiConverterOptionalSequenceTypeAssetIdINSTANCE = FfiConverterOptionalSequenceTypeAssetId{}

func (c FfiConverterOptionalSequenceTypeAssetId) Lift(rb RustBufferI) *[]AssetId {
	return LiftFromRustBuffer[*[]AssetId](c, rb)
}

func (_ FfiConverterOptionalSequenceTypeAssetId) Read(reader io.Reader) *[]AssetId {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterSequenceTypeAssetIdINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSequenceTypeAssetId) Lower(value *[]AssetId) C.RustBuffer {
	return LowerIntoRustBuffer[*[]AssetId](c, value)
}

func (c FfiConverterOptionalSequenceTypeAssetId) LowerExternal(value *[]AssetId) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[*[]AssetId](c, value))
}

func (_ FfiConverterOptionalSequenceTypeAssetId) Write(writer io.Writer, value *[]AssetId) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterSequenceTypeAssetIdINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSequenceTypeAssetId struct{}

func (_ FfiDestroyerOptionalSequenceTypeAssetId) Destroy(value *[]AssetId) {
	if value != nil {
		FfiDestroyerSequenceTypeAssetId{}.Destroy(*value)
	}
}

type FfiConverterOptionalTypeAssetId struct{}

var FfiConverterOptionalTypeAssetIdINSTANCE = FfiConverterOptionalTypeAssetId{}

func (c FfiConverterOptionalTypeAssetId) Lift(rb RustBufferI) *AssetId {
	return LiftFromRustBuffer[*AssetId](c, rb)
}

func (_ FfiConverterOptionalTypeAssetId) Read(reader io.Reader) *AssetId {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterTypeAssetIdINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalTypeAssetId) Lower(value *AssetId) C.RustBuffer {
	return LowerIntoRustBuffer[*AssetId](c, value)
}

func (c FfiConverterOptionalTypeAssetId) LowerExternal(value *AssetId) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[*AssetId](c, value))
}

func (_ FfiConverterOptionalTypeAssetId) Write(writer io.Writer, value *AssetId) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterTypeAssetIdINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalTypeAssetId struct{}

func (_ FfiDestroyerOptionalTypeAssetId) Destroy(value *AssetId) {
	if value != nil {
		FfiDestroyerTypeAssetId{}.Destroy(*value)
	}
}

type FfiConverterSequenceString struct{}

var FfiConverterSequenceStringINSTANCE = FfiConverterSequenceString{}

func (c FfiConverterSequenceString) Lift(rb RustBufferI) []string {
	return LiftFromRustBuffer[[]string](c, rb)
}

func (c FfiConverterSequenceString) Read(reader io.Reader) []string {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]string, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterStringINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceString) Lower(value []string) C.RustBuffer {
	return LowerIntoRustBuffer[[]string](c, value)
}

func (c FfiConverterSequenceString) LowerExternal(value []string) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]string](c, value))
}

func (c FfiConverterSequenceString) Write(writer io.Writer, value []string) {
	if len(value) > math.MaxInt32 {
		panic("[]string is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterStringINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceString struct{}

func (FfiDestroyerSequenceString) Destroy(sequence []string) {
	for _, value := range sequence {
		FfiDestroyerString{}.Destroy(value)
	}
}

type FfiConverterSequenceExternalUtxo struct{}

var FfiConverterSequenceExternalUtxoINSTANCE = FfiConverterSequenceExternalUtxo{}

func (c FfiConverterSequenceExternalUtxo) Lift(rb RustBufferI) []*ExternalUtxo {
	return LiftFromRustBuffer[[]*ExternalUtxo](c, rb)
}

func (c FfiConverterSequenceExternalUtxo) Read(reader io.Reader) []*ExternalUtxo {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*ExternalUtxo, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterExternalUtxoINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceExternalUtxo) Lower(value []*ExternalUtxo) C.RustBuffer {
	return LowerIntoRustBuffer[[]*ExternalUtxo](c, value)
}

func (c FfiConverterSequenceExternalUtxo) LowerExternal(value []*ExternalUtxo) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]*ExternalUtxo](c, value))
}

func (c FfiConverterSequenceExternalUtxo) Write(writer io.Writer, value []*ExternalUtxo) {
	if len(value) > math.MaxInt32 {
		panic("[]*ExternalUtxo is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterExternalUtxoINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceExternalUtxo struct{}

func (FfiDestroyerSequenceExternalUtxo) Destroy(sequence []*ExternalUtxo) {
	for _, value := range sequence {
		FfiDestroyerExternalUtxo{}.Destroy(value)
	}
}

type FfiConverterSequenceIssuance struct{}

var FfiConverterSequenceIssuanceINSTANCE = FfiConverterSequenceIssuance{}

func (c FfiConverterSequenceIssuance) Lift(rb RustBufferI) []*Issuance {
	return LiftFromRustBuffer[[]*Issuance](c, rb)
}

func (c FfiConverterSequenceIssuance) Read(reader io.Reader) []*Issuance {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*Issuance, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterIssuanceINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceIssuance) Lower(value []*Issuance) C.RustBuffer {
	return LowerIntoRustBuffer[[]*Issuance](c, value)
}

func (c FfiConverterSequenceIssuance) LowerExternal(value []*Issuance) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]*Issuance](c, value))
}

func (c FfiConverterSequenceIssuance) Write(writer io.Writer, value []*Issuance) {
	if len(value) > math.MaxInt32 {
		panic("[]*Issuance is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterIssuanceINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceIssuance struct{}

func (FfiDestroyerSequenceIssuance) Destroy(sequence []*Issuance) {
	for _, value := range sequence {
		FfiDestroyerIssuance{}.Destroy(value)
	}
}

type FfiConverterSequenceOutPoint struct{}

var FfiConverterSequenceOutPointINSTANCE = FfiConverterSequenceOutPoint{}

func (c FfiConverterSequenceOutPoint) Lift(rb RustBufferI) []*OutPoint {
	return LiftFromRustBuffer[[]*OutPoint](c, rb)
}

func (c FfiConverterSequenceOutPoint) Read(reader io.Reader) []*OutPoint {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*OutPoint, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterOutPointINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceOutPoint) Lower(value []*OutPoint) C.RustBuffer {
	return LowerIntoRustBuffer[[]*OutPoint](c, value)
}

func (c FfiConverterSequenceOutPoint) LowerExternal(value []*OutPoint) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]*OutPoint](c, value))
}

func (c FfiConverterSequenceOutPoint) Write(writer io.Writer, value []*OutPoint) {
	if len(value) > math.MaxInt32 {
		panic("[]*OutPoint is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterOutPointINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceOutPoint struct{}

func (FfiDestroyerSequenceOutPoint) Destroy(sequence []*OutPoint) {
	for _, value := range sequence {
		FfiDestroyerOutPoint{}.Destroy(value)
	}
}

type FfiConverterSequencePsetInput struct{}

var FfiConverterSequencePsetInputINSTANCE = FfiConverterSequencePsetInput{}

func (c FfiConverterSequencePsetInput) Lift(rb RustBufferI) []*PsetInput {
	return LiftFromRustBuffer[[]*PsetInput](c, rb)
}

func (c FfiConverterSequencePsetInput) Read(reader io.Reader) []*PsetInput {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*PsetInput, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterPsetInputINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequencePsetInput) Lower(value []*PsetInput) C.RustBuffer {
	return LowerIntoRustBuffer[[]*PsetInput](c, value)
}

func (c FfiConverterSequencePsetInput) LowerExternal(value []*PsetInput) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]*PsetInput](c, value))
}

func (c FfiConverterSequencePsetInput) Write(writer io.Writer, value []*PsetInput) {
	if len(value) > math.MaxInt32 {
		panic("[]*PsetInput is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterPsetInputINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequencePsetInput struct{}

func (FfiDestroyerSequencePsetInput) Destroy(sequence []*PsetInput) {
	for _, value := range sequence {
		FfiDestroyerPsetInput{}.Destroy(value)
	}
}

type FfiConverterSequencePsetOutput struct{}

var FfiConverterSequencePsetOutputINSTANCE = FfiConverterSequencePsetOutput{}

func (c FfiConverterSequencePsetOutput) Lift(rb RustBufferI) []*PsetOutput {
	return LiftFromRustBuffer[[]*PsetOutput](c, rb)
}

func (c FfiConverterSequencePsetOutput) Read(reader io.Reader) []*PsetOutput {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*PsetOutput, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterPsetOutputINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequencePsetOutput) Lower(value []*PsetOutput) C.RustBuffer {
	return LowerIntoRustBuffer[[]*PsetOutput](c, value)
}

func (c FfiConverterSequencePsetOutput) LowerExternal(value []*PsetOutput) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]*PsetOutput](c, value))
}

func (c FfiConverterSequencePsetOutput) Write(writer io.Writer, value []*PsetOutput) {
	if len(value) > math.MaxInt32 {
		panic("[]*PsetOutput is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterPsetOutputINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequencePsetOutput struct{}

func (FfiDestroyerSequencePsetOutput) Destroy(sequence []*PsetOutput) {
	for _, value := range sequence {
		FfiDestroyerPsetOutput{}.Destroy(value)
	}
}

type FfiConverterSequencePsetSignatures struct{}

var FfiConverterSequencePsetSignaturesINSTANCE = FfiConverterSequencePsetSignatures{}

func (c FfiConverterSequencePsetSignatures) Lift(rb RustBufferI) []*PsetSignatures {
	return LiftFromRustBuffer[[]*PsetSignatures](c, rb)
}

func (c FfiConverterSequencePsetSignatures) Read(reader io.Reader) []*PsetSignatures {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*PsetSignatures, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterPsetSignaturesINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequencePsetSignatures) Lower(value []*PsetSignatures) C.RustBuffer {
	return LowerIntoRustBuffer[[]*PsetSignatures](c, value)
}

func (c FfiConverterSequencePsetSignatures) LowerExternal(value []*PsetSignatures) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]*PsetSignatures](c, value))
}

func (c FfiConverterSequencePsetSignatures) Write(writer io.Writer, value []*PsetSignatures) {
	if len(value) > math.MaxInt32 {
		panic("[]*PsetSignatures is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterPsetSignaturesINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequencePsetSignatures struct{}

func (FfiDestroyerSequencePsetSignatures) Destroy(sequence []*PsetSignatures) {
	for _, value := range sequence {
		FfiDestroyerPsetSignatures{}.Destroy(value)
	}
}

type FfiConverterSequenceRecipient struct{}

var FfiConverterSequenceRecipientINSTANCE = FfiConverterSequenceRecipient{}

func (c FfiConverterSequenceRecipient) Lift(rb RustBufferI) []*Recipient {
	return LiftFromRustBuffer[[]*Recipient](c, rb)
}

func (c FfiConverterSequenceRecipient) Read(reader io.Reader) []*Recipient {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*Recipient, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterRecipientINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceRecipient) Lower(value []*Recipient) C.RustBuffer {
	return LowerIntoRustBuffer[[]*Recipient](c, value)
}

func (c FfiConverterSequenceRecipient) LowerExternal(value []*Recipient) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]*Recipient](c, value))
}

func (c FfiConverterSequenceRecipient) Write(writer io.Writer, value []*Recipient) {
	if len(value) > math.MaxInt32 {
		panic("[]*Recipient is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterRecipientINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceRecipient struct{}

func (FfiDestroyerSequenceRecipient) Destroy(sequence []*Recipient) {
	for _, value := range sequence {
		FfiDestroyerRecipient{}.Destroy(value)
	}
}

type FfiConverterSequenceTxIn struct{}

var FfiConverterSequenceTxInINSTANCE = FfiConverterSequenceTxIn{}

func (c FfiConverterSequenceTxIn) Lift(rb RustBufferI) []*TxIn {
	return LiftFromRustBuffer[[]*TxIn](c, rb)
}

func (c FfiConverterSequenceTxIn) Read(reader io.Reader) []*TxIn {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*TxIn, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTxInINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTxIn) Lower(value []*TxIn) C.RustBuffer {
	return LowerIntoRustBuffer[[]*TxIn](c, value)
}

func (c FfiConverterSequenceTxIn) LowerExternal(value []*TxIn) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]*TxIn](c, value))
}

func (c FfiConverterSequenceTxIn) Write(writer io.Writer, value []*TxIn) {
	if len(value) > math.MaxInt32 {
		panic("[]*TxIn is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTxInINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTxIn struct{}

func (FfiDestroyerSequenceTxIn) Destroy(sequence []*TxIn) {
	for _, value := range sequence {
		FfiDestroyerTxIn{}.Destroy(value)
	}
}

type FfiConverterSequenceTxOut struct{}

var FfiConverterSequenceTxOutINSTANCE = FfiConverterSequenceTxOut{}

func (c FfiConverterSequenceTxOut) Lift(rb RustBufferI) []*TxOut {
	return LiftFromRustBuffer[[]*TxOut](c, rb)
}

func (c FfiConverterSequenceTxOut) Read(reader io.Reader) []*TxOut {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*TxOut, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTxOutINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTxOut) Lower(value []*TxOut) C.RustBuffer {
	return LowerIntoRustBuffer[[]*TxOut](c, value)
}

func (c FfiConverterSequenceTxOut) LowerExternal(value []*TxOut) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]*TxOut](c, value))
}

func (c FfiConverterSequenceTxOut) Write(writer io.Writer, value []*TxOut) {
	if len(value) > math.MaxInt32 {
		panic("[]*TxOut is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTxOutINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTxOut struct{}

func (FfiDestroyerSequenceTxOut) Destroy(sequence []*TxOut) {
	for _, value := range sequence {
		FfiDestroyerTxOut{}.Destroy(value)
	}
}

type FfiConverterSequenceValidatedLiquidexProposal struct{}

var FfiConverterSequenceValidatedLiquidexProposalINSTANCE = FfiConverterSequenceValidatedLiquidexProposal{}

func (c FfiConverterSequenceValidatedLiquidexProposal) Lift(rb RustBufferI) []*ValidatedLiquidexProposal {
	return LiftFromRustBuffer[[]*ValidatedLiquidexProposal](c, rb)
}

func (c FfiConverterSequenceValidatedLiquidexProposal) Read(reader io.Reader) []*ValidatedLiquidexProposal {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*ValidatedLiquidexProposal, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterValidatedLiquidexProposalINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceValidatedLiquidexProposal) Lower(value []*ValidatedLiquidexProposal) C.RustBuffer {
	return LowerIntoRustBuffer[[]*ValidatedLiquidexProposal](c, value)
}

func (c FfiConverterSequenceValidatedLiquidexProposal) LowerExternal(value []*ValidatedLiquidexProposal) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]*ValidatedLiquidexProposal](c, value))
}

func (c FfiConverterSequenceValidatedLiquidexProposal) Write(writer io.Writer, value []*ValidatedLiquidexProposal) {
	if len(value) > math.MaxInt32 {
		panic("[]*ValidatedLiquidexProposal is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterValidatedLiquidexProposalINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceValidatedLiquidexProposal struct{}

func (FfiDestroyerSequenceValidatedLiquidexProposal) Destroy(sequence []*ValidatedLiquidexProposal) {
	for _, value := range sequence {
		FfiDestroyerValidatedLiquidexProposal{}.Destroy(value)
	}
}

type FfiConverterSequenceWalletTx struct{}

var FfiConverterSequenceWalletTxINSTANCE = FfiConverterSequenceWalletTx{}

func (c FfiConverterSequenceWalletTx) Lift(rb RustBufferI) []*WalletTx {
	return LiftFromRustBuffer[[]*WalletTx](c, rb)
}

func (c FfiConverterSequenceWalletTx) Read(reader io.Reader) []*WalletTx {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*WalletTx, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterWalletTxINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceWalletTx) Lower(value []*WalletTx) C.RustBuffer {
	return LowerIntoRustBuffer[[]*WalletTx](c, value)
}

func (c FfiConverterSequenceWalletTx) LowerExternal(value []*WalletTx) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]*WalletTx](c, value))
}

func (c FfiConverterSequenceWalletTx) Write(writer io.Writer, value []*WalletTx) {
	if len(value) > math.MaxInt32 {
		panic("[]*WalletTx is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterWalletTxINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceWalletTx struct{}

func (FfiDestroyerSequenceWalletTx) Destroy(sequence []*WalletTx) {
	for _, value := range sequence {
		FfiDestroyerWalletTx{}.Destroy(value)
	}
}

type FfiConverterSequenceWalletTxOut struct{}

var FfiConverterSequenceWalletTxOutINSTANCE = FfiConverterSequenceWalletTxOut{}

func (c FfiConverterSequenceWalletTxOut) Lift(rb RustBufferI) []*WalletTxOut {
	return LiftFromRustBuffer[[]*WalletTxOut](c, rb)
}

func (c FfiConverterSequenceWalletTxOut) Read(reader io.Reader) []*WalletTxOut {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*WalletTxOut, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterWalletTxOutINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceWalletTxOut) Lower(value []*WalletTxOut) C.RustBuffer {
	return LowerIntoRustBuffer[[]*WalletTxOut](c, value)
}

func (c FfiConverterSequenceWalletTxOut) LowerExternal(value []*WalletTxOut) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]*WalletTxOut](c, value))
}

func (c FfiConverterSequenceWalletTxOut) Write(writer io.Writer, value []*WalletTxOut) {
	if len(value) > math.MaxInt32 {
		panic("[]*WalletTxOut is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterWalletTxOutINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceWalletTxOut struct{}

func (FfiDestroyerSequenceWalletTxOut) Destroy(sequence []*WalletTxOut) {
	for _, value := range sequence {
		FfiDestroyerWalletTxOut{}.Destroy(value)
	}
}

type FfiConverterSequenceOptionalWalletTxOut struct{}

var FfiConverterSequenceOptionalWalletTxOutINSTANCE = FfiConverterSequenceOptionalWalletTxOut{}

func (c FfiConverterSequenceOptionalWalletTxOut) Lift(rb RustBufferI) []**WalletTxOut {
	return LiftFromRustBuffer[[]**WalletTxOut](c, rb)
}

func (c FfiConverterSequenceOptionalWalletTxOut) Read(reader io.Reader) []**WalletTxOut {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]**WalletTxOut, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterOptionalWalletTxOutINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceOptionalWalletTxOut) Lower(value []**WalletTxOut) C.RustBuffer {
	return LowerIntoRustBuffer[[]**WalletTxOut](c, value)
}

func (c FfiConverterSequenceOptionalWalletTxOut) LowerExternal(value []**WalletTxOut) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]**WalletTxOut](c, value))
}

func (c FfiConverterSequenceOptionalWalletTxOut) Write(writer io.Writer, value []**WalletTxOut) {
	if len(value) > math.MaxInt32 {
		panic("[]**WalletTxOut is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterOptionalWalletTxOutINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceOptionalWalletTxOut struct{}

func (FfiDestroyerSequenceOptionalWalletTxOut) Destroy(sequence []**WalletTxOut) {
	for _, value := range sequence {
		FfiDestroyerOptionalWalletTxOut{}.Destroy(value)
	}
}

type FfiConverterSequenceTypeAssetId struct{}

var FfiConverterSequenceTypeAssetIdINSTANCE = FfiConverterSequenceTypeAssetId{}

func (c FfiConverterSequenceTypeAssetId) Lift(rb RustBufferI) []AssetId {
	return LiftFromRustBuffer[[]AssetId](c, rb)
}

func (c FfiConverterSequenceTypeAssetId) Read(reader io.Reader) []AssetId {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]AssetId, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTypeAssetIdINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTypeAssetId) Lower(value []AssetId) C.RustBuffer {
	return LowerIntoRustBuffer[[]AssetId](c, value)
}

func (c FfiConverterSequenceTypeAssetId) LowerExternal(value []AssetId) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[[]AssetId](c, value))
}

func (c FfiConverterSequenceTypeAssetId) Write(writer io.Writer, value []AssetId) {
	if len(value) > math.MaxInt32 {
		panic("[]AssetId is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTypeAssetIdINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTypeAssetId struct{}

func (FfiDestroyerSequenceTypeAssetId) Destroy(sequence []AssetId) {
	for _, value := range sequence {
		FfiDestroyerTypeAssetId{}.Destroy(value)
	}
}

type FfiConverterMapStringString struct{}

var FfiConverterMapStringStringINSTANCE = FfiConverterMapStringString{}

func (c FfiConverterMapStringString) Lift(rb RustBufferI) map[string]string {
	return LiftFromRustBuffer[map[string]string](c, rb)
}

func (_ FfiConverterMapStringString) Read(reader io.Reader) map[string]string {
	result := make(map[string]string)
	length := readInt32(reader)
	for i := int32(0); i < length; i++ {
		key := FfiConverterStringINSTANCE.Read(reader)
		value := FfiConverterStringINSTANCE.Read(reader)
		result[key] = value
	}
	return result
}

func (c FfiConverterMapStringString) Lower(value map[string]string) C.RustBuffer {
	return LowerIntoRustBuffer[map[string]string](c, value)
}

func (c FfiConverterMapStringString) LowerExternal(value map[string]string) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[map[string]string](c, value))
}

func (_ FfiConverterMapStringString) Write(writer io.Writer, mapValue map[string]string) {
	if len(mapValue) > math.MaxInt32 {
		panic("map[string]string is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(mapValue)))
	for key, value := range mapValue {
		FfiConverterStringINSTANCE.Write(writer, key)
		FfiConverterStringINSTANCE.Write(writer, value)
	}
}

type FfiDestroyerMapStringString struct{}

func (_ FfiDestroyerMapStringString) Destroy(mapValue map[string]string) {
	for key, value := range mapValue {
		FfiDestroyerString{}.Destroy(key)
		FfiDestroyerString{}.Destroy(value)
	}
}

type FfiConverterMapTypeAssetIdUint64 struct{}

var FfiConverterMapTypeAssetIdUint64INSTANCE = FfiConverterMapTypeAssetIdUint64{}

func (c FfiConverterMapTypeAssetIdUint64) Lift(rb RustBufferI) map[AssetId]uint64 {
	return LiftFromRustBuffer[map[AssetId]uint64](c, rb)
}

func (_ FfiConverterMapTypeAssetIdUint64) Read(reader io.Reader) map[AssetId]uint64 {
	result := make(map[AssetId]uint64)
	length := readInt32(reader)
	for i := int32(0); i < length; i++ {
		key := FfiConverterTypeAssetIdINSTANCE.Read(reader)
		value := FfiConverterUint64INSTANCE.Read(reader)
		result[key] = value
	}
	return result
}

func (c FfiConverterMapTypeAssetIdUint64) Lower(value map[AssetId]uint64) C.RustBuffer {
	return LowerIntoRustBuffer[map[AssetId]uint64](c, value)
}

func (c FfiConverterMapTypeAssetIdUint64) LowerExternal(value map[AssetId]uint64) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[map[AssetId]uint64](c, value))
}

func (_ FfiConverterMapTypeAssetIdUint64) Write(writer io.Writer, mapValue map[AssetId]uint64) {
	if len(mapValue) > math.MaxInt32 {
		panic("map[AssetId]uint64 is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(mapValue)))
	for key, value := range mapValue {
		FfiConverterTypeAssetIdINSTANCE.Write(writer, key)
		FfiConverterUint64INSTANCE.Write(writer, value)
	}
}

type FfiDestroyerMapTypeAssetIdUint64 struct{}

func (_ FfiDestroyerMapTypeAssetIdUint64) Destroy(mapValue map[AssetId]uint64) {
	for key, value := range mapValue {
		FfiDestroyerTypeAssetId{}.Destroy(key)
		FfiDestroyerUint64{}.Destroy(value)
	}
}

type FfiConverterMapTypeAssetIdInt64 struct{}

var FfiConverterMapTypeAssetIdInt64INSTANCE = FfiConverterMapTypeAssetIdInt64{}

func (c FfiConverterMapTypeAssetIdInt64) Lift(rb RustBufferI) map[AssetId]int64 {
	return LiftFromRustBuffer[map[AssetId]int64](c, rb)
}

func (_ FfiConverterMapTypeAssetIdInt64) Read(reader io.Reader) map[AssetId]int64 {
	result := make(map[AssetId]int64)
	length := readInt32(reader)
	for i := int32(0); i < length; i++ {
		key := FfiConverterTypeAssetIdINSTANCE.Read(reader)
		value := FfiConverterInt64INSTANCE.Read(reader)
		result[key] = value
	}
	return result
}

func (c FfiConverterMapTypeAssetIdInt64) Lower(value map[AssetId]int64) C.RustBuffer {
	return LowerIntoRustBuffer[map[AssetId]int64](c, value)
}

func (c FfiConverterMapTypeAssetIdInt64) LowerExternal(value map[AssetId]int64) ExternalCRustBuffer {
	return RustBufferFromC(LowerIntoRustBuffer[map[AssetId]int64](c, value))
}

func (_ FfiConverterMapTypeAssetIdInt64) Write(writer io.Writer, mapValue map[AssetId]int64) {
	if len(mapValue) > math.MaxInt32 {
		panic("map[AssetId]int64 is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(mapValue)))
	for key, value := range mapValue {
		FfiConverterTypeAssetIdINSTANCE.Write(writer, key)
		FfiConverterInt64INSTANCE.Write(writer, value)
	}
}

type FfiDestroyerMapTypeAssetIdInt64 struct{}

func (_ FfiDestroyerMapTypeAssetIdInt64) Destroy(mapValue map[AssetId]int64) {
	for key, value := range mapValue {
		FfiDestroyerTypeAssetId{}.Destroy(key)
		FfiDestroyerInt64{}.Destroy(value)
	}
}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type AssetId = string
type FfiConverterTypeAssetId = FfiConverterString
type FfiDestroyerTypeAssetId = FfiDestroyerString

var FfiConverterTypeAssetIdINSTANCE = FfiConverterString{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type Hex = string
type FfiConverterTypeHex = FfiConverterString
type FfiDestroyerTypeHex = FfiDestroyerString

var FfiConverterTypeHexINSTANCE = FfiConverterString{}

// Derive asset id from contract and transaction input
func DeriveAssetId(txin *TxIn, contract *Contract) (AssetId, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_func_derive_asset_id(FfiConverterTxInINSTANCE.Lower(txin), FfiConverterContractINSTANCE.Lower(contract), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue AssetId
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTypeAssetIdINSTANCE.Lift(_uniffiRV), nil
	}
}

// Derive token id from contract and transaction input
func DeriveTokenId(txin *TxIn, contract *Contract) (AssetId, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_func_derive_token_id(FfiConverterTxInINSTANCE.Lower(txin), FfiConverterContractINSTANCE.Lower(contract), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue AssetId
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTypeAssetIdINSTANCE.Lift(_uniffiRV), nil
	}
}

// Whether a script pubkey is provably segwit
func IsProvablySegwit(scriptPubkey *Script, redeemScript **Script) bool {
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_func_is_provably_segwit(FfiConverterScriptINSTANCE.Lower(scriptPubkey), FfiConverterOptionalScriptINSTANCE.Lower(redeemScript), _uniffiStatus)
	}))
}
