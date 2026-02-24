package validation

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// FieldError is a single validation error (Zod/Yup-style field error).
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// messageForTag returns a human-readable message for a validator tag (Zod/Yup-style).
func messageForTag(tag, param string) string {
	switch tag {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		if param != "" {
			return "must be at least " + param + " characters"
		}
		return "is too short"
	case "max":
		if param != "" {
			return "must be at most " + param + " characters"
		}
		return "is too long"
	case "len":
		return "has invalid length"
	case "gte", "lte", "oneof", "uuid", "url", "alphanum":
		if param != "" {
			return "is invalid (" + tag + ": " + param + ")"
		}
		return "is invalid (" + tag + ")"
	default:
		if param != "" {
			return "is invalid (" + tag + " " + param + ")"
		}
		return "is invalid (" + tag + ")"
	}
}

// FormatValidationErrors converts validator.ValidationErrors into a slice of FieldError with readable messages.
func FormatValidationErrors(err error) ([]FieldError, bool) {
	var valErr validator.ValidationErrors
	if !errors.As(err, &valErr) {
		return nil, false
	}
	out := make([]FieldError, 0, len(valErr))
	for _, e := range valErr {
		msg := messageForTag(e.Tag(), e.Param())
		out = append(out, FieldError{
			Field:   e.Field(),
			Message: msg,
		})
	}
	return out, true
}

// ValidationErrorResponse sends 400 with structured field errors (Zod/Yup-style).
func ValidationErrorResponse(ctx *gin.Context, code string, fieldErrors []FieldError) {
	ctx.JSON(http.StatusBadRequest, gin.H{
		"status":        "error",
		"message":       "Validation failed",
		"responseCode":  code,
		"errors":        fieldErrors,
	})
}

// BindAndValidate runs ShouldBindJSON and, on error, sends a validation response if applicable.
// Returns true if binding succeeded, false if the response was already sent (validation or JSON error).
func BindAndValidate(ctx *gin.Context, code string, req any) bool {
	if err := ctx.ShouldBindJSON(req); err != nil {
		if fieldErrors, ok := FormatValidationErrors(err); ok {
			ValidationErrorResponse(ctx, code, fieldErrors)
			return false
		}
		// JSON syntax or other bind error
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":       "error",
			"message":      "Invalid request body",
			"responseCode": code,
		})
		return false
	}
	return true
}
