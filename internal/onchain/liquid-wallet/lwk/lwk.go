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

	FfiConverterForeignPersisterINSTANCE.register()
	uniffiCheckChecksums()
}

func uniffiCheckChecksums() {
	// Get the bindings contract version from our ComponentInterface
	bindingsContractVersion := 26
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
			return C.uniffi_lwk_checksum_func_is_provably_segwit()
		})
		if checksum != 25275 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_func_is_provably_segwit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_address_is_blinded()
		})
		if checksum != 34440 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_address_is_blinded: UniFFI API checksum mismatch")
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
		if checksum != 23569 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_address_script_pubkey: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_address_to_unconfidential()
		})
		if checksum != 28990 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_address_to_unconfidential: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_addressresult_address()
		})
		if checksum != 57079 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_addressresult_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_addressresult_index()
		})
		if checksum != 6170 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_addressresult_index: UniFFI API checksum mismatch")
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
			return C.uniffi_lwk_checksum_method_amp2_register()
		})
		if checksum != 53300 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_amp2_register: UniFFI API checksum mismatch")
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
		if checksum != 1080 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_assetamount_amount: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_assetamount_asset()
		})
		if checksum != 31724 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_assetamount_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_electrumclient_broadcast()
		})
		if checksum != 41537 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_electrumclient_broadcast: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_electrumclient_full_scan()
		})
		if checksum != 5919 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_electrumclient_full_scan: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_electrumclient_full_scan_to_index()
		})
		if checksum != 64210 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_electrumclient_full_scan_to_index: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_electrumclient_get_tx()
		})
		if checksum != 11605 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_electrumclient_get_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_electrumclient_ping()
		})
		if checksum != 49466 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_electrumclient_ping: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_esploraclient_broadcast()
		})
		if checksum != 54439 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_esploraclient_broadcast: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_esploraclient_full_scan()
		})
		if checksum != 27446 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_esploraclient_full_scan: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_esploraclient_full_scan_to_index()
		})
		if checksum != 37814 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_esploraclient_full_scan_to_index: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_foreignpersister_get()
		})
		if checksum != 54855 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_foreignpersister_get: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_foreignpersister_push()
		})
		if checksum != 22972 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_foreignpersister_push: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_asset()
		})
		if checksum != 59545 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_asset_satoshi()
		})
		if checksum != 13924 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_asset_satoshi: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_is_confidential()
		})
		if checksum != 28108 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_is_confidential: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_is_issuance()
		})
		if checksum != 36847 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_is_issuance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_is_null()
		})
		if checksum != 41097 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_is_null: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_is_reissuance()
		})
		if checksum != 19752 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_is_reissuance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_prev_txid()
		})
		if checksum != 29158 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_prev_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_prev_vout()
		})
		if checksum != 47940 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_prev_vout: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_token()
		})
		if checksum != 31197 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_token: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_issuance_token_satoshi()
		})
		if checksum != 10642 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_issuance_token_satoshi: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_electrum_url()
		})
		if checksum != 55646 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_electrum_url: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_generate()
		})
		if checksum != 26765 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_generate: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_get_new_address()
		})
		if checksum != 19321 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_get_new_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_height()
		})
		if checksum != 2430 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_height: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_issue_asset()
		})
		if checksum != 1145 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_issue_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_lwktestenv_send_to_address()
		})
		if checksum != 56643 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_lwktestenv_send_to_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_network_default_electrum_client()
		})
		if checksum != 57493 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_network_default_electrum_client: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_network_default_esplora_client()
		})
		if checksum != 7540 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_network_default_esplora_client: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_network_is_mainnet()
		})
		if checksum != 38901 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_network_is_mainnet: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_network_policy_asset()
		})
		if checksum != 61043 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_network_policy_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_network_tx_builder()
		})
		if checksum != 62021 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_network_tx_builder: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_outpoint_txid()
		})
		if checksum != 59660 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_outpoint_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_outpoint_vout()
		})
		if checksum != 56493 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_outpoint_vout: UniFFI API checksum mismatch")
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
			return C.uniffi_lwk_checksum_method_pset_combine()
		})
		if checksum != 29157 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_pset_combine: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_pset_extract_tx()
		})
		if checksum != 24108 {
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
		if checksum != 59953 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_pset_inputs: UniFFI API checksum mismatch")
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
		if checksum != 56410 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetdetails_balance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetdetails_inputs_issuances()
		})
		if checksum != 57319 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetdetails_inputs_issuances: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetdetails_signatures()
		})
		if checksum != 49463 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetdetails_signatures: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_issuance()
		})
		if checksum != 24131 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_issuance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_issuance_asset()
		})
		if checksum != 63028 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_issuance_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_issuance_token()
		})
		if checksum != 28592 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_issuance_token: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_previous_script_pubkey()
		})
		if checksum != 29126 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_previous_script_pubkey: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_previous_txid()
		})
		if checksum != 21436 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_previous_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_previous_vout()
		})
		if checksum != 7375 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_previous_vout: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_psetinput_redeem_script()
		})
		if checksum != 44187 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_psetinput_redeem_script: UniFFI API checksum mismatch")
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
			return C.uniffi_lwk_checksum_method_script_asm()
		})
		if checksum != 38627 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_script_asm: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_script_bytes()
		})
		if checksum != 31898 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_script_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_secretkey_bytes()
		})
		if checksum != 44270 {
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
			return C.uniffi_lwk_checksum_method_signer_keyorigin_xpub()
		})
		if checksum != 15198 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_signer_keyorigin_xpub: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_signer_mnemonic()
		})
		if checksum != 29480 {
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
		if checksum != 28847 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_signer_singlesig_desc: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_signer_wpkh_slip77_descriptor()
		})
		if checksum != 55215 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_signer_wpkh_slip77_descriptor: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_transaction_bytes()
		})
		if checksum != 35387 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_transaction_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_transaction_fee()
		})
		if checksum != 42284 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_transaction_fee: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_transaction_inputs()
		})
		if checksum != 51474 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_transaction_inputs: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_transaction_outputs()
		})
		if checksum != 59927 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_transaction_outputs: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_transaction_txid()
		})
		if checksum != 8927 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_transaction_txid: UniFFI API checksum mismatch")
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
			return C.uniffi_lwk_checksum_method_txbuilder_add_external_utxos()
		})
		if checksum != 29722 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_add_external_utxos: UniFFI API checksum mismatch")
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
			return C.uniffi_lwk_checksum_method_txbuilder_issue_asset()
		})
		if checksum != 32494 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_issue_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_liquidex_make()
		})
		if checksum != 47954 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_liquidex_make: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_liquidex_take()
		})
		if checksum != 14367 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_liquidex_take: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_reissue_asset()
		})
		if checksum != 54385 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txbuilder_reissue_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txbuilder_set_wallet_utxos()
		})
		if checksum != 12661 {
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
		if checksum != 21742 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txoutsecrets_asset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txoutsecrets_asset_bf()
		})
		if checksum != 27606 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txoutsecrets_asset_bf: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txoutsecrets_value()
		})
		if checksum != 64117 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txoutsecrets_value: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txoutsecrets_value_bf()
		})
		if checksum != 4095 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txoutsecrets_value_bf: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_txid_bytes()
		})
		if checksum != 15950 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_txid_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_unvalidatedliquidexproposal_insecure_validate()
		})
		if checksum != 63611 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_unvalidatedliquidexproposal_insecure_validate: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_unvalidatedliquidexproposal_needed_tx()
		})
		if checksum != 61339 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_unvalidatedliquidexproposal_needed_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_unvalidatedliquidexproposal_validate()
		})
		if checksum != 39721 {
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
		if checksum != 9990 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_update_serialize: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_validatedliquidexproposal_input()
		})
		if checksum != 49227 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_validatedliquidexproposal_input: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_validatedliquidexproposal_output()
		})
		if checksum != 43380 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_validatedliquidexproposal_output: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_balance()
		})
		if checksum != 44398 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_balance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_fee()
		})
		if checksum != 39011 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_fee: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_height()
		})
		if checksum != 12656 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_height: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_inputs()
		})
		if checksum != 3951 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_inputs: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_outputs()
		})
		if checksum != 55588 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_outputs: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_timestamp()
		})
		if checksum != 12633 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_timestamp: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_tx()
		})
		if checksum != 23689 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_tx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_txid()
		})
		if checksum != 36652 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_type_()
		})
		if checksum != 59416 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_type_: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettx_unblinded_url()
		})
		if checksum != 45766 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettx_unblinded_url: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettxout_address()
		})
		if checksum != 55633 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettxout_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettxout_ext_int()
		})
		if checksum != 60402 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettxout_ext_int: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettxout_height()
		})
		if checksum != 50237 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettxout_height: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettxout_outpoint()
		})
		if checksum != 58785 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettxout_outpoint: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettxout_script_pubkey()
		})
		if checksum != 50610 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettxout_script_pubkey: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettxout_unblinded()
		})
		if checksum != 57421 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettxout_unblinded: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wallettxout_wildcard_index()
		})
		if checksum != 49286 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wallettxout_wildcard_index: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_address()
		})
		if checksum != 14903 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_apply_update()
		})
		if checksum != 55233 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_apply_update: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_balance()
		})
		if checksum != 6265 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_balance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_descriptor()
		})
		if checksum != 25068 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_descriptor: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_finalize()
		})
		if checksum != 63816 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_finalize: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_pset_details()
		})
		if checksum != 45882 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_pset_details: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_transactions()
		})
		if checksum != 35692 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wollet_transactions: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wollet_transactions_paginated()
		})
		if checksum != 32144 {
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
			return C.uniffi_lwk_checksum_method_wolletdescriptor_derive_blinding_key()
		})
		if checksum != 27121 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_method_wolletdescriptor_derive_blinding_key: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_method_wolletdescriptor_is_mainnet()
		})
		if checksum != 62487 {
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
			return C.uniffi_lwk_checksum_constructor_address_new()
		})
		if checksum != 52129 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_address_new: UniFFI API checksum mismatch")
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
			return C.uniffi_lwk_checksum_constructor_contract_new()
		})
		if checksum != 55905 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_contract_new: UniFFI API checksum mismatch")
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
			return C.uniffi_lwk_checksum_constructor_foreignpersisterlink_new()
		})
		if checksum != 13549 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_foreignpersisterlink_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_lwktestenv_new()
		})
		if checksum != 23847 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_lwktestenv_new: UniFFI API checksum mismatch")
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
		if checksum != 55931 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_network_mainnet: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_network_regtest()
		})
		if checksum != 26689 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_network_regtest: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_network_regtest_default()
		})
		if checksum != 53192 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_network_regtest_default: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_network_testnet()
		})
		if checksum != 37103 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_network_testnet: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_outpoint_new()
		})
		if checksum != 61639 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_outpoint_new: UniFFI API checksum mismatch")
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
			return C.uniffi_lwk_checksum_constructor_script_new()
		})
		if checksum != 43814 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_script_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_secretkey_from_bytes()
		})
		if checksum != 14021 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_secretkey_from_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_secretkey_from_wif()
		})
		if checksum != 46565 {
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
			return C.uniffi_lwk_checksum_constructor_transaction_new()
		})
		if checksum != 3065 {
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
		if checksum != 42031 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_unvalidatedliquidexproposal_from_pset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_unvalidatedliquidexproposal_new()
		})
		if checksum != 8682 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_unvalidatedliquidexproposal_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_update_new()
		})
		if checksum != 35370 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_update_new: UniFFI API checksum mismatch")
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
			return C.uniffi_lwk_checksum_constructor_wollet_with_custom_persister()
		})
		if checksum != 63220 {
			// If this happens try cleaning and rebuilding your project
			panic("lwk: uniffi_lwk_checksum_constructor_wollet_with_custom_persister: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_lwk_checksum_constructor_wolletdescriptor_new()
		})
		if checksum != 57700 {
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

var FfiCounter atomic.Int64

func newFfiObject(
	pointer unsafe.Pointer,
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer,
	freeFunction func(unsafe.Pointer, *C.RustCallStatus),
) FfiObject {
	FfiCounter.Add(1)
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
		FfiCounter.Add(-1)
		ffiObject.freeFunction(ffiObject.pointer, status)
		return 0
	})
}

