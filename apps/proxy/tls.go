package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"time"
)

// TLS LINT
type TLS struct {
	Country    []string `json:"country"` // "GB"
	Org        []string `json:"org"`
	CommonName string   `json:"common_name"` // "*.domain.com"
}

// Config lint
type Config struct {
	Remotehost          string `json:"remote_host"`
	Localhost           string `json:"local_host"`
	Localport           int    `json:"local_port"`
	ControlHost         string `json:"control_host"`
	ControlPort         int    `json:"control_port"`
	TLS                 *TLS   `json:"tls"`
	CertFile            string `json:"cert_file"`
	RecordMessages      bool   `json:"record_messages"`
	MessageLogDirectory string `json:"message_log_directory"`
}

var config Config
var ids = 0

func genCert() ([]byte, *rsa.PrivateKey) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1653),
		Subject: pkix.Name{
			Country:      config.TLS.Country,
			Organization: config.TLS.Org,
			CommonName:   config.TLS.CommonName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		SubjectKeyId:          []byte{1, 2, 3, 4, 5},
		BasicConstraintsValid: true,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	priv, _ := rsa.GenerateKey(rand.Reader, 1024)
	pub := &priv.PublicKey
	caB, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, priv)
	if err != nil {
		fmt.Println("create ca failed", err)
	}
	return caB, priv
}

func tlsListen() (conn net.Listener, err error) {
	var cert tls.Certificate

	if config.CertFile != "" {
		cert, _ = tls.LoadX509KeyPair(fmt.Sprint(config.CertFile, ".pem"), fmt.Sprint(config.CertFile, ".key"))
	} else {
		fmt.Println("[*] Generating cert")
		caB, priv := genCert()
		cert = tls.Certificate{
			Certificate: [][]byte{caB},
			PrivateKey:  priv,
		}
	}

	conf := tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	conf.Rand = rand.Reader

	conn, err = tls.Listen("tcp", fmt.Sprint(config.Localhost, ":", config.Localport), &conf)
	return
}
