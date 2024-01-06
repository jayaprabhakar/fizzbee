package modelchecker

import (
	"fizz/ast"
	"fmt"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

func GetProtoFieldByPath(file *ast.File, location string) proto.Message {
	field := GetFieldByPath(file, location)
	if field == nil {
		return nil
	}
	glog.Infof("field: %+v, value:%+v, type:%+v", field, field.Interface(), field.Type())
	protobuf := convertToProto(field.Elem().Interface(), field.Type())
	glog.Infof("protobuf type: %+v", reflect.TypeOf(protobuf))
	return protobuf
}

func GetStringFieldByPath(file *ast.File, location string) (string, bool) {
	field := GetFieldByPath(file, location)
	if field == nil {
		return "", false
	}
	t := field.Type()
	glog.Infof("field: %+v, value:%+v, type:%+v", field, field.Interface(), t)
	if t.Kind() == reflect.String {
		str := field.Interface().(string)
		return str, true
	}
	return "", false
}

func convertToProto(value interface{}, messageType reflect.Type) proto.Message {
	// Create a new instance of the protobuf message type
	protoInstance := reflect.New(messageType.Elem()).Interface().(proto.Message)

	// Use reflection to set the value of the message
	protoValue := reflect.ValueOf(protoInstance).Elem()
	protoValue.Set(reflect.ValueOf(value))

	return protoInstance.(proto.Message)
}

func GetFieldByPath(msg proto.Message, path string) *reflect.Value {
	v := reflect.ValueOf(msg).Elem()
	parts := strings.Split(path, ".")
	//fmt.Printf("before loop, %+v\n", v)
	//fmt.Printf("parts, %+v\n", parts)

	for _, part := range parts {
		if strings.Contains(part, "[") && strings.Contains(part, "]") {
			// Handle repeated field index
			fieldName := strings.Split(part, "[")[0]
			indexStr := strings.Split(strings.Split(part, "[")[1], "]")[0]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				panic(err)
				return nil
			}
			if v.Kind() == reflect.Ptr {
				v = v.Elem()
			}
			field := v.FieldByName(fieldName)
			//fmt.Printf("index: %d, fieldName: %s, field: %+v\n", index, fieldName, field)
			if !field.IsValid() || field.Kind() != reflect.Slice {
				return nil
			}
			if index < 0 || index >= field.Len() {
				return nil
			}

			v = field.Index(index)
		} else {
			// Handle regular fields
			field := v.Elem().FieldByName(part)
			if !field.IsValid() {
				return nil
			}
			v = field
		}
	}

	return &v //.Interface()
}

func GetNextFieldPath(msg proto.Message, path string) (string, *reflect.Value) {
	v := reflect.ValueOf(msg).Elem()
	_ = v
	parts := strings.Split(path, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if strings.Contains(part, "[") && strings.Contains(part, "]") {
			fieldName := strings.Split(part, "[")[0]
			indexStr := strings.Split(strings.Split(part, "[")[1], "]")[0]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				panic(err)
				return "", nil
			}
			if fieldName != "Stmts" {
				continue
			}
			nextFieldName := ""
			if i > 0 {
				prefix := strings.Join(parts[0:i], ".")
				nextFieldName = fmt.Sprintf("%s.%s[%d]", prefix, fieldName, index+1)
			} else {
				nextFieldName = fmt.Sprintf("%s[%d]", fieldName, index+1)
			}

			nextField := GetFieldByPath(msg, nextFieldName)
			if nextField != nil {
				return nextFieldName, nextField
			}
			nextFieldName = ""
			if i > 0 {
				prefix := strings.Join(parts[0:i], ".")
				nextFieldName = fmt.Sprintf("%s.$", prefix)
			} else {
				nextFieldName = fmt.Sprintf("%s[%d]", fieldName, index+1)
			}
			return nextFieldName, nil
		}
	}
	return "", nil
}

func ParentBlockPath(path string) string {
	lastIndex := strings.LastIndex(path, ".Block")
	if lastIndex == -1 {
		return ""
	}
	return path[:lastIndex] + ".Block"
}

func RemoveLastBlock(path string) string {
	return RemoveLastSegment(path, ".Block")
}

func RemoveLastForStmt(path string) string {
	return RemoveLastSegment(path, ".ForStmt")
}

func RemoveLastWhileStmt(path string) string {
	return RemoveLastSegment(path, ".WhileStmt")
}

func RemoveLastSegment(path string, substr string) string {
	lastIndex := strings.LastIndex(path, substr)
	if lastIndex == -1 {
		return ""
	}
	return path[:lastIndex]
}

func EndOfBlock(path string) string {
	return replaceLastStmts(path, "$")
}

func replaceLastStmts(input, replacement string) string {
	re := regexp.MustCompile(`Stmts\[\d+\]`)
	matches := re.FindAllStringIndex(input, -1)

	if matches == nil {
		// Pattern not found
		return input
	}

	lastMatch := matches[len(matches)-1]
	result := input[:lastMatch[0]] + replacement + input[lastMatch[1]:]
	return result
}