type AddressInterface interface {
	IsBlinded() bool
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
	ScriptPubkey() *Script
	ToUnconfidential() *Address
}
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

func (_self *Address) IsBlinded() bool {
	_pointer := _self.ffiObject.incrementPointer("*Address")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_address_is_blinded(
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

func (_self *Address) ScriptPubkey() *Script {
	_pointer := _self.ffiObject.incrementPointer("*Address")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterScriptINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_address_script_pubkey(
			_pointer, _uniffiStatus)
	}))
}

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

type AddressResultInterface interface {
	Address() *Address
	Index() uint32
}
type AddressResult struct {
	ffiObject FfiObject
}

func (_self *AddressResult) Address() *Address {
	_pointer := _self.ffiObject.incrementPointer("*AddressResult")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterAddressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_addressresult_address(
			_pointer, _uniffiStatus)
	}))
}

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

// Wrapper over [`lwk_wollet::amp2::Amp2`]
type Amp2Interface interface {
	// Ask the AMP2 server to cosign a PSET
	Cosign(pset *Pset) (*Pset, error)
	// Create an AMP2 wallet descriptor from the keyorigin xpub of a signer
	DescriptorFromStr(keyoriginXpub string) (*Amp2Descriptor, error)
	// Register an AMP2 wallet with the AMP2 server
	Register(desc *Amp2Descriptor) (string, error)
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
func (_self *Amp2) Register(desc *Amp2Descriptor) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Amp2")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_amp2_register(
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

// Wrapper over [`lwk_wollet::AssetAmount`]
type AssetAmountInterface interface {
	Amount() uint64
	Asset() AssetId
}

// Wrapper over [`lwk_wollet::AssetAmount`]
type AssetAmount struct {
	ffiObject FfiObject
}

func (_self *AssetAmount) Amount() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*AssetAmount")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_assetamount_amount(
			_pointer, _uniffiStatus)
	}))
}

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

