package main

/*
#cgo pkg-config: python3
#cgo CFLAGS: -Werror
#define Py_LIMITED_API
#include <Python.h>
#include <stdint.h>

typedef struct {
    PyObject_HEAD
    uint64_t objId; // ID in solution object map (use it to prevent exchanging pointers between C and Go)
} solutions_SolutionObj;

int PyArg_ParseTuple_S(PyObject * args, char ** arg);
int PyArg_ParseTuple_SS(PyObject * args, char ** key, char ** value);
int PyArg_MyArgsParsing(PyObject *args, char **content, char **user, char **label, char **branch);
void Py_decref(PyObject *obj);
void Py_incref(PyObject *obj);
PyObject *Py_BuildEmpty(); // value to return if method not produces useful value
PyObject *Py_BuildString(char * string); // convert C string to Python string
PyObject *SolutionError; // 'solutions.SolutionError' exception
*/
import "C"
import (
	"errors"
	"reflect"
	"sync"
	"unsafe"

	"github.com/containerum/solutions"
)

// special struct to link python object to go object without passing pointers to C code
var objMap = struct {
	mp map[uint64]*solutions.Solution
	mu *sync.Mutex
	id uint64 // write-than-increment
}{
	mp: make(map[uint64]*solutions.Solution),
	mu: new(sync.Mutex),
	id: 0,
}

func objToString(obj *C.PyObject) (string, error) {
	pyAsciiStr := C.PyUnicode_AsASCIIString(obj)
	if pyAsciiStr == nil {
		return "", errors.New("non-string object received")
	}
	retRaw := C.PyBytes_AsString(pyAsciiStr)
	if retRaw == nil {
		return "", errors.New("can`t decode string")
	}
	C.Py_decref(pyAsciiStr)
	return C.GoString(retRaw), nil
}

// raise 'solutions.SolutionError' exception
func raiseString(text string) {
	textPtr := C.CString(text)
	defer C.free(unsafe.Pointer(textPtr))
	C.PyErr_SetString(C.SolutionError, textPtr)
}

//export solutions_SolutionInit
func solutions_SolutionInit(self *C.solutions_SolutionObj, args, kwds *C.PyObject) int {
	var contentRaw, userRaw, labelRaw, branchRaw *C.char
	if C.PyArg_MyArgsParsing(args, &contentRaw, &userRaw, &labelRaw, &branchRaw) == -1 {
		raiseString("Args parsing error")
		return -1
	}

	content, user, label, branch := C.GoString(contentRaw), C.GoString(userRaw), C.GoString(labelRaw), C.GoString(branchRaw)
	s, err := solutions.OpenSolution(content, user, label, branch)
	if err != nil {
		raiseString("open solution: " + err.Error())
		return -1
	}

	// store solution object in map and set id in C struct
	objMap.mu.Lock()
	objMap.mp[objMap.id] = s
	self.objId = C.uint64_t(objMap.id)
	objMap.id++
	objMap.mu.Unlock()

	return 0
}

//export solutions_SolutionDealloc
func solutions_SolutionDealloc(self *C.solutions_SolutionObj) {
	objMap.mu.Lock()
	objMap.mp[uint64(self.objId)] = nil
	objMap.mu.Unlock()
}

//export solutions_Solution_GenerateRunSequence
func solutions_Solution_GenerateRunSequence(self *C.solutions_SolutionObj, nsArg *C.PyObject) *C.PyObject {
	var ns string
	var err error
	if ns, err = objToString(nsArg); err != nil {
		raiseString("expected one string argument, decode error " + err.Error())
		return nil
	}

	objMap.mu.Lock()
	s, ok := objMap.mp[uint64(self.objId)]
	objMap.mu.Unlock()
	if !ok {
		raiseString("cannot find object in objectMap")
		return nil
	}

	seq, err := s.GenerateRunSequence(ns)
	if err != nil {
		raiseString("generate run sequence: " + err.Error())
		return nil
	}

	ret := C.PyList_New(C.Py_ssize_t(len(seq)))
	if ret == nil {
		raiseString("cannot create return list")
		return nil
	}

	for i, v := range seq {
		dict := C.PyDict_New()
		if dict == nil {
			raiseString("cannot create dict")
			return nil
		}

		refV := reflect.ValueOf(v)
		for field := 0; field < refV.NumField(); field++ {
			fieldName := refV.Type().Field(field).Name
			fieldValue := refV.Field(field).String()
			fieldNamePtr := C.CString(fieldName)
			fieldValuePtr := C.CString(fieldValue)
			code := C.PyDict_SetItemString(dict, fieldNamePtr, C.Py_BuildString(fieldValuePtr))
			C.free(unsafe.Pointer(fieldNamePtr))
			C.free(unsafe.Pointer(fieldValuePtr))
			if code == -1 {
				raiseString("error putting to " + fieldName)
				return nil
			}
		}

		C.PyList_SetItem(ret, C.Py_ssize_t(i), dict)
	}

	return ret
}

//export solutions_Solution_SetValue
func solutions_Solution_SetValue(self *C.solutions_SolutionObj, args *C.PyObject) *C.PyObject {
	var keyRaw, valueRaw *C.char
	if C.PyArg_ParseTuple_SS(args, &keyRaw, &valueRaw) == -1 {
		raiseString("expected two string arguments")
		return nil
	}
	key, value := C.GoString(keyRaw), C.GoString(valueRaw)

	objMap.mu.Lock()
	s, ok := objMap.mp[uint64(self.objId)]
	objMap.mu.Unlock()
	if !ok {
		raiseString("cannot find object in objectMap")
		return nil
	}

	s.SetValue(key, value)

	return C.Py_BuildEmpty()
}

//export solutions_Solution_AddValues
func solutions_Solution_AddValues(self *C.solutions_SolutionObj, dictArg *C.PyObject) *C.PyObject {
	objMap.mu.Lock()
	s, ok := objMap.mp[uint64(self.objId)]
	objMap.mu.Unlock()
	if !ok {
		raiseString("cannot find object in objectMap")
		return nil
	}

	var keyArg, valueArg *C.PyObject
	var pos C.Py_ssize_t
	var kv = make(map[string]interface{})
	for C.PyDict_Next(dictArg, &pos, &keyArg, &valueArg) != 0 {
		key, err := objToString(keyArg)
		if err != nil {
			raiseString("can`t decode dict key, must be string: " + err.Error())
			return nil
		}

		value, err := objToString(valueArg)
		if err != nil {
			raiseString("can`t decode dict value, must be string: " + err.Error())
			return nil
		}
		kv[key] = value
	}

	s.AddValues(kv)

	return C.Py_BuildEmpty()
}

// to setup go runtime
func main() {}
