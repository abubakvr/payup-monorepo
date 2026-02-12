module github.com/abubakvr/payup-backend/services/user

go 1.25.0

require (
	github.com/segmentio/kafka-go v0.4.47
	golang.org/x/crypto v0.48.0
)

require (
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
)

replace github.com/abubakvr/payup-backend/proto => ../../proto