// Wrapper over [`lwk_wollet::ElectrumClient`]
type ElectrumClientInterface interface {
	Broadcast(tx *Transaction) (*Txid, error)
	FullScan(wollet *Wollet) (**Update, error)
	FullScanToIndex(wollet *Wollet, index uint32) (**Update, error)
	GetTx(txid *Txid) (*Transaction, error)
	Ping() error
}

// Wrapper over [`lwk_wollet::ElectrumClient`]
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

// Wrapper over [`blocking::EsploraClient`]
type EsploraClientInterface interface {
	Broadcast(tx *Transaction) (*Txid, error)
	// See [`BlockchainBackend::full_scan`]
	FullScan(wollet *Wollet) (**Update, error)
	// See [`BlockchainBackend::full_scan_to_index`]
	FullScanToIndex(wollet *Wollet, index uint32) (**Update, error)
}

// Wrapper over [`blocking::EsploraClient`]
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

// See [`BlockchainBackend::full_scan`]
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

// See [`BlockchainBackend::full_scan_to_index`]
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

type ExternalUtxoInterface interface {
}
type ExternalUtxo struct {
	ffiObject FfiObject
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

// An exported trait, useful for caller-defined persistence.
type ForeignPersister interface {
	Get(index uint64) (**Update, error)
	Push(update *Update) error
}

// An exported trait, useful for caller-defined persistence.
type ForeignPersisterImpl struct {
	ffiObject FfiObject
}

func (_self *ForeignPersisterImpl) Get(index uint64) (**Update, error) {
	_pointer := _self.ffiObject.incrementPointer("ForeignPersister")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_foreignpersister_get(
				_pointer, FfiConverterUint64INSTANCE.Lower(index), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue **Update
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalUpdateINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *ForeignPersisterImpl) Push(update *Update) error {
	_pointer := _self.ffiObject.incrementPointer("ForeignPersister")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_foreignpersister_push(
			_pointer, FfiConverterUpdateINSTANCE.Lower(update), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}
func (object *ForeignPersisterImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterForeignPersister struct {
	handleMap *concurrentHandleMap[ForeignPersister]
}

var FfiConverterForeignPersisterINSTANCE = FfiConverterForeignPersister{
	handleMap: newConcurrentHandleMap[ForeignPersister](),
}

func (c FfiConverterForeignPersister) Lift(pointer unsafe.Pointer) ForeignPersister {
	result := &ForeignPersisterImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_foreignpersister(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_foreignpersister(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ForeignPersisterImpl).Destroy)
	return result
}

func (c FfiConverterForeignPersister) Read(reader io.Reader) ForeignPersister {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterForeignPersister) Lower(value ForeignPersister) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterForeignPersister) Write(writer io.Writer, value ForeignPersister) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerForeignPersister struct{}

func (_ FfiDestroyerForeignPersister) Destroy(value ForeignPersister) {
	if val, ok := value.(*ForeignPersisterImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *ForeignPersisterImpl")
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

//export lwk_cgo_dispatchCallbackInterfaceForeignPersisterMethod0
func lwk_cgo_dispatchCallbackInterfaceForeignPersisterMethod0(uniffiHandle C.uint64_t, index C.uint64_t, uniffiOutReturn *C.RustBuffer, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterForeignPersisterINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	res, err :=
		uniffiObj.Get(
			FfiConverterUint64INSTANCE.Lift(index),
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

	*uniffiOutReturn = FfiConverterOptionalUpdateINSTANCE.Lower(res)
}

//export lwk_cgo_dispatchCallbackInterfaceForeignPersisterMethod1
func lwk_cgo_dispatchCallbackInterfaceForeignPersisterMethod1(uniffiHandle C.uint64_t, update unsafe.Pointer, uniffiOutReturn *C.void, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterForeignPersisterINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	err :=
		uniffiObj.Push(
			FfiConverterUpdateINSTANCE.Lift(update),
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

var UniffiVTableCallbackInterfaceForeignPersisterINSTANCE = C.UniffiVTableCallbackInterfaceForeignPersister{
	get:  (C.UniffiCallbackInterfaceForeignPersisterMethod0)(C.lwk_cgo_dispatchCallbackInterfaceForeignPersisterMethod0),
	push: (C.UniffiCallbackInterfaceForeignPersisterMethod1)(C.lwk_cgo_dispatchCallbackInterfaceForeignPersisterMethod1),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.lwk_cgo_dispatchCallbackInterfaceForeignPersisterFree),
}

//export lwk_cgo_dispatchCallbackInterfaceForeignPersisterFree
func lwk_cgo_dispatchCallbackInterfaceForeignPersisterFree(handle C.uint64_t) {
	FfiConverterForeignPersisterINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterForeignPersister) register() {
	C.uniffi_lwk_fn_init_callback_vtable_foreignpersister(&UniffiVTableCallbackInterfaceForeignPersisterINSTANCE)
}

// Implements [`ForeignPersister`]
type ForeignPersisterLinkInterface interface {
}

// Implements [`ForeignPersister`]
type ForeignPersisterLink struct {
	ffiObject FfiObject
}

func NewForeignPersisterLink(persister ForeignPersister) *ForeignPersisterLink {
	return FfiConverterForeignPersisterLinkINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_foreignpersisterlink_new(FfiConverterForeignPersisterINSTANCE.Lower(persister), _uniffiStatus)
	}))
}

func (object *ForeignPersisterLink) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterForeignPersisterLink struct{}

var FfiConverterForeignPersisterLinkINSTANCE = FfiConverterForeignPersisterLink{}

func (c FfiConverterForeignPersisterLink) Lift(pointer unsafe.Pointer) *ForeignPersisterLink {
	result := &ForeignPersisterLink{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_lwk_fn_clone_foreignpersisterlink(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_lwk_fn_free_foreignpersisterlink(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*ForeignPersisterLink).Destroy)
	return result
}

func (c FfiConverterForeignPersisterLink) Read(reader io.Reader) *ForeignPersisterLink {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterForeignPersisterLink) Lower(value *ForeignPersisterLink) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*ForeignPersisterLink")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterForeignPersisterLink) Write(writer io.Writer, value *ForeignPersisterLink) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerForeignPersisterLink struct{}

func (_ FfiDestroyerForeignPersisterLink) Destroy(value *ForeignPersisterLink) {
	value.Destroy()
}

type IssuanceInterface interface {
	Asset() *AssetId
	AssetSatoshi() *uint64
	IsConfidential() bool
	IsIssuance() bool
	IsNull() bool
	IsReissuance() bool
	PrevTxid() **Txid
	PrevVout() *uint32
	Token() *AssetId
	TokenSatoshi() *uint64
}
type Issuance struct {
	ffiObject FfiObject
}

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

func (_self *Issuance) IsConfidential() bool {
	_pointer := _self.ffiObject.incrementPointer("*Issuance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_issuance_is_confidential(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Issuance) IsIssuance() bool {
	_pointer := _self.ffiObject.incrementPointer("*Issuance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_issuance_is_issuance(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Issuance) IsNull() bool {
	_pointer := _self.ffiObject.incrementPointer("*Issuance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_issuance_is_null(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Issuance) IsReissuance() bool {
	_pointer := _self.ffiObject.incrementPointer("*Issuance")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_issuance_is_reissuance(
			_pointer, _uniffiStatus)
	}))
}

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

// Represent a test environment with an elements node and an electrum server.
// useful for testing only, wrapper over [`lwk_test_util::TestElectrumServer`]
type LwkTestEnvInterface interface {
	ElectrumUrl() string
	Generate(blocks uint32)
	GetNewAddress() *Address
	Height() uint64
	IssueAsset(satoshi uint64) AssetId
	SendToAddress(address *Address, satoshi uint64, asset *AssetId) *Txid
}

// Represent a test environment with an elements node and an electrum server.
// useful for testing only, wrapper over [`lwk_test_util::TestElectrumServer`]
type LwkTestEnv struct {
	ffiObject FfiObject
}

func NewLwkTestEnv() *LwkTestEnv {
	return FfiConverterLwkTestEnvINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_lwktestenv_new(_uniffiStatus)
	}))
}

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

func (_self *LwkTestEnv) Generate(blocks uint32) {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestEnv")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_lwk_fn_method_lwktestenv_generate(
			_pointer, FfiConverterUint32INSTANCE.Lower(blocks), _uniffiStatus)
		return false
	})
}

