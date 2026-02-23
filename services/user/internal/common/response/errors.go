package response

type ErrorCode string

const (
	Success              ErrorCode = "01"
	ValidationError      ErrorCode = "02"
	AuthenticationFailed ErrorCode = "AUTH_401"
	AuthorizationFailed  ErrorCode = "AUTH_403"
	ResourceNotFound     ErrorCode = "04"
	InternalServerError  ErrorCode = "99"
)
