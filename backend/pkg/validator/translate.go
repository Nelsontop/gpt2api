// Package validator 翻译 Gin binding 校验错误为可读中文消息。
package validator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/kleinai/backend/pkg/errcode"
)

// fieldTagMap 全局字段 JSON tag 映射表，key = "StructName.FieldName"。
// 由 Register 自动填充，无需手动维护。
var fieldTagMap = make(map[string]string)

// Register 用反射扫描 struct 类型，提取所有字段的 JSON tag 并注册到映射表。
// 传入 DTO struct 指针或零值实例即可，如 Register(&dto.RegisterReq{})。
func Register(types ...any) {
	for _, t := range types {
		rv := reflect.ValueOf(t)
		rt := rv.Type()
		if rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct {
			continue
		}
		scanStruct(rt, "")
	}
}

// scanStruct 递归扫描 struct 类型，提取 JSON tag。
func scanStruct(rt reflect.Type, parent string) {
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		// 跳过非导出字段
		if !f.IsExported() {
			continue
		}
		// 嵌入的 struct 递归扫描
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			scanStruct(f.Type, parent)
			continue
		}
		// 提取 JSON tag（取逗号前的部分）
		jsonKey := f.Tag.Get("json")
		if jsonKey == "" || jsonKey == "-" {
			jsonKey = strings.ToLower(f.Name)
		}
		if idx := strings.Index(jsonKey, ","); idx >= 0 {
			jsonKey = jsonKey[:idx]
		}
		if jsonKey == "" {
			jsonKey = strings.ToLower(f.Name)
		}
		// key = "StructName.FieldName"
		key := rt.Name() + "." + f.Name
		if parent != "" {
			// 嵌套字段
			key = parent + "." + f.Name
		}
		fieldTagMap[key] = jsonKey

		// 递归扫描嵌套 struct 字段（非匿名）
		if f.Type.Kind() == reflect.Struct {
			scanStruct(f.Type, rt.Name()+"."+f.Name)
		}
	}
}

// Translate 将 Gin ShouldBindJSON/ShouldBindQuery 返回的 error 翻译为
// 具体的 errcode.Error（msg 包含字段名和约束）。
// 非 ValidationErrors（如 JSON 解析失败）返回 BadRequest。
func Translate(err error) *errcode.Error {
	if err == nil {
		return errcode.OK
	}

	verrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return errcode.BadRequest.WithMsg("请求格式不正确")
	}

	msgs := make([]string, 0, len(verrs))
	for _, fe := range verrs {
		msgs = append(msgs, translateFieldError(fe))
		if len(msgs) >= 3 {
			break
		}
	}
	return errcode.InvalidParam.WithMsg(strings.Join(msgs, "; "))
}

// translateFieldError 翻译单条字段校验错误。
func translateFieldError(fe validator.FieldError) string {
	field := jsonFieldName(fe)
	tag := fe.Tag()
	param := fe.Param()

	switch tag {
	case "required":
		return fmt.Sprintf("%s 不能为空", field)
	case "min":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("%s 最少%s位", field, param)
		}
		return fmt.Sprintf("%s 最小值为%s", field, param)
	case "max":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("%s 最长%s位", field, param)
		}
		return fmt.Sprintf("%s 最大值为%s", field, param)
	case "len":
		return fmt.Sprintf("%s 长度必须为%s", field, param)
	case "oneof":
		opts := strings.Join(strings.Split(param, " "), " / ")
		return fmt.Sprintf("%s 仅支持 %s", field, opts)
	case "url":
		return fmt.Sprintf("%s URL格式不正确", field)
	case "email":
		return fmt.Sprintf("%s 箱格式不正确", field)
	case "gte":
		return fmt.Sprintf("%s 必须 ≥ %s", field, param)
	case "lte":
		return fmt.Sprintf("%s 必须 ≤ %s", field, param)
	case "dive":
		return fmt.Sprintf("%s 列表元素校验失败", field)
	default:
		return fmt.Sprintf("%s 不满足 %s 约束", field, tag)
	}
}

// jsonFieldName 从 FieldError 中提取 JSON tag 名。
func jsonFieldName(fe validator.FieldError) string {
	// fe.StructNamespace() = "RegisterReq.Password" 或嵌套 "Req.Inner.Field"
	ns := fe.StructNamespace()
	if name, ok := fieldTagMap[ns]; ok {
		return name
	}
	// 嵌套时 namespace 可能包含多层 struct，逐层查找
	parts := strings.Split(ns, ".")
	for i := len(parts) - 1; i >= 1; i-- {
		key := strings.Join(parts[:i+1], ".")
		if name, ok := fieldTagMap[key]; ok {
			return name
		}
	}
	// fallback: Go 字段名小写
	return strings.ToLower(fe.Field())
}