func (_self *LwkTestEnv) GetNewAddress() *Address {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestEnv")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterAddressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_lwktestenv_get_new_address(
			_pointer, _uniffiStatus)
	}))
}

func (_self *LwkTestEnv) Height() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestEnv")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_lwktestenv_height(
			_pointer, _uniffiStatus)
	}))
}

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

func (_self *LwkTestEnv) SendToAddress(address *Address, satoshi uint64, asset *AssetId) *Txid {
	_pointer := _self.ffiObject.incrementPointer("*LwkTestEnv")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTxidINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_lwktestenv_send_to_address(
			_pointer, FfiConverterAddressINSTANCE.Lower(address), FfiConverterUint64INSTANCE.Lower(satoshi), FfiConverterOptionalTypeAssetIdINSTANCE.Lower(asset), _uniffiStatus)
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

// Wrapper over [`bip39::Mnemonic`]
type MnemonicInterface interface {
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

// Wrapper over [`lwk_wollet::ElementsNetwork`]
type NetworkInterface interface {
	DefaultElectrumClient() (*ElectrumClient, error)
	DefaultEsploraClient() (*EsploraClient, error)
	IsMainnet() bool
	PolicyAsset() AssetId
	TxBuilder() *TxBuilder
}

// Wrapper over [`lwk_wollet::ElementsNetwork`]
type Network struct {
	ffiObject FfiObject
}

func NetworkMainnet() *Network {
	return FfiConverterNetworkINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_network_mainnet(_uniffiStatus)
	}))
}

