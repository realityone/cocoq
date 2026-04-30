package proxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func NewMitmConnectAction(ca tls.Certificate) goproxy.FuncHttpsHandler {
	connectAction := &goproxy.ConnectAction{
		Action:    goproxy.ConnectMitm,
		TLSConfig: goproxy.TLSConfigFromCA(&ca),
	}

	return func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		logrus.Infof("MITM enabled for %s", host)
		return connectAction, host
	}
}

func LoadOrCreateCA(cocoqDirName, caCertFile, caKeyFile string) (tls.Certificate, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return tls.Certificate{}, errors.Wrap(err, "resolve home directory")
	}

	caDir := filepath.Join(homeDir, cocoqDirName)
	certPath := filepath.Join(caDir, caCertFile)
	keyPath := filepath.Join(caDir, caKeyFile)

	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return loadCA(certPath, keyPath)
		}
		if !os.IsNotExist(err) {
			return tls.Certificate{}, errors.Wrap(err, "stat CA private key")
		}
	} else if !os.IsNotExist(err) {
		return tls.Certificate{}, errors.Wrap(err, "stat CA certificate")
	}

	if err := os.MkdirAll(caDir, 0o700); err != nil {
		return tls.Certificate{}, errors.Wrap(err, "create cocoq directory")
	}

	return generateAndStoreCA(certPath, keyPath)
}

func loadCA(certPath, keyPath string) (tls.Certificate, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return tls.Certificate{}, errors.Wrap(err, "read CA certificate")
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return tls.Certificate{}, errors.Wrap(err, "read CA private key")
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, errors.Wrap(err, "parse CA key pair")
	}

	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return tls.Certificate{}, errors.Wrap(err, "parse CA certificate leaf")
	}

	return cert, nil
}

func generateAndStoreCA(certPath, keyPath string) (tls.Certificate, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, errors.Wrap(err, "generate private key")
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, errors.Wrap(err, "generate serial number")
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "cocoq Root CA",
			Organization: []string{"cocoq"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
		MaxPathLen:            0,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, errors.Wrap(err, "create root certificate")
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: der,
	})

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return tls.Certificate{}, errors.Wrap(err, "write CA certificate")
	}

	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return tls.Certificate{}, errors.Wrap(err, "write CA private key")
	}

	cert := tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  privateKey,
		Leaf:        template,
	}

	return cert, nil
}
