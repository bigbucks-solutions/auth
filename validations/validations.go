package validations

import (
	"encoding/json"
	"fmt"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
)

// use a single instance , it caches struct info
var (
	Uni      *ut.UniversalTranslator
	Validate *validator.Validate
	Trans    ut.Translator
)

type ValidationErrors struct {
	Errors map[string]string `json:"errors"`
}

// NewErrorDict Constructor for ValidationErrors
func NewErrorDict() *ValidationErrors {
	var err ValidationErrors
	err.Errors = make(map[string]string)
	return &err
}

func (val *ValidationErrors) Error() string {
	jsonString, _ := json.Marshal(val.Errors)
	return string(jsonString)
}

func (wraperr *ValidationErrors) GetErrorTranslations(origerr error) {
	trans, _ := Uni.GetTranslator("en")
	fmt.Println("New")
	for _, err := range origerr.(validator.ValidationErrors) {
		wraperr.Errors[err.Field()] = err.Translate(trans)
	}
}

func InitializeValidations() {
	en := en.New()
	Uni = ut.New(en, en)

	// this is usually know or extracted from http 'Accept-Language' header
	// also see uni.FindTranslator(...)
	Trans, _ = Uni.GetTranslator("en")

	Validate = validator.New()
	en_translations.RegisterDefaultTranslations(Validate, Trans)
	Validate.RegisterTranslation("required", Trans, func(ut ut.Translator) error {
		return ut.Add("required", "{0} must have a value!", true) // see universal-translator for details
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("required", fe.Field())

		return t
	})
}
