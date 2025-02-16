package validations

import (
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/loging"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"

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
	err := en_translations.RegisterDefaultTranslations(Validate, Trans)
	if err != nil {
		loging.Logger.Error(err)
	}
	err = Validate.RegisterTranslation("required", Trans, func(ut ut.Translator) error {
		return ut.Add("required", "{0} must have a value!", true) // see universal-translator for details
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("required", fe.Field())

		return t
	})
	if err != nil {
		loging.Logger.Error(err)
	}

	err = Validate.RegisterValidation("alphanum_", func(fl validator.FieldLevel) bool {
		value := fl.Field().String()
		for _, char := range value {
			if !unicode.IsLetter(char) && !unicode.IsNumber(char) && char != '_' {
				return false
			}
		}
		return true
	})
	if err != nil {
		loging.Logger.Error(err)
	}

	err = Validate.RegisterValidation("valid_resources", func(fl validator.FieldLevel) bool {
		value := fl.Field().String()
		return slices.Contains(constants.Resources, value)
	})
	if err != nil {
		loging.Logger.Error(err)
	}
	err = Validate.RegisterValidation("valid_actions", func(fl validator.FieldLevel) bool {
		value := fl.Field().String()
		return slices.Contains(constants.Actions, constants.Action(strings.ToLower(value)))
	})
	if err != nil {
		loging.Logger.Error(err)
	}

	err = Validate.RegisterValidation("valid_scopes", func(fl validator.FieldLevel) bool {
		value := fl.Field().String()
		return slices.Contains(constants.Scopes, constants.Scope(strings.ToLower(value)))
	})
	if err != nil {
		loging.Logger.Error(err)
	}
}