func NetworkRegtest(policyAsset AssetId) *Network {
	return FfiConverterNetworkINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_network_regtest(FfiConverterTypeAssetIdINSTANCE.Lower(policyAsset), _uniffiStatus)
	}))
}

func NetworkRegtestDefault() *Network {
	return FfiConverterNetworkINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_network_regtest_default(_uniffiStatus)
	}))
}

func NetworkTestnet() *Network {
	return FfiConverterNetworkINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_network_testnet(_uniffiStatus)
	}))
}

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

func (_self *Network) IsMainnet() bool {
	_pointer := _self.ffiObject.incrementPointer("*Network")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_method_network_is_mainnet(
			_pointer, _uniffiStatus)
	}))
}

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

type OutPointInterface interface {
	Txid() *Txid
	Vout() uint32
}
type OutPoint struct {
	ffiObject FfiObject
}

// Construct an OutPoint object
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

func (_self *OutPoint) Txid() *Txid {
	_pointer := _self.ffiObject.incrementPointer("*OutPoint")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTxidINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_outpoint_txid(
			_pointer, _uniffiStatus)
	}))
}

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

// Partially Signed Elements Transaction, wrapper over [`elements::pset::PartiallySignedTransaction`]
type PsetInterface interface {
	Combine(other *Pset) (*Pset, error)
	ExtractTx() (*Transaction, error)
	// Finalize and extract the PSET
	Finalize() (*Transaction, error)
	Inputs() []*PsetInput
	// Get the unique id of the PSET as defined by [BIP-370](https://github.com/bitcoin/bips/blob/master/bip-0370.mediawiki#unique-identification)
	//
	// The unique id is the txid of the PSET with sequence numbers of inputs set to 0
	UniqueId() (*Txid, error)
}

