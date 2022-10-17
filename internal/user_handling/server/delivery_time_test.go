package uh_server_test

import (
	"testing"
	"time"

	uh "github.com/KSpaceer/go_watermelon/internal/user_handling/server"

	"github.com/stretchr/testify/assert"
)

func TestSetDeliveryTimeEmpty(t *testing.T) {
	testDeliveryTime := ""
	defaultHour, defaultMinute, defaultSecond := uh.GetDeliveryTime()
	err := uh.SetDeliveryTime(testDeliveryTime)
	if assert.Nil(t, err) {
		h, m, s := uh.GetDeliveryTime()
		assert.Equal(t, defaultHour, h)
		assert.Equal(t, defaultMinute, m)
		assert.Equal(t, defaultSecond, s)
	}
}

func TestSetDeliveryTimeIncorrect(t *testing.T) {
	testDeliveryTime := "55:55:55"
	defaultHour, defaultMinute, defaultSecond := uh.GetDeliveryTime()
	err := uh.SetDeliveryTime(testDeliveryTime)
	if assert.NotNil(t, err) {
		h, m, s := uh.GetDeliveryTime()
		assert.Equal(t, defaultHour, h)
		assert.Equal(t, defaultMinute, m)
		assert.Equal(t, defaultSecond, s)
	}
}

func TestSetDeliveryTimeCorrect(t *testing.T) {
	testDeliveryTime := "21:42:02"
	expectedHour, expectedMinute, expectedSecond := 21, 42, 02
	err := uh.SetDeliveryTime(testDeliveryTime)
	if assert.Nil(t, err) {
		h, m, s := uh.GetDeliveryTime()
		assert.Equal(t, expectedHour, h)
		assert.Equal(t, expectedMinute, m)
		assert.Equal(t, expectedSecond, s)
	}
}

func TestSetDeliveryIntervalEmpty(t *testing.T) {
	testDeliveryInterval := ""
	defaultInterval := uh.GetDeliveryInterval()
	err := uh.SetDeliveryInterval(testDeliveryInterval)
	if assert.Nil(t, err) {
		assert.Equal(t, defaultInterval, uh.GetDeliveryInterval())
	}
}

func TestSetDeliveryIntervalIncorrect(t *testing.T) {
	testDeliveryInterval := "55"
	defaultInterval := uh.GetDeliveryInterval()
	err := uh.SetDeliveryInterval(testDeliveryInterval)
	if assert.NotNil(t, err) {
		assert.Equal(t, defaultInterval, uh.GetDeliveryInterval())
	}
}

func TestSetDeliveryIntervalCorrect(t *testing.T) {
	testDeliveryInterval := "10h20m30s"
	expectedInterval := 10*time.Hour + 20*time.Minute + 30*time.Second
	err := uh.SetDeliveryInterval(testDeliveryInterval)
	if assert.Nil(t, err) {
		assert.Equal(t, expectedInterval, uh.GetDeliveryInterval())
	}
}
