//go:build tinygo.wasm || wasm

// Command gotebanare-wasm is the engine that the browser extension and
// other JavaScript hosts load. Build it with TinyGo for wasm-unknown (see
// the Makefile target wasm). The module imports nothing and exports the
// functions below; internal/bridge holds the logic.
//
// Memory protocol: the host calls alloc, copies input bytes to the returned
// address, and calls an export. Exports that return data return the address
// of a JSON buffer whose length is result_len(). The host frees every input
// buffer and every result buffer with free.
//
// A YAML syntax error in the config passed to compile stops the module
// with a trap (see bridge.Bridge.Compile). yaml_progress_ptr returns the
// address of a uint32 that the host reads after such a trap to find where
// the YAML parser stopped.
package main

import (
	"unsafe"

	"github.com/miyamo2/go-tebanare/internal/bridge"
)

var (
	engine = bridge.New()
	// buffers keeps every buffer handed to the host reachable until the
	// host frees it.
	buffers = map[uint32][]byte{}
	lastLen uint32
)

//go:wasmexport alloc
func alloc(size uint32) uint32 {
	// Allocate at least one byte so every live buffer has its own address.
	buf := make([]byte, size, max(size, 1))
	return keep(buf)
}

//go:wasmexport free
func free(ptr uint32) {
	delete(buffers, ptr)
}

//go:wasmexport result_len
func resultLen() uint32 {
	return lastLen
}

//go:wasmexport info
func info() uint32 {
	return result(engine.Info())
}

//go:wasmexport compile
func compile(yamlPtr, yamlLen uint32) uint32 {
	return result(engine.Compile(input(yamlPtr, yamlLen)))
}

//go:wasmexport analyze_change
func analyzeChange(handle, metaPtr, metaLen, oldPtr, oldLen, newPtr, newLen uint32) uint32 {
	return result(engine.AnalyzeChange(handle, input(metaPtr, metaLen), input(oldPtr, oldLen), input(newPtr, newLen)))
}

//go:wasmexport release
func release(handle uint32) {
	engine.Release(handle)
}

//go:wasmexport presets
func presets() uint32 {
	return result(engine.Presets())
}

//go:wasmexport yaml_progress_ptr
func yamlProgressPtr() uint32 {
	return uint32(uintptr(unsafe.Pointer(engine.YAMLProgress())))
}

func keep(buf []byte) uint32 {
	ptr := uint32(uintptr(unsafe.Pointer(unsafe.SliceData(buf))))
	buffers[ptr] = buf
	return ptr
}

// input views host-written bytes in linear memory. A pointer outside the
// memory traps on first access.
func input(ptr, n uint32) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), n)
}

func result(b []byte) uint32 {
	if cap(b) == 0 {
		b = make([]byte, 0, 1)
	}
	lastLen = uint32(len(b))
	return keep(b)
}

func main() {}
