package email_server

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xhit/go-simple-mail/v2"
)

func TestDefineMainServiceLocationLocalhost(t *testing.T) {
	testHost := "localhost"
	eServer := EmailServer{}
	addr, err := eServer.defineMainServiceLocation(testHost)
	if assert.Nil(t, err) {
		assert.NotEqual(t, testHost, addr)
	}
}

func TestDefineMainServiceLocationExternalDomain(t *testing.T) {
	testHost := "https://example.com"
	eServer := EmailServer{}
	addr, err := eServer.defineMainServiceLocation(testHost)
	if assert.Nil(t, err) {
		assert.Equal(t, testHost, addr)
	}
}

func TestReadEmailInfoFileCorrect(t *testing.T) {
	testFilePath := "testdata/email_info.csv"
	eServer := EmailServer{}
	eServer.SMTPServer = mail.NewSMTPClient()
	err := eServer.readEmailInfoFile(testFilePath)
	if assert.Nil(t, err) {
		assert.Equal(t, eServer.Host, "example.com")
		assert.Equal(t, eServer.Port, 8000)
		assert.Equal(t, eServer.Username, "username")
		assert.Equal(t, eServer.Password, "password")
	}
}

func TestReadEmailInfoFileWrong(t *testing.T) {
	testFilePath := "testdata/invalid_email_info.csv"
	eServer := EmailServer{}
	eServer.SMTPServer = mail.NewSMTPClient()
	err := eServer.readEmailInfoFile(testFilePath)
	assert.NotNil(t, err)
}

func TestChooseRandomImg(t *testing.T) {
	eServer := EmailServer{}
	path, err := eServer.chooseRandomImg()
	if assert.Nil(t, err) {
		if !assert.True(t, filepath.IsAbs(path)) {
			t.Errorf("Path is not absolute: got %q", path)
		}
	}
}
