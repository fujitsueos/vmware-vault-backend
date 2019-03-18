package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"github.com/fujitsueos/vmware-vault-backend/sts"
	"github.com/satori/go.uuid"
	"github.com/vmware/govmomi/vim25/soap"
	"log"
	"math/big"
	"net/url"
	"time"

	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
)

const (
	tokenDuration = 72 * time.Hour
	Time          = "2006-01-02T15:04:05.000Z"
	errMessage    = "Failed to acquire token"
)

func pathAccessToken(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "token",
		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.ReadOperation: b.pathAccessTokenRead,
		},
	}
}

func (b *backend) pathAccessTokenRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {

	config, err := b.getConfig(ctx, req.Storage)

	if err != nil {
		return nil, err
	}

	selfSignedCert, privateKeyPEM, sign, err := generateSelfSignedCertificate(config.Region)
	if err != nil {
		log.Printf("Cert generation error: %s", err.Error())
		return nil, fmt.Errorf(errMessage)
	}

	clientCert, err := tls.X509KeyPair(pem.EncodeToMemory(selfSignedCert), pem.EncodeToMemory(privateKeyPEM))

	u, err := soap.ParseURL(config.AuthenticationURL)
	if err != nil {
		log.Printf("Failed to parse auth url: %s", err.Error())
		return nil, fmt.Errorf(errMessage)
	}

	u.User = url.UserPassword(config.Username, config.Password)

	soapClient := soap.NewClient(u, true)

	stsClient, err := sts.NewClient(context.Background(), soapClient)
	if err != nil {
		log.Printf("Sts client error: %s", err.Error())
		return nil, fmt.Errorf(errMessage)
	}

	tokenReq := sts.TokenRequest{
		Certificate: &clientCert,
		Delegatable: true,
		Userinfo:    u.User,
		Lifetime:    tokenDuration,
	}

	s, err := stsClient.Issue(ctx, tokenReq)
	if err != nil {
		log.Printf("Failed to Issue sts token: %s", err.Error())
		return nil, fmt.Errorf(errMessage)
	}

	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)

	if _, err = gz.Write([]byte(s.Token)); err != nil {
		return nil, fmt.Errorf(errMessage)
	}
	if err := gz.Flush(); err != nil {
		return nil, fmt.Errorf(errMessage)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf(errMessage)
	}

	token := base64.StdEncoding.EncodeToString(buf.Bytes())

	resp := &logical.Response{
		Data: map[string]interface{}{
			"token":     token,
			"expires":   s.Lifetime.Expires.Format(Time),
			"signature": sign,
		},
	}
	return resp, nil
}

func generateID() string {
	return uuid.NewV4().String()
}

func generateSelfSignedCertificate(region string) (certPEM *pem.Block, keyPem *pem.Block, sign []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	san, err := asn1.Marshal([]asn1.RawValue{{
		Tag:   2,
		Class: 2,
		Bytes: []byte(region),
	}})
	if err != nil {
		return
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(tokenDuration)

	extSubjectAltName := pkix.Extension{}
	extSubjectAltName.Id = asn1.ObjectIdentifier{2, 5, 29, 17}
	extSubjectAltName.Critical = false
	extSubjectAltName.Value = san

	vsphereDomainComponent := pkix.AttributeTypeAndValue{
		Type:  asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 25},
		Value: "vsphere",
	}

	localDomainComponent := pkix.AttributeTypeAndValue{
		Type:  asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 25},
		Value: "local",
	}

	publicKey, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return
	}
	hash_data := sha256.Sum256(publicKey)

	template := x509.Certificate{
		Subject: pkix.Name{
			CommonName:         "CA",
			Country:            []string{"US"},
			Province:           []string{"California"},
			Organization:       []string{region},
			OrganizationalUnit: []string{"VMware Engineering"},
			Locality:           []string{"Palo Alto"},
			ExtraNames:         []pkix.AttributeTypeAndValue{vsphereDomainComponent, localDomainComponent},
		},
		KeyUsage:        x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageContentCommitment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		SerialNumber:    serialNumber,
		NotBefore:       notBefore,
		NotAfter:        notAfter,
		ExtraExtensions: []pkix.Extension{extSubjectAltName},
		IsCA:            true,
		SubjectKeyId:    hash_data[:],
		AuthorityKeyId:  hash_data[:],
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, key.Public(), key)
	if err != nil {
		return
	}

	randBytesHash := sha256.Sum256([]byte("{}"))

	certPEM = &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}
	keyPem = &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	sign, err = rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, randBytesHash[:])
	if err != nil {
		return
	}

	return
}
