package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/abubakvr/payup-backend/services/kyc/internal/crypto"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

const metadataEncrypted = "x-payup-encrypted"
const metadataContentType = "content-type"

// S3Config holds S3 settings for KYC selfie, identity, and address verification uploads.
type S3Config struct {
	Bucket                  string
	Region                  string
	Prefix                  string // e.g. "kyc/selfies"
	IdentityPrefix          string // e.g. "kyc/identity"
	AddressVerificationPrefix string // e.g. "kyc/address_verification"
	Endpoint                string
	EncryptionKey           string
}

// LoadS3ConfigFromEnv loads S3 config from environment.
// These vars are used only for KYC S3 verification (selfie upload/download).
// Required: KYC_S3_BUCKET, KYC_S3_REGION, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY.
// Optional: KYC_S3_PREFIX, KYC_S3_ENDPOINT, KYC_ENCRYPTION_KEY (or ENCRYPTION_KEY).
func LoadS3ConfigFromEnv() S3Config {
	bucket := os.Getenv("KYC_S3_BUCKET")
	region := os.Getenv("KYC_S3_REGION")
	if region == "" {
		region = "us-east-1"
	}
	prefix := os.Getenv("KYC_S3_PREFIX")
	if prefix == "" {
		prefix = "kyc/selfies"
	}
	identityPrefix := os.Getenv("KYC_S3_IDENTITY_PREFIX")
	if identityPrefix == "" {
		identityPrefix = "kyc/identity"
	}
	addrVerPrefix := os.Getenv("KYC_S3_ADDRESS_VERIFICATION_PREFIX")
	if addrVerPrefix == "" {
		addrVerPrefix = "kyc/address_verification"
	}
	encKey := os.Getenv("KYC_ENCRYPTION_KEY")
	if encKey == "" {
		encKey = os.Getenv("ENCRYPTION_KEY")
	}
	return S3Config{
		Bucket:                    bucket,
		Region:                    region,
		Prefix:                    strings.Trim(path.Clean(prefix), "/"),
		IdentityPrefix:            strings.Trim(path.Clean(identityPrefix), "/"),
		AddressVerificationPrefix: strings.Trim(path.Clean(addrVerPrefix), "/"),
		Endpoint:                  os.Getenv("KYC_S3_ENDPOINT"),
		EncryptionKey:             encKey,
	}
}

// S3Uploader uploads KYC selfie images to S3.
type S3Uploader struct {
	client *s3.Client
	cfg    S3Config
}

// NewS3Uploader creates an S3 uploader. Returns nil if cfg.Bucket is empty (upload will be no-op).
func NewS3Uploader(cfg S3Config) (*S3Uploader, error) {
	if cfg.Bucket == "" {
		return nil, nil
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	// Use static credentials only for KYC S3 verification (not default chain).
	// Region must be set explicitly so SDK does not resolve an empty/invalid region.
	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(region))
	if id, secret := os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"); id != "" && secret != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			id,
			secret,
			os.Getenv("AWS_SESSION_TOKEN"), // optional, for temporary creds
		)))
	}
	awsCfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("s3 config: %w", err)
	}

	var clientOpts []func(*s3.Options)
	if cfg.Endpoint != "" {
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	}

	return &S3Uploader{
		client: s3.NewFromConfig(awsCfg, clientOpts...),
		cfg:    cfg,
	}, nil
}

// decodeBase64Image decodes base64 image and returns body and content type (image/jpeg or image/png).
func decodeBase64Image(b64 string) ([]byte, string, error) {
	// Strip data URL prefix if present
	const prefix = "base64,"
	if i := strings.Index(strings.ToLower(b64), prefix); i >= 0 {
		b64 = b64[i+len(prefix):]
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, "", fmt.Errorf("base64 decode: %w", err)
	}
	if len(raw) < 12 {
		return nil, "", fmt.Errorf("image too short")
	}
	// Detect content type from magic bytes
	contentType := "image/jpeg"
	if len(raw) >= 8 && raw[0] == 0x89 && raw[1] == 0x50 && raw[2] == 0x4E {
		contentType = "image/png"
	}
	return raw, contentType, nil
}

