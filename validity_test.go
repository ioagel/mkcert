// Copyright 2018 The mkcert Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestParseValidity(t *testing.T) {
	tests := []struct {
		input       string
		wantErr     bool
		checkDur    func(validityDuration) bool
		description string
	}{
		{
			input: "10y",
			checkDur: func(v validityDuration) bool {
				return v.years == 10 && v.months == 0 && v.days == 0 && v.timeDur == 0
			},
			description: "10 years",
		},
		{
			input: "2y3m",
			checkDur: func(v validityDuration) bool {
				return v.years == 2 && v.months == 3 && v.days == 0 && v.timeDur == 0
			},
			description: "2 years 3 months",
		},
		{
			input: "2y 3m",
			checkDur: func(v validityDuration) bool {
				return v.years == 2 && v.months == 3 && v.days == 0 && v.timeDur == 0
			},
			description: "2 years 3 months with space",
		},
		{
			input: "2 years 3 months",
			checkDur: func(v validityDuration) bool {
				return v.years == 2 && v.months == 3 && v.days == 0 && v.timeDur == 0
			},
			description: "2 years 3 months full words",
		},
		{
			input: "1y6mo15d",
			checkDur: func(v validityDuration) bool {
				return v.years == 1 && v.months == 6 && v.days == 15 && v.timeDur == 0
			},
			description: "composite 1y6mo15d",
		},
		{
			input: "825d",
			checkDur: func(v validityDuration) bool {
				return v.years == 0 && v.months == 0 && v.days == 825 && v.timeDur == 0
			},
			description: "825 days",
		},
		{
			input: "30d",
			checkDur: func(v validityDuration) bool {
				return v.years == 0 && v.months == 0 && v.days == 30 && v.timeDur == 0
			},
			description: "30 days",
		},
		{
			input: "24h",
			checkDur: func(v validityDuration) bool {
				return v.years == 0 && v.months == 0 && v.days == 0 && v.timeDur == 24*time.Hour
			},
			description: "24 hours",
		},
		{
			input: "90min",
			checkDur: func(v validityDuration) bool {
				return v.years == 0 && v.months == 0 && v.days == 0 && v.timeDur == 90*time.Minute
			},
			description: "90 minutes",
		},
		{
			input: "30s",
			checkDur: func(v validityDuration) bool {
				return v.years == 0 && v.months == 0 && v.days == 0 && v.timeDur == 30*time.Second
			},
			description: "30 seconds",
		},
		// Invalid cases
		{
			input:       "",
			wantErr:     true,
			description: "empty string",
		},
		{
			input:       "   ",
			wantErr:     true,
			description: "whitespace only",
		},
		{
			input:       "0d",
			wantErr:     true,
			description: "zero duration",
		},
		{
			input:       "0y0m0d",
			wantErr:     true,
			description: "zero composite",
		},
		{
			input:       "10",
			wantErr:     true,
			description: "missing unit",
		},
		{
			input:       "10x",
			wantErr:     true,
			description: "unknown unit",
		},
		{
			input:       "2y 3x",
			wantErr:     true,
			description: "partially unknown unit",
		},
		{
			input:       "-5d",
			wantErr:     true,
			description: "negative duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			res, err := parseValidity(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseValidity(%q) err = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && tt.checkDur != nil && !tt.checkDur(res) {
				t.Errorf("parseValidity(%q) returned unexpected result: %+v", tt.input, res)
			}
		})
	}
}

func TestApplyValidity(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	v, err := parseValidity("10y")
	if err != nil {
		t.Fatal(err)
	}
	expected10y := time.Date(2036, 1, 1, 12, 0, 0, 0, time.UTC)
	if got := v.apply(now); !got.Equal(expected10y) {
		t.Errorf("apply 10y: got %v, want %v", got, expected10y)
	}

	v2, err := parseValidity("2y3m")
	if err != nil {
		t.Fatal(err)
	}
	expected2y3m := time.Date(2028, 4, 1, 12, 0, 0, 0, time.UTC)
	if got := v2.apply(now); !got.Equal(expected2y3m) {
		t.Errorf("apply 2y3m: got %v, want %v", got, expected2y3m)
	}

	v3, err := parseValidity("30d")
	if err != nil {
		t.Fatal(err)
	}
	expected30d := time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC)
	if got := v3.apply(now); !got.Equal(expected30d) {
		t.Errorf("apply 30d: got %v, want %v", got, expected30d)
	}
}

func readCertFromPEM(path string) (*x509.Certificate, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	return x509.ParseCertificate(block.Bytes)
}

func TestCAAndCertGenerationWithValidity(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "mkcert-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test 1: Generate CA with custom validity 5y and cert with 30d
	m := &mkcert{
		CAROOT:       tmpDir,
		caValidity:   "5y",
		certValidity: "30d",
		certFile:     filepath.Join(tmpDir, "test.pem"),
		keyFile:      filepath.Join(tmpDir, "test-key.pem"),
	}

	m.loadCA()

	// Check CA cert validity
	caCertPath := filepath.Join(tmpDir, rootName)
	caCert, err := readCertFromPEM(caCertPath)
	if err != nil {
		t.Fatalf("failed to read CA cert: %v", err)
	}

	expectedCAExpire := caCert.NotBefore.AddDate(5, 0, 0)
	if !caCert.NotAfter.Equal(expectedCAExpire) {
		t.Errorf("CA NotAfter: got %v, want %v", caCert.NotAfter, expectedCAExpire)
	}

	// Generate leaf cert
	m.makeCert([]string{"example.com"})

	leafCert, err := readCertFromPEM(m.certFile)
	if err != nil {
		t.Fatalf("failed to read leaf cert: %v", err)
	}

	expectedLeafExpire := leafCert.NotBefore.AddDate(0, 0, 30)
	if !leafCert.NotAfter.Equal(expectedLeafExpire) {
		t.Errorf("Leaf cert NotAfter: got %v, want %v", leafCert.NotAfter, expectedLeafExpire)
	}
}

