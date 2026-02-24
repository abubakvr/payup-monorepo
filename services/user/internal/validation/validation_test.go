package validation

import (
	"testing"

	"github.com/abubakvr/payup-backend/services/user/internal/dto"
	"github.com/go-playground/validator/v10"
)

// testValidator uses the same tag as Gin ("binding") so DTO rules match.
var testValidator = func() *validator.Validate {
	v := validator.New()
	v.SetTagName("binding")
	return v
}()

func TestRegisterRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     dto.RegisterRequest
		wantErr bool
	}{
		{
			name: "valid",
			req: dto.RegisterRequest{
				Email:       "user@example.com",
				Password:    "password123",
				FirstName:   "John",
				LastName:    "Doe",
				PhoneNumber: "2348012345678",
			},
			wantErr: false,
		},
		{
			name: "missing email",
			req: dto.RegisterRequest{
				Password:    "password123",
				FirstName:   "John",
				LastName:    "Doe",
				PhoneNumber: "2348012345678",
			},
			wantErr: true,
		},
		{
			name: "invalid email",
			req: dto.RegisterRequest{
				Email:       "not-an-email",
				Password:    "password123",
				FirstName:   "John",
				LastName:    "Doe",
				PhoneNumber: "2348012345678",
			},
			wantErr: true,
		},
		{
			name: "password too short",
			req: dto.RegisterRequest{
				Email:       "user@example.com",
				Password:    "short",
				FirstName:   "John",
				LastName:    "Doe",
				PhoneNumber: "2348012345678",
			},
			wantErr: true,
		},
		{
			name: "missing first name",
			req: dto.RegisterRequest{
				Email:       "user@example.com",
				Password:    "password123",
				LastName:    "Doe",
				PhoneNumber: "2348012345678",
			},
			wantErr: true,
		},
		{
			name: "name too long",
			req: dto.RegisterRequest{
				Email:       "user@example.com",
				Password:    "password123",
				FirstName:   string(make([]byte, 101)),
				LastName:    "Doe",
				PhoneNumber: "2348012345678",
			},
			wantErr: true,
		},
		{
			name: "phone too short",
			req: dto.RegisterRequest{
				Email:       "user@example.com",
				Password:    "password123",
				FirstName:   "John",
				LastName:    "Doe",
				PhoneNumber: "123",
			},
			wantErr: true,
		},
		{
			name: "empty body",
			req:  dto.RegisterRequest{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := testValidator.Struct(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterRequest validation: err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				fieldErrs, ok := FormatValidationErrors(err)
				if !ok || len(fieldErrs) == 0 {
					t.Errorf("FormatValidationErrors: expected field errors, got ok=%v len=%d", ok, len(fieldErrs))
				}
			}
		})
	}
}

func TestLoginRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     dto.LoginRequest
		wantErr bool
	}{
		{
			name: "valid",
			req: dto.LoginRequest{
				Email:    "user@example.com",
				Password: "anypassword",
			},
			wantErr: false,
		},
		{
			name: "missing email",
			req: dto.LoginRequest{
				Password: "anypassword",
			},
			wantErr: true,
		},
		{
			name: "invalid email",
			req: dto.LoginRequest{
				Email:    "bad",
				Password: "anypassword",
			},
			wantErr: true,
		},
		{
			name: "missing password",
			req: dto.LoginRequest{
				Email: "user@example.com",
			},
			wantErr: true,
		},
		{
			name: "empty",
			req:  dto.LoginRequest{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := testValidator.Struct(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoginRequest validation: err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestForgotPasswordRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     dto.ForgotPasswordRequest
		wantErr bool
	}{
		{
			name: "valid",
			req:  dto.ForgotPasswordRequest{Email: "user@example.com"},
			wantErr: false,
		},
		{
			name:    "missing email",
			req:     dto.ForgotPasswordRequest{},
			wantErr: true,
		},
		{
			name:    "invalid email",
			req:     dto.ForgotPasswordRequest{Email: "not-email"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := testValidator.Struct(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ForgotPasswordRequest validation: err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_messageForTag(t *testing.T) {
	tests := []struct {
		tag   string
		param string
		want  string
	}{
		{"required", "", "is required"},
		{"email", "", "must be a valid email address"},
		{"min", "8", "must be at least 8 characters"},
		{"max", "72", "must be at most 72 characters"},
		{"unknown", "x", "is invalid (unknown x)"},
	}
	for _, tt := range tests {
		got := messageForTag(tt.tag, tt.param)
		if got != tt.want {
			t.Errorf("messageForTag(%q, %q) = %q, want %q", tt.tag, tt.param, got, tt.want)
		}
	}
}
