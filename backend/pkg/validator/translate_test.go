package validator

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/kleinai/backend/pkg/errcode"
)

// testDTO 用于测试翻译器。
type testDTO struct {
	Account  string `json:"account"  binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=8,max=64"`
}

func init() {
	Register(testDTO{})
}

func TestTranslate_Required(t *testing.T) {
	v := binding.Validator.Engine().(*validator.Validate)
	err := v.Struct(testDTO{})
	ec := Translate(err)
	if ec.Code != errcode.InvalidParam.Code {
		t.Fatalf("expected code %d, got %d", errcode.InvalidParam.Code, ec.Code)
	}
	if ec.Msg != "account 不能为空; password 不能为空" {
		t.Fatalf("expected specific msg, got: %s", ec.Msg)
	}
}

func TestTranslate_MinLength(t *testing.T) {
	v := binding.Validator.Engine().(*validator.Validate)
	err := v.Struct(testDTO{Account: "ab", Password: "short"})
	ec := Translate(err)
	if ec.Code != errcode.InvalidParam.Code {
		t.Fatalf("expected code %d, got %d", errcode.InvalidParam.Code, ec.Code)
	}
	if ec.Msg != "account 最少3位; password 最少8位" {
		t.Fatalf("expected specific msg, got: %s", ec.Msg)
	}
}

func TestTranslate_NonValidationErr(t *testing.T) {
	ec := Translate(errcode.InvalidParam)
	if ec.Msg != "请求格式不正确" {
		t.Fatalf("expected fallback msg, got: %s", ec.Msg)
	}
}