func TestDefaultValidity(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "mkcert-default-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	m := &mkcert{
		CAROOT:   tmpDir,
		certFile: filepath.Join(tmpDir, "default.pem"),
		keyFile:  filepath.Join(tmpDir, "default-key.pem"),
	}

	m.loadCA()

	caCert, err := readCertFromPEM(filepath.Join(tmpDir, rootName))
	if err != nil {
		t.Fatalf("failed to read CA cert: %v", err)
	}

	// Default CA validity is 10 years
	expectedCAExpire := caCert.NotBefore.AddDate(10, 0, 0)
	if !caCert.NotAfter.Equal(expectedCAExpire) {
		t.Errorf("Default CA NotAfter: got %v, want %v", caCert.NotAfter, expectedCAExpire)
	}

	// Generate leaf cert with defaults
	m.makeCert([]string{"default.example.com"})

	leafCert, err := readCertFromPEM(m.certFile)
	if err != nil {
		t.Fatalf("failed to read leaf cert: %v", err)
	}

	// Default leaf cert validity is 2 years 3 months
	expectedLeafExpire := leafCert.NotBefore.AddDate(2, 3, 0)
	if !leafCert.NotAfter.Equal(expectedLeafExpire) {
		t.Errorf("Default leaf cert NotAfter: got %v, want %v", leafCert.NotAfter, expectedLeafExpire)
	}
}

func TestCLIFlagsExecution(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "mkcert-cli-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test with -ca-validity 1y and -days 90
	certPath := filepath.Join(tmpDir, "cli.pem")
	keyPath := filepath.Join(tmpDir, "cli-key.pem")

	cmd := exec.Command("go", "run", ".", "-ca-validity", "1y", "-days", "90", "-cert-file", certPath, "-key-file", keyPath, "cli.example.com")
	cmd.Env = append(os.Environ(), "CAROOT="+tmpDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI command failed: %v, output: %s", err, out)
	}

	// Verify CA validity is 1 year
	caCert, err := readCertFromPEM(filepath.Join(tmpDir, rootName))
	if err != nil {
		t.Fatalf("failed to read CA cert: %v", err)
	}
	expectedCAExpire := caCert.NotBefore.AddDate(1, 0, 0)
	if !caCert.NotAfter.Equal(expectedCAExpire) {
		t.Errorf("CLI CA NotAfter: got %v, want %v", caCert.NotAfter, expectedCAExpire)
	}

	// Verify leaf cert validity is 90 days
	leafCert, err := readCertFromPEM(certPath)
	if err != nil {
		t.Fatalf("failed to read leaf cert: %v", err)
	}
	expectedLeafExpire := leafCert.NotBefore.AddDate(0, 0, 90)
	if !leafCert.NotAfter.Equal(expectedLeafExpire) {
		t.Errorf("CLI leaf NotAfter: got %v, want %v", leafCert.NotAfter, expectedLeafExpire)
	}
}

func TestCLIFlagsErrorHandling(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "mkcert-err-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test invalid ca-validity
	cmd := exec.Command("go", "run", ".", "-ca-validity", "invalid", "example.com")
	cmd.Env = append(os.Environ(), "CAROOT="+tmpDir)
	if err := cmd.Run(); err == nil {
		t.Error("expected error for invalid -ca-validity, got success")
	}

	// Test conflicting -days and -cert-validity
	cmd = exec.Command("go", "run", ".", "-days", "30", "-cert-validity", "60d", "example.com")
	cmd.Env = append(os.Environ(), "CAROOT="+tmpDir)
	if err := cmd.Run(); err == nil {
		t.Error("expected error for conflicting -days and -cert-validity, got success")
	}

	// Test negative -days
	cmd = exec.Command("go", "run", ".", "-days", "-5", "example.com")
	cmd.Env = append(os.Environ(), "CAROOT="+tmpDir)
	if err := cmd.Run(); err == nil {
		t.Error("expected error for negative -days, got success")
	}
}

func TestCLIEnvironmentVariables(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "mkcert-env-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	certPath := filepath.Join(tmpDir, "env.pem")
	keyPath := filepath.Join(tmpDir, "env-key.pem")

	cmd := exec.Command("go", "run", ".", "-cert-file", certPath, "-key-file", keyPath, "env.example.com")
	cmd.Env = append(os.Environ(),
		"CAROOT="+tmpDir,
		"CA_VALIDITY=3y",
		"CERT_VALIDITY=45d",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v, output: %s", err, out)
	}

	// Check CA has 3y validity
	caCert, err := readCertFromPEM(filepath.Join(tmpDir, rootName))
	if err != nil {
		t.Fatalf("failed to read CA cert: %v", err)
	}
	expectedCAExpire := caCert.NotBefore.AddDate(3, 0, 0)
	if !caCert.NotAfter.Equal(expectedCAExpire) {
		t.Errorf("Env CA NotAfter: got %v, want %v", caCert.NotAfter, expectedCAExpire)
	}

	// Check leaf has 45d validity
	leafCert, err := readCertFromPEM(certPath)
	if err != nil {
		t.Fatalf("failed to read leaf cert: %v", err)
	}
	expectedLeafExpire := leafCert.NotBefore.AddDate(0, 0, 45)
	if !leafCert.NotAfter.Equal(expectedLeafExpire) {
		t.Errorf("Env leaf NotAfter: got %v, want %v", leafCert.NotAfter, expectedLeafExpire)
	}
}