// UploadSelfie uploads a base64-encoded selfie to S3 and returns the public URL.
// Object key format: prefix/profileID/uuid.jpg or .png.
func (u *S3Uploader) UploadSelfie(ctx context.Context, profileID string, imageBase64 string) (string, error) {
	if u == nil || u.client == nil {
		return "", nil
	}
	if imageBase64 == "" {
		return "", nil
	}

	body, contentType, err := decodeBase64Image(imageBase64)
	if err != nil {
		return "", err
	}

	ext := ".jpg"
	if contentType == "image/png" {
		ext = ".png"
	}
	key := fmt.Sprintf("%s/%s/%s%s", u.cfg.Prefix, profileID, uuid.New().String(), ext)

	uploadBody := body
	metadata := map[string]string{
		metadataContentType: contentType,
	}
	if u.cfg.EncryptionKey != "" {
		encrypted, err := crypto.Encrypt(body, u.cfg.EncryptionKey)
		if err != nil {
			return "", fmt.Errorf("encrypt selfie: %w", err)
		}
		uploadBody = encrypted
		metadata[metadataEncrypted] = "1"
	}

	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(u.cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(uploadBody),
		ContentType: aws.String(contentType),
		Metadata:    metadata,
	}
	if u.cfg.EncryptionKey != "" {
		putInput.ContentType = aws.String("application/octet-stream")
	}
	_, err = u.client.PutObject(ctx, putInput)
	if err != nil {
		return "", fmt.Errorf("s3 put object: %w", err)
	}

	return u.buildURL(key), nil
}

// UploadIdentityImage uploads an identity document image to S3 (encrypted when EncryptionKey set) and returns the object URL.
// imageType: id_front, id_back, customer_image, signature.
func (u *S3Uploader) UploadIdentityImage(ctx context.Context, profileID, imageType string, body []byte, contentType string) (string, error) {
	if u == nil || u.client == nil {
		return "", nil
	}
	if len(body) == 0 {
		return "", nil
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}
	ext := ".jpg"
	if contentType == "image/png" {
		ext = ".png"
	}
	key := fmt.Sprintf("%s/%s/%s-%s%s", u.cfg.IdentityPrefix, profileID, imageType, uuid.New().String(), ext)
	uploadBody := body
	metadata := map[string]string{metadataContentType: contentType}
	if u.cfg.EncryptionKey != "" {
		encrypted, err := crypto.Encrypt(body, u.cfg.EncryptionKey)
		if err != nil {
			return "", fmt.Errorf("encrypt identity image: %w", err)
		}
		uploadBody = encrypted
		metadata[metadataEncrypted] = "1"
	}
	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(u.cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(uploadBody),
		ContentType: aws.String(contentType),
		Metadata:    metadata,
	}
	if u.cfg.EncryptionKey != "" {
		putInput.ContentType = aws.String("application/octet-stream")
	}
	if _, err := u.client.PutObject(ctx, putInput); err != nil {
		return "", fmt.Errorf("s3 put object: %w", err)
	}
	return u.buildURL(key), nil
}

// UploadAddressVerificationImage uploads utility bill or proof-of-address image to S3 (encrypted when EncryptionKey set). imageType: utility_bill, proof_of_address.
func (u *S3Uploader) UploadAddressVerificationImage(ctx context.Context, profileID, imageType string, body []byte, contentType string) (string, error) {
	if u == nil || u.client == nil {
		return "", fmt.Errorf("s3 uploader not configured")
	}
	if len(body) == 0 {
		return "", fmt.Errorf("image body is empty")
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}
	ext := ".jpg"
	if contentType == "image/png" {
		ext = ".png"
	}
	key := fmt.Sprintf("%s/%s/%s-%s%s", u.cfg.AddressVerificationPrefix, profileID, imageType, uuid.New().String(), ext)
	uploadBody := body
	metadata := map[string]string{metadataContentType: contentType}
	if u.cfg.EncryptionKey != "" {
		encrypted, err := crypto.Encrypt(body, u.cfg.EncryptionKey)
		if err != nil {
			return "", fmt.Errorf("encrypt address verification image: %w", err)
		}
		uploadBody = encrypted
		metadata[metadataEncrypted] = "1"
	}
	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(u.cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(uploadBody),
		ContentType: aws.String(contentType),
		Metadata:    metadata,
	}
	if u.cfg.EncryptionKey != "" {
		putInput.ContentType = aws.String("application/octet-stream")
	}
	if _, err := u.client.PutObject(ctx, putInput); err != nil {
		return "", fmt.Errorf("s3 put object: %w", err)
	}
	return u.buildURL(key), nil
}