// Partially Signed Elements Transaction, wrapper over [`elements::pset::PartiallySignedTransaction`]
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

type PsetDetailsInterface interface {
	Balance() *PsetBalance
	InputsIssuances() []*Issuance
	Signatures() []*PsetSignatures
}
type PsetDetails struct {
	ffiObject FfiObject
}

func (_self *PsetDetails) Balance() *PsetBalance {
	_pointer := _self.ffiObject.incrementPointer("*PsetDetails")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterPsetBalanceINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_psetdetails_balance(
			_pointer, _uniffiStatus)
	}))
}

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

// PSET input
type PsetInputInterface interface {
	// If the input has a (re)issuance, the issuance object
	Issuance() **Issuance
	// If the input has an issuance, the asset id
	IssuanceAsset() *AssetId
	// If the input has an issuance, the token id
	IssuanceToken() *AssetId
	// Prevout scriptpubkey of the input
	PreviousScriptPubkey() **Script
	// Prevout TXID of the input
	PreviousTxid() *Txid
	// Prevout vout of the input
	PreviousVout() uint32
	// Redeem script of the input
	RedeemScript() **Script
}

// PSET input
type PsetInput struct {
	ffiObject FfiObject
}

// If the input has a (re)issuance, the issuance object
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

// If the input has an issuance, the asset id
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

// If the input has an issuance, the token id
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

// Prevout scriptpubkey of the input
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

// Prevout TXID of the input
func (_self *PsetInput) PreviousTxid() *Txid {
	_pointer := _self.ffiObject.incrementPointer("*PsetInput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTxidINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_psetinput_previous_txid(
			_pointer, _uniffiStatus)
	}))
}

// Prevout vout of the input
func (_self *PsetInput) PreviousVout() uint32 {
	_pointer := _self.ffiObject.incrementPointer("*PsetInput")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint32INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.uniffi_lwk_fn_method_psetinput_previous_vout(
			_pointer, _uniffiStatus)
	}))
}

// Redeem script of the input
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

type ScriptInterface interface {
	Asm() string
	Bytes() []byte
}
type Script struct {
	ffiObject FfiObject
}

// Construct a Script object
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

func (_self *Script) Asm() string {
	_pointer := _self.ffiObject.incrementPointer("*Script")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_lwk_fn_method_script_asm(
				_pointer, _uniffiStatus),
		}
	}))
}

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
	Bytes() []byte
	// Sign the given `pset`
	Sign(pset *Pset) (*Pset, error)
}

