package gorm

import (
	"reflect"
	"strings"
	"strconv"
	"os"
)

func getTypeName(obj interface{}) (typestr string) {
	typ := reflect.TypeOf(obj)
	typestr = typ.String()

	lastDotIndex := strings.LastIndex(typestr, ".")
	if lastDotIndex != -1 {
		typestr = typestr[lastDotIndex+1:]
	}

	return
}

func snakeCasedName(name string) string {
	newstr := make([]int, 0)
	firstTime := true

	for _, chr := range name {
		if isUpper := 'A' <= chr && chr <= 'Z'; isUpper {
			if firstTime == true {
				firstTime = false
			} else {
				newstr = append(newstr, '_')
			}
			chr -= ('A' - 'a')
		}
		newstr = append(newstr, chr)
	}

	return string(newstr)
}

func titleCasedName(name string) string {
	newstr := make([]int, 0)
	upNextChar := true

	for _, chr := range name {
		switch {
		case upNextChar:
			upNextChar = false
			chr -= ('a' - 'A')
		case chr == '_':
			upNextChar = true
			continue
		}

		newstr = append(newstr, chr)
	}

	return string(newstr)
}

func pluralizeString(str string) string {
	if strings.HasSuffix(str, "y") {
		str = str[:len(str)-1] + "ie"
	}
	return str + "s"
}

func scanMapIntoStruct(obj interface{}, objMap map[string][]byte) os.Error {
	dataStruct := reflect.Indirect(reflect.ValueOf(obj))
	if dataStruct.Kind() != reflect.Struct {
		return os.NewError("expected a pointer to a struct")
	}

	for key, data := range objMap {
		structField := dataStruct.FieldByName(titleCasedName(key))
		if !structField.CanSet() {
			continue
		}

		var v interface{}

		switch structField.Type().Kind() {
		case reflect.Slice:
			v = data
		case reflect.String:
			v = string(data)
		case reflect.Bool:
			v = string(data) == "1"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			x, err := strconv.Atoi(string(data))
			if err != nil {
				return os.NewError("arg " + key + " as int: " + err.String())
			}
			v = x
		case reflect.Float32, reflect.Float64:
			x, err := strconv.Atof64(string(data))
			if err != nil {
				return os.NewError("arg " + key + " as float64: " + err.String())
			}
			v = x
		default:
			return os.NewError("unsupported type in Scan: " + reflect.TypeOf(v).String())
		}

		structField.Set(reflect.ValueOf(v))
	}

	return nil
}

func scanStructIntoMap(obj interface{}) (map[string]interface{}, os.Error) {
	dataStruct := reflect.Indirect(reflect.ValueOf(obj))
	if dataStruct.Kind() != reflect.Struct {
		return nil, os.NewError("expected a pointer to a struct")
	}

	dataStructType := dataStruct.Type()

	mapped := make(map[string]interface{})

	for i := 0; i < dataStructType.NumField(); i++ {
		field := dataStructType.Field(i)
		fieldName := field.Name

		mapKey := snakeCasedName(fieldName)
		value := dataStruct.FieldByName(fieldName).Interface()

		mapped[mapKey] = value
	}

	return mapped, nil
}