// DeleteObject deletes the object at objectURL (e.g. to remove old image on re-upload).
func (u *S3Uploader) DeleteObject(ctx context.Context, objectURL string) error {
	if u == nil || u.client == nil {
		return nil
	}
	if objectURL == "" {
		return nil
	}
	key, err := u.keyFromURL(objectURL)
	if err != nil {
		return err
	}
	_, err = u.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(u.cfg.Bucket),
		Key:    aws.String(key),
	})
	return err
}

// buildURL returns the public URL for the object (standard S3 or custom endpoint).
func (u *S3Uploader) buildURL(key string) string {
	if u.cfg.Endpoint != "" {
		return strings.TrimSuffix(u.cfg.Endpoint, "/") + "/" + u.cfg.Bucket + "/" + key
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", u.cfg.Bucket, u.cfg.Region, key)
}

// keyFromURL extracts the S3 object key from a URL we produced (buildURL).
func (u *S3Uploader) keyFromURL(objectURL string) (string, error) {
	parsed, err := url.Parse(objectURL)
	if err != nil {
		return "", err
	}
	pathStr := strings.TrimPrefix(parsed.Path, "/")
	if pathStr == "" {
		return "", fmt.Errorf("invalid object URL: no path")
	}
	if u.cfg.Endpoint != "" {
		prefix := u.cfg.Bucket + "/"
		if strings.HasPrefix(pathStr, prefix) {
			return pathStr[len(prefix):], nil
		}
		return pathStr, nil
	}
	return pathStr, nil
}

// GetSelfie downloads the object at objectURL and returns decrypted body and content-type.
// If the object was stored with client-side encryption (EncryptionKey set), it is decrypted automatically.
func (u *S3Uploader) GetSelfie(ctx context.Context, objectURL string) ([]byte, string, error) {
	return u.GetObjectByURL(ctx, objectURL)
}

// GetObjectByURL downloads the S3 object at objectURL and returns decrypted body and content-type.
// Used for admin image download (identity, address verification, selfie). Decrypts when metadata indicates encryption.
func (u *S3Uploader) GetObjectByURL(ctx context.Context, objectURL string) ([]byte, string, error) {
	if u == nil || u.client == nil {
		return nil, "", fmt.Errorf("s3 uploader not configured")
	}
	if objectURL == "" {
		return nil, "", fmt.Errorf("object URL is empty")
	}
	key, err := u.keyFromURL(objectURL)
	if err != nil {
		return nil, "", err
	}

	out, err := u.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(u.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("s3 get object: %w", err)
	}
	defer out.Body.Close()

	body, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := ""
	if out.Metadata != nil {
		if v, ok := out.Metadata[metadataContentType]; ok {
			contentType = v
		}
	}
	if contentType == "" && out.ContentType != nil {
		contentType = *out.ContentType
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}

	encrypted := false
	if out.Metadata != nil {
		if v, ok := out.Metadata[metadataEncrypted]; ok && v == "1" {
			encrypted = true
		}
	}
	if encrypted && u.cfg.EncryptionKey != "" {
		decrypted, err := crypto.Decrypt(body, u.cfg.EncryptionKey)
		if err != nil {
			return nil, "", fmt.Errorf("decrypt object: %w", err)
		}
		body = decrypted
	}

	return body, contentType, nil
}