// A secret key
type SecretKey struct {
	ffiObject FfiObject
}

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
	KeyoriginXpub(bip *Bip) (string, error)
	Mnemonic() (*Mnemonic, error)
	// Sign the given `pset`
	//
	// Note from an API perspective it would be better to consume the `pset` parameter so it would
	// be clear the signed PSET is the returned one, but it's not possible with uniffi bindings
	Sign(pset *Pset) (*Pset, error)
	SinglesigDesc(scriptVariant Singlesig, blindingVariant DescriptorBlindingKey) (*WolletDescriptor, error)
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

type TransactionInterface interface {
	Bytes() []byte
	Fee(policyAsset AssetId) uint64
	Inputs() []*TxIn
	Outputs() []*TxOut
	Txid() *Txid
}
type Transaction struct {
	ffiObject FfiObject
}

// Construct a Transaction object
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

func (_self *Transaction) Fee(policyAsset AssetId) uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Transaction")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_transaction_fee(
			_pointer, FfiConverterTypeAssetIdINSTANCE.Lower(policyAsset), _uniffiStatus)
	}))
}

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

func (_self *Transaction) Txid() *Txid {
	_pointer := _self.ffiObject.incrementPointer("*Transaction")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTxidINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_transaction_txid(
			_pointer, _uniffiStatus)
	}))
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
	// Add external utxos, wrapper of [`lwk_wollet::TxBuilder::add_external_utxos()`]
	AddExternalUtxos(utxos []*ExternalUtxo) error
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
	// Issue an asset, wrapper of [`lwk_wollet::TxBuilder::issue_asset()`]
	IssueAsset(assetSats uint64, assetReceiver **Address, tokenSats uint64, tokenReceiver **Address, contract **Contract) error
	LiquidexMake(utxo *OutPoint, address *Address, amount uint64, asset AssetId) error
	LiquidexTake(proposals []*ValidatedLiquidexProposal) error
	// Reissue an asset, wrapper of [`lwk_wollet::TxBuilder::reissue_asset()`]
	ReissueAsset(assetToReissue AssetId, satoshiToReissue uint64, assetReceiver **Address, issuanceTx **Transaction) error
	// Manual coin selection, wrapper of [`lwk_wollet::TxBuilder::set_wallet_utxos()`]
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

// Add external utxos, wrapper of [`lwk_wollet::TxBuilder::add_external_utxos()`]
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

// Issue an asset, wrapper of [`lwk_wollet::TxBuilder::issue_asset()`]
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

// Reissue an asset, wrapper of [`lwk_wollet::TxBuilder::reissue_asset()`]
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

// Manual coin selection, wrapper of [`lwk_wollet::TxBuilder::set_wallet_utxos()`]
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

type TxInInterface interface {
	// Outpoint
	Outpoint() *OutPoint
}
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
	// If explicit returns the value, if confidential [None]
	Value() *uint64
}
type TxOut struct {
	ffiObject FfiObject
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

type TxOutSecretsInterface interface {
	Asset() AssetId
	AssetBf() Hex
	Value() uint64
	ValueBf() Hex
}
type TxOutSecrets struct {
	ffiObject FfiObject
}

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

func (_self *TxOutSecrets) Value() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*TxOutSecrets")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_txoutsecrets_value(
			_pointer, _uniffiStatus)
	}))
}

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

type TxidInterface interface {
	Bytes() []byte
}
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

// Wrapper over [`lwk_wollet::LiquidexProposal<Unvalidated>`]
type UnvalidatedLiquidexProposalInterface interface {
	InsecureValidate() (*ValidatedLiquidexProposal, error)
	NeededTx() (*Txid, error)
	Validate(previousTx *Transaction) (*ValidatedLiquidexProposal, error)
}

// Wrapper over [`lwk_wollet::LiquidexProposal<Unvalidated>`]
type UnvalidatedLiquidexProposal struct {
	ffiObject FfiObject
}

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
	Serialize() ([]byte, error)
}

// Wrapper over [`lwk_wollet::Update`]
type Update struct {
	ffiObject FfiObject
}

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

// Wrapper over [`lwk_wollet::LiquidexProposal<Validated>`]
type ValidatedLiquidexProposalInterface interface {
	Input() *AssetAmount
	Output() *AssetAmount
}

// Wrapper over [`lwk_wollet::LiquidexProposal<Validated>`]
type ValidatedLiquidexProposal struct {
	ffiObject FfiObject
}

func (_self *ValidatedLiquidexProposal) Input() *AssetAmount {
	_pointer := _self.ffiObject.incrementPointer("*ValidatedLiquidexProposal")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterAssetAmountINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_validatedliquidexproposal_input(
			_pointer, _uniffiStatus)
	}))
}

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

