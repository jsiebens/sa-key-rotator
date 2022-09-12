package sakeyrotator

import (
	"cloud.google.com/go/storage"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/api/iam/v1"
	"math/big"
	"net/http"
	"strings"
	"time"
)

const DefaultName = "sa-key-rotator"

type Rotator struct {
	logger         *Logger
	iamService     *iam.Service
	storageService *storage.Client
}

func NewRotator(ctx context.Context, logger *Logger) (*Rotator, error) {
	iamService, err := iam.NewService(ctx)
	if err != nil {
		return nil, err
	}

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return &Rotator{
		logger:         logger,
		iamService:     iamService,
		storageService: storageClient,
	}, nil
}

func (r *Rotator) Rotate(ctx context.Context,
	serviceAccountEmail,
	name,
	bucket string,
	expiryInDays,
	renewalWindowInDays int,
	forceCreate,
	forceDelete bool) error {

	r.logger.Info("checking keys for service account", "service_account", serviceAccountEmail)

	resource := "projects/-/serviceAccounts/" + serviceAccountEmail

	now := startOfDay()
	notBefore := now
	notAfter := notBefore.AddDate(0, 0, expiryInDays)

	account, err := r.iamService.Projects.ServiceAccounts.Get(resource).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get service account %s: %w", serviceAccountEmail, err)
	}

	userManagedKeys, err := r.iamService.Projects.ServiceAccounts.Keys.List(resource).KeyTypes("USER_MANAGED").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get service account keys %s: %w", serviceAccountEmail, err)
	}

	if len(userManagedKeys.Keys) == 0 {
		r.logger.Info("no keys found, uploading a new one", "service_account", serviceAccountEmail)
		return r.uploadNewKey(ctx, account, name, bucket, notBefore, notAfter)
	}

	commonNames, err := readCommonNames(serviceAccountEmail)
	if err != nil {
		return err
	}

	var createNewKey = true
	var keysToRemove = []string{}

	for _, k := range userManagedKeys.Keys {
		id := strings.Split(k.Name, "/")[5]
		cn := commonNames[id]

		validBefore, err := time.Parse(time.RFC3339, k.ValidBeforeTime)
		if err != nil {
			return err
		}

		if (forceDelete && cn == name) || now.After(validBefore) {
			keysToRemove = append(keysToRemove, k.Name)
		}

		if cn != name {
			continue
		}

		pivotDate := validBefore.AddDate(0, 0, -renewalWindowInDays)

		if now.Before(pivotDate) {
			createNewKey = false
		}
	}

	if forceCreate || createNewKey {
		if forceCreate {
			r.logger.Info("creating and uploading a new key (forced)", "service_account", serviceAccountEmail)
		} else {
			r.logger.Info("current key is about to expire, uploading a new one", "service_account", serviceAccountEmail)
		}
		if err := r.uploadNewKey(ctx, account, name, bucket, notBefore, notAfter); err != nil {
			return err
		}
	}

	for _, k := range keysToRemove {
		if _, err := r.iamService.Projects.ServiceAccounts.Keys.Delete(k).Context(ctx).Do(); err != nil {
			r.logger.Warn("failed to delete expired key", "service_account", serviceAccountEmail, "key_id", k, "err", err)
		}
		if forceDelete {
			r.logger.Info("deleted existing key (forced)", "service_account", serviceAccountEmail, "key_id", k)
		} else {
			r.logger.Info("deleted expired key", "service_account", serviceAccountEmail, "key_id", k)
		}
	}

	if !createNewKey && len(keysToRemove) == 0 {
		r.logger.Info("nothing do to, everything is fine!", "service_account", serviceAccountEmail)
	}

	return nil
}

func (r *Rotator) uploadNewKey(ctx context.Context, account *iam.ServiceAccount, name string, bucket string, notBefore, notAfter time.Time) error {
	rawPrivateKey, publicCert, err := r.generatePrivateKeyAndCertificate(name, notBefore, notAfter)
	if err != nil {
		return err
	}
	publicString := base64.StdEncoding.EncodeToString(publicCert)

	key, err := r.iamService.Projects.ServiceAccounts.Keys.Upload(account.Name, &iam.UploadServiceAccountKeyRequest{PublicKeyData: publicString}).Context(ctx).Do()
	if err != nil {
		return err
	}

	split := strings.Split(key.Name, "/")
	keyData := map[string]string{
		"type":                        "service_account",
		"project_id":                  split[1],
		"private_key_id":              split[5],
		"private_key":                 string(rawPrivateKey),
		"client_email":                split[3],
		"client_id":                   account.UniqueId,
		"auth_uri":                    "https://accounts.google.com/o/oauth2/auth",
		"token_uri":                   "https://accounts.google.com/o/oauth2/token",
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		"client_x509_cert_url":        fmt.Sprintf("https://www.googleapis.com/robot/v1/metadata/x509/%s", split[3]),
	}

	marshal, err := json.Marshal(keyData)
	if err != nil {
		return err
	}

	jsonFileName := fmt.Sprintf("%s-%s.json", notBefore.Format("2006-02-01"), split[5][:10])
	writer := r.storageService.Bucket(bucket).Object(jsonFileName).NewWriter(ctx)

	if _, err := writer.Write(marshal); err != nil {
		return fmt.Errorf("failed to store service account key in bucket %s: %w", bucket, err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to store service account key in bucket %s: %w", bucket, err)
	}

	r.logger.Info("uploaded a new key",
		"service_account", split[3],
		"key_id", split[5],
		"valid_from", notBefore.Format(time.RFC3339),
		"valid_to", notAfter.Format(time.RFC3339))

	return nil
}

func (r *Rotator) generatePrivateKeyAndCertificate(name string, notBefore, notAfter time.Time) ([]byte, []byte, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		Subject: pkix.Name{
			CommonName: name,
		},
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}
	rawPrivateKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, privateKey.Public(), privateKey)
	if err != nil {
		return nil, nil, err
	}

	rawPublicKey := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	return rawPrivateKey, rawPublicKey, nil
}

func readCommonNames(serviceAccountEmail string) (map[string]string, error) {
	getJson := func(url string, target interface{}) error {
		c := http.Client{Timeout: 5 * time.Second}
		r, err := c.Get(url)
		if err != nil {
			return err
		}
		defer r.Body.Close()

		return json.NewDecoder(r.Body).Decode(target)
	}

	m := make(map[string]string)

	if err := getJson(fmt.Sprintf("https://www.googleapis.com/robot/v1/metadata/x509/%s", serviceAccountEmail), &m); err != nil {
		return nil, err
	}

	for k, v := range m {
		name, err := extractCommonName(v)
		if err != nil {
			return nil, err
		}
		m[k] = name
	}

	return m, nil
}

func extractCommonName(cert string) (string, error) {
	p, _ := pem.Decode([]byte(cert))
	certificate, err := x509.ParseCertificate(p.Bytes)
	if err != nil {
		return "", err
	}
	return certificate.Subject.CommonName, nil
}
