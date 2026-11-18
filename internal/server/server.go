package server

/*
#cgo darwin CFLAGS: -DSAP_UC_is_wchar -DSAPwithUNICODE -DSAPwithTHREADS -DSAPonDARW
#cgo linux CFLAGS: -DSAPwithUNICODE -DSAPwithTHREADS -DSAPonUNIX -DSAPonLIN
#cgo LDFLAGS: -lsapnwrfc -lsapucum
#include "bridge.h"
*/
import "C"

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"unicode/utf16"
	"unsafe"

	"rfc-server/internal/config"
)

var (
	functions map[string]config.Function
	runMu     sync.Mutex
)

func sap(s string) *C.SAP_UC {
	units := utf16.Encode([]rune(s))
	ptr := C.malloc(C.size_t((len(units) + 1) * 2))
	if ptr == nil {
		panic("SAP string allocation failed")
	}

	dst := unsafe.Slice((*uint16)(ptr), len(units)+1)
	copy(dst, units)
	dst[len(units)] = 0
	return (*C.SAP_UC)(ptr)
}

func text(ptr *C.SAP_UC) string {
	var units []uint16
	for i := 0; ; i++ {
		unit := *(*uint16)(unsafe.Add(unsafe.Pointer(ptr), i*2))
		if unit == 0 {
			break
		}

		units = append(units, unit)
	}

	return string(utf16.Decode(units))
}

func sdkError(info *C.RFC_ERROR_INFO) error {
	return fmt.Errorf("RFC %d %s: %s", info.code, text(&info.key[0]), text(&info.message[0]))
}

func description(fn config.Function) (C.RFC_FUNCTION_DESC_HANDLE, error) {
	var info C.RFC_ERROR_INFO
	name := sap(fn.Name)
	defer C.free(unsafe.Pointer(name))

	desc := C.RfcCreateFunctionDesc(name, &info)
	if desc == nil {
		return nil, sdkError(&info)
	}

	for _, p := range fn.Parameters {
		var param C.RFC_PARAMETER_DESC
		for i, r := range p.Name {
			param.name[i] = C.SAP_UC(r)
		}

		switch p.Direction {
		case "import":
			param.direction = C.RFC_IMPORT
		case "export":
			param.direction = C.RFC_EXPORT
		case "changing":
			param.direction = C.RFC_CHANGING
		}

		switch p.Type {
		case "CHAR":
			param._type = C.RFCTYPE_CHAR
			param.nucLength = C.uint(p.Length)
			param.ucLength = C.uint(p.Length * 2)
		case "STRING":
			param._type = C.RFCTYPE_STRING
		case "INT":
			param._type = C.RFCTYPE_INT
			param.nucLength = 4
			param.ucLength = 4
		case "FLOAT":
			param._type = C.RFCTYPE_FLOAT
			param.nucLength = 8
			param.ucLength = 8
		}

		if C.RfcAddParameter(desc, &param, &info) != C.RFC_OK {
			err := sdkError(&info)
			C.RfcDestroyFunctionDesc(desc, &info)
			return nil, err
		}
	}

	return desc, nil
}

func Run(ctx context.Context, cfg *config.Config) error {
	runMu.Lock()
	defer runMu.Unlock()

	if err := cfg.Validate(); err != nil {
		return err
	}

	functions = make(map[string]config.Function, len(cfg.Functions))
	var info C.RFC_ERROR_INFO
	for _, fn := range cfg.Functions {
		desc, err := description(fn)
		if err != nil {
			return err
		}

		if C.mockInstall(desc, &info) != C.RFC_OK {
			return sdkError(&info)
		}

		functions[fn.Name] = fn
	}

	if C.mockMetadata(&info) != C.RFC_OK {
		return sdkError(&info)
	}

	if C.mockAuth(&info) != C.RFC_OK {
		return sdkError(&info)
	}

	values := [][2]string{{"SERVER_NAME", "LAWANG_MOCK"}, {"WSPORT", strconv.Itoa(cfg.Port)}, {"USE_TLS", "0"}, {"REG_COUNT", "4"}}
	params := make([]C.RFC_CONNECTION_PARAMETER, len(values))
	for i, pair := range values {
		params[i].name = sap(pair[0])
		params[i].value = sap(pair[1])
	}
	defer func() {
		for _, p := range params {
			C.free(unsafe.Pointer(p.name))
			C.free(unsafe.Pointer(p.value))
		}
	}()

	handle := C.RfcCreateServer(&params[0], C.uint(len(params)), &info)
	if handle == nil {
		return sdkError(&info)
	}
	defer C.RfcDestroyServer(handle, &info)

	if C.RfcLaunchServer(handle, &info) != C.RFC_OK {
		return sdkError(&info)
	}

	log.Printf("RFC WebSocket mock on port %d; %d functions; TLS/auth disabled for local testing", cfg.Port, len(cfg.Functions))
	<-ctx.Done()
	if C.RfcShutdownServer(handle, 10, &info) != C.RFC_OK {
		return sdkError(&info)
	}

	return nil
}