type WalletTxInterface interface {
	Balance() map[AssetId]int64
	Fee() uint64
	Height() *uint32
	Inputs() []**WalletTxOut
	Outputs() []**WalletTxOut
	Timestamp() *uint32
	Tx() *Transaction
	Txid() *Txid
	Type() string
	UnblindedUrl(explorerUrl string) string
}
type WalletTx struct {
	ffiObject FfiObject
}

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

func (_self *WalletTx) Fee() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*WalletTx")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_lwk_fn_method_wallettx_fee(
			_pointer, _uniffiStatus)
	}))
}

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

func (_self *WalletTx) Tx() *Transaction {
	_pointer := _self.ffiObject.incrementPointer("*WalletTx")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTransactionINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wallettx_tx(
			_pointer, _uniffiStatus)
	}))
}

func (_self *WalletTx) Txid() *Txid {
	_pointer := _self.ffiObject.incrementPointer("*WalletTx")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTxidINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wallettx_txid(
			_pointer, _uniffiStatus)
	}))
}

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

type WalletTxOutInterface interface {
	Address() *Address
	ExtInt() Chain
	Height() *uint32
	Outpoint() *OutPoint
	ScriptPubkey() *Script
	Unblinded() *TxOutSecrets
	WildcardIndex() uint32
}
type WalletTxOut struct {
	ffiObject FfiObject
}

func (_self *WalletTxOut) Address() *Address {
	_pointer := _self.ffiObject.incrementPointer("*WalletTxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterAddressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wallettxout_address(
			_pointer, _uniffiStatus)
	}))
}

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

func (_self *WalletTxOut) Outpoint() *OutPoint {
	_pointer := _self.ffiObject.incrementPointer("*WalletTxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOutPointINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wallettxout_outpoint(
			_pointer, _uniffiStatus)
	}))
}

func (_self *WalletTxOut) ScriptPubkey() *Script {
	_pointer := _self.ffiObject.incrementPointer("*WalletTxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterScriptINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wallettxout_script_pubkey(
			_pointer, _uniffiStatus)
	}))
}

func (_self *WalletTxOut) Unblinded() *TxOutSecrets {
	_pointer := _self.ffiObject.incrementPointer("*WalletTxOut")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTxOutSecretsINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_method_wallettxout_unblinded(
			_pointer, _uniffiStatus)
	}))
}

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

// A Watch-Only wallet, wrapper over [`lwk_wollet::Wollet`]
type WolletInterface interface {
	Address(index *uint32) (*AddressResult, error)
	ApplyUpdate(update *Update) error
	Balance() (map[AssetId]uint64, error)
	Descriptor() (*WolletDescriptor, error)
	Finalize(pset *Pset) (*Pset, error)
	PsetDetails(pset *Pset) (*PsetDetails, error)
	Transactions() ([]*WalletTx, error)
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

// Construct a Watch-Only wallet object with a caller provided persister
func WolletWithCustomPersister(network *Network, descriptor *WolletDescriptor, persister *ForeignPersisterLink) (*Wollet, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[LwkError](FfiConverterLwkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_lwk_fn_constructor_wollet_with_custom_persister(FfiConverterNetworkINSTANCE.Lower(network), FfiConverterWolletDescriptorINSTANCE.Lower(descriptor), FfiConverterForeignPersisterLinkINSTANCE.Lower(persister), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Wollet
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWolletINSTANCE.Lift(_uniffiRV), nil
	}
}

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

// The output descriptors, wrapper over [`lwk_wollet::WolletDescriptor`]
type WolletDescriptorInterface interface {
	// Derive the private blinding key
	DeriveBlindingKey(scriptPubkey *Script) **SecretKey
	IsMainnet() bool
	// Derive a scriptpubkey
	ScriptPubkey(extInt Chain, index uint32) (*Script, error)
}

// The output descriptors, wrapper over [`lwk_wollet::WolletDescriptor`]
type WolletDescriptor struct {
	ffiObject FfiObject
}

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

type FfiConverterLwkError struct{}

var FfiConverterLwkErrorINSTANCE = FfiConverterLwkError{}

func (c FfiConverterLwkError) Lift(eb RustBufferI) *LwkError {
	return LiftFromRustBuffer[*LwkError](c, eb)
}

func (c FfiConverterLwkError) Lower(value *LwkError) C.RustBuffer {
	return LowerIntoRustBuffer[*LwkError](c, value)
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
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerLwkError.Destroy", value))
	}
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

func IsProvablySegwit(scriptPubkey *Script, redeemScript **Script) bool {
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_lwk_fn_func_is_provably_segwit(FfiConverterScriptINSTANCE.Lower(scriptPubkey), FfiConverterOptionalScriptINSTANCE.Lower(redeemScript), _uniffiStatus)
	}))
}
