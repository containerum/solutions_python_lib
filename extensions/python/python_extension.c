#define Py_LIMITED_API // use stable api
#include <Python.h>
#include "_cgo_export.h"

static PyMethodDef solutions_SolutionMethods[] = {
    { "generate_run_sequence", (PyCFunction)solutions_Solution_GenerateRunSequence, METH_O,
        PyDoc_STR("Generate list of json configs to run solution")},
    { "set_value", (PyCFunction)solutions_Solution_SetValue, METH_VARARGS,
        PyDoc_STR("Set value for custom template variable")},
    { "add_values", (PyCFunction)solutions_Solution_AddValues, METH_O,
        PyDoc_STR("Set values for custom template variables using dict")},
    { NULL, NULL } // terminating
};

static PyType_Slot solutions_SolutionTypeSlots[] = {
    { Py_tp_doc, PyDoc_STR("Parses solution config and generate run sequence") },
    { Py_tp_dealloc, solutions_SolutionDealloc },
    { Py_tp_methods, solutions_SolutionMethods },
    { Py_tp_init, solutions_SolutionInit },
    { Py_tp_new, PyType_GenericNew },
    { 0 } // terminating
};

static PyType_Spec solutions_SolutionTypeSpec = {
    "solutions.Solution",                     // name
    sizeof(solutions_SolutionObj),            // basicsize
    0,                                        // itemsize
    Py_TPFLAGS_DEFAULT | Py_TPFLAGS_BASETYPE, // flags
    solutions_SolutionTypeSlots               // slots
};

static PyTypeObject *solutions_SolutionType;

static PyModuleDef solutionsModule = {
	PyModuleDef_HEAD_INIT,
	"solutions",                                     // module name
	PyDoc_STR("Module for parsing solution config"), // module documentation
	-1,
	NULL, NULL, NULL, NULL, NULL
};

// Workaround missing variadic function support
// https://github.com/golang/go/issues/975
int PyArg_ParseTuple_S(PyObject * args, char ** arg) {
    return PyArg_ParseTuple(args, "s", arg);
}

int PyArg_ParseTuple_SS(PyObject * args, char ** key, char ** value) {
    return PyArg_ParseTuple(args, "ss", key, value);
}

int PyArg_MyArgsParsing(PyObject *args, char **content, char **user, char **label, char **branch){
	return PyArg_ParseTuple(args, "ssss", content, user, label, branch);
};

PyObject *Py_BuildString(char * string) {
	return Py_BuildValue("s", string);
}

PyObject *Py_BuildEmpty() {
    return Py_BuildValue("");
}

void Py_decref(PyObject * obj) {
    Py_DECREF(obj);
}

void Py_incref(PyObject * obj) {
    Py_INCREF(obj);
}

PyMODINIT_FUNC
PyInit_solutions(void)
{
    PyObject *m;

    m = PyModule_Create(&solutionsModule);
    if (m == NULL)
        return NULL;

    solutions_SolutionType = (PyTypeObject *)PyType_FromSpec(&solutions_SolutionTypeSpec);
    Py_INCREF(solutions_SolutionType);
    PyModule_AddObject(m, "Solution", (PyObject *)solutions_SolutionType); // attach type to package

    SolutionError = PyErr_NewException("solutions.SolutionError", NULL, NULL);
    Py_INCREF(SolutionError);
    PyModule_AddObject(m, "SolutionError", SolutionError); // attach exception to package
    return m;
}