//export mockDispatch
func mockDispatch(_ C.RFC_CONNECTION_HANDLE, handle C.RFC_FUNCTION_HANDLE, info *C.RFC_ERROR_INFO) C.RFC_RC {
	desc := C.RfcDescribeFunction(handle, info)
	if desc == nil {
		return info.code
	}

	var name C.RFC_ABAP_NAME
	if rc := C.RfcGetFunctionName(desc, &name[0], info); rc != C.RFC_OK {
		return rc
	}

	if text(&name[0]) == "RFC_GET_FUNCTION_INTERFACE" {
		return metadata(handle, info)
	}

	fn, ok := functions[text(&name[0])]
	if !ok {
		return C.RFC_NOT_FOUND
	}

	values := make(map[string]any)
	for _, p := range fn.Parameters {
		if p.Direction == "export" {
			continue
		}

		value, rc := read(handle, p, info)
		if rc != C.RFC_OK {
			return rc
		}

		values[p.Name] = value
	}

	for _, p := range fn.Parameters {
		if p.Direction == "import" {
			continue
		}

		value := p.Value
		if p.From != "" {
			value = values[p.From]
		}

		if value == nil {
			continue
		}

		if rc := write(handle, p, value, info); rc != C.RFC_OK {
			return rc
		}
	}

	log.Printf("RFC %s", fn.Name)
	return C.RFC_OK
}

func read(handle C.RFC_FUNCTION_HANDLE, p config.Parameter, info *C.RFC_ERROR_INFO) (any, C.RFC_RC) {
	name := sap(p.Name)
	defer C.free(unsafe.Pointer(name))

	switch p.Type {
	case "INT":
		var value C.RFC_INT
		rc := C.RfcGetInt(handle, name, &value, info)
		return int(value), rc
	case "FLOAT":
		var value C.RFC_FLOAT
		rc := C.RfcGetFloat(handle, name, &value, info)
		return float64(value), rc
	default:
		length := C.uint(p.Length)
		if p.Type == "STRING" {
			if rc := C.RfcGetStringLength(handle, name, &length, info); rc != C.RFC_OK {
				return nil, rc
			}
		}

		buffer := make([]C.SAP_UC, int(length)+1)
		rc := C.RfcGetString(handle, name, &buffer[0], C.uint(len(buffer)), &length, info)
		return text(&buffer[0]), rc
	}
}

func write(handle C.RFC_FUNCTION_HANDLE, p config.Parameter, value any, info *C.RFC_ERROR_INFO) C.RFC_RC {
	name := sap(p.Name)
	defer C.free(unsafe.Pointer(name))

	switch p.Type {
	case "INT":
		return C.RfcSetInt(handle, name, C.RFC_INT(value.(int)), info)
	case "FLOAT":
		number, ok := value.(float64)
		if !ok {
			number = float64(value.(int))
		}

		return C.RfcSetFloat(handle, name, C.RFC_FLOAT(number), info)
	default:
		s := value.(string)
		data := sap(s)
		defer C.free(unsafe.Pointer(data))

		return C.RfcSetString(handle, name, data, C.uint(len(utf16.Encode([]rune(s)))), info)
	}
}

func metadata(handle C.RFC_FUNCTION_HANDLE, info *C.RFC_ERROR_INFO) C.RFC_RC {
	value, rc := read(handle, config.Parameter{Name: "FUNCNAME", Type: "CHAR", Length: 30}, info)
	if rc != C.RFC_OK {
		return rc
	}

	fn, ok := functions[strings.TrimSpace(value.(string))]
	if !ok {
		return C.RFC_NOT_FOUND
	}

	name := sap("PARAMS")
	defer C.free(unsafe.Pointer(name))

	var table C.RFC_TABLE_HANDLE
	if rc := C.RfcGetTable(handle, name, &table, info); rc != C.RFC_OK {
		return rc
	}

	for i, p := range fn.Parameters {
		row := C.RfcAppendNewRow(table, info)
		if row == nil {
			return info.code
		}

		direction := map[string]string{"import": "I", "export": "E", "changing": "C"}[p.Direction]
		code := map[string]string{"CHAR": "C", "STRING": "g", "INT": "I", "FLOAT": "F"}[p.Type]
		length := p.Length
		switch p.Type {
		case "INT":
			length = 4
		case "FLOAT", "STRING":
			length = 8
		}

		for key, value := range map[string]any{"PARAMCLASS": direction, "PARAMETER": p.Name, "EXID": code, "POSITION": i + 1, "INTLENGTH": length} {
			kind := "STRING"
			if _, ok := value.(int); ok {
				kind = "INT"
			}

			if rc := write(row, config.Parameter{Name: key, Type: kind}, value, info); rc != C.RFC_OK {
				return rc
			}
		}
	}

	return C.RFC_OK
}
