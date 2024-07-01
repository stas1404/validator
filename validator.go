package validotor

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

var ErrNotStruct = errors.New("wrong argument given, should be a struct")
var ErrInvalidValidatorSyntax = errors.New("invalid validator syntax")
var ErrValidateForUnexportedFields = errors.New("validation for unexported field is not allowed")

type ValidationError struct {
	Err error
}

type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	var err string
	for _, errs := range v {
		err = err + errs.Err.Error() + " ,"
	}
	if len(err) > 0 {
		return err[:len(err)-2]
	}
	return err
}

func ValidateField(f reflect.StructField, v reflect.Value, Mu *sync.Mutex, valid_err *ValidationErrors, wg *sync.WaitGroup) {
	defer wg.Done()
	tagstr, ok := f.Tag.Lookup("validate")
	if !ok {
		return
	}
	err := CheckExport(f, Mu, valid_err)
	if err != nil {
		return
	}
	tags := strings.Split(tagstr, ", ")
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		tag_word, res, found := strings.Cut(tag, ":")
		if !found {
			Mu.Lock()
			*valid_err = append(*valid_err, ValidationError{Err: ErrInvalidValidatorSyntax})
			Mu.Unlock()
			return
		}
		switch tag_word {
		case "min":
			min_value, err := strconv.Atoi(res)
			if err != nil {
				Mu.Lock()
				*valid_err = append(*valid_err, ValidationError{Err: ErrInvalidValidatorSyntax})
				Mu.Unlock()
				return
			}
			MinCheck(min_value, f, v, Mu, valid_err)
		case "max":
			max_value, err := strconv.Atoi(res)
			if err != nil {
				Mu.Lock()
				*valid_err = append(*valid_err, ValidationError{Err: ErrInvalidValidatorSyntax})
				Mu.Unlock()
				return
			}
			MaxCheck(max_value, f, v, Mu, valid_err)
		case "len":
			len, err := strconv.Atoi(res)
			if err != nil {
				Mu.Lock()
				*valid_err = append(*valid_err, ValidationError{Err: ErrInvalidValidatorSyntax})
				Mu.Unlock()
				return
			}
			MaxCheck(len, f, v, Mu, valid_err)
			MinCheck(len, f, v, Mu, valid_err)
		case "in":
			InCheck(f, v, res, f.Name, Mu, valid_err)
		}
	}
}

func CheckExport(f reflect.StructField, Mu *sync.Mutex, valid_err *ValidationErrors) error {
	if !f.IsExported() {
		Mu.Lock()
		*valid_err = append(*valid_err, ValidationError{Err: ErrValidateForUnexportedFields})
		Mu.Unlock()
		return errors.New("field is not exported")
	}
	return nil
}

func MinCheck(min_value int, f reflect.StructField, v reflect.Value, Mu *sync.Mutex, valid_err *ValidationErrors) {
	switch f.Type.Kind() {
	case reflect.String:
		MinCheckInt(int64(v.Len()), min_value, f.Name, Mu, valid_err)
	case reflect.Int:
		MinCheckInt(v.Int(), min_value, f.Name, Mu, valid_err)
	}
}

func MinCheckInt(v int64, min_value int, name string, Mu *sync.Mutex, valid_err *ValidationErrors) {
	if v < int64(min_value) {
		Mu.Lock()
		*valid_err = append(*valid_err, ValidationError{Err: errors.New("field " + name + " is less than " + strconv.Itoa(min_value))})
		Mu.Unlock()
	}
}

func MaxCheck(max_value int, f reflect.StructField, v reflect.Value, Mu *sync.Mutex, valid_err *ValidationErrors) {
	switch f.Type.Kind() {
	case reflect.String:
		MaxCheckInt(int64(v.Len()), max_value, f.Name, Mu, valid_err)
	case reflect.Int:
		MaxCheckInt(v.Int(), max_value, f.Name, Mu, valid_err)
	}
}

func MaxCheckInt(v int64, max_value int, name string, Mu *sync.Mutex, valid_err *ValidationErrors) {
	if v > int64(max_value) {
		Mu.Lock()
		*valid_err = append(*valid_err, ValidationError{Err: errors.New("field " + name + " is greater than " + strconv.Itoa(max_value))})
		Mu.Unlock()
	}
}

func InCheck(f reflect.StructField, v reflect.Value, res string, name string, Mu *sync.Mutex, valid_err *ValidationErrors) {
	var s string
	switch f.Type.Kind() {
	case reflect.Int:
		s = strconv.FormatInt(v.Int(), 10)
	case reflect.String:
		s = v.String()
	}
	InCheckString(s, res, name, Mu, valid_err)
}

func InCheckString(value string, res string, name string, Mu *sync.Mutex, valid_err *ValidationErrors) {
	if res == "" {
		WriteInCheckError(res, name, Mu, valid_err)
		return
	}
	acceptable_values := strings.Split(res, ",")
	for _, acc_val := range acceptable_values {
		if acc_val == value {
			return
		}
	}
	WriteInCheckError(res, name, Mu, valid_err)

}

func WriteInCheckError(res string, name string, Mu *sync.Mutex, valid_err *ValidationErrors) {
	Mu.Lock()
	*valid_err = append(*valid_err, ValidationError{Err: errors.New("value of field " + name + " is not in " + res)})
	Mu.Unlock()
}

func Validate(v any) error {
	v_type := reflect.TypeOf(v)
	if v_type.Kind() != reflect.Struct {
		return ErrNotStruct
	}
	var (
		Mu        sync.Mutex
		valid_err ValidationErrors
		wg        sync.WaitGroup
	)
	v_value := reflect.ValueOf(v)
	for i := 0; i < v_type.NumField(); i++ {
		f := v_type.Field(i)
		f_val := v_value.Field(i)
		wg.Add(1)
		go ValidateField(f, f_val, &Mu, &valid_err, &wg)
	}
	wg.Wait()
	if len(valid_err) == 0 {
		return nil
	}
	return valid_err
}
