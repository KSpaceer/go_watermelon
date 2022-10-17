package uh_server

import (
	"fmt"
	"regexp"
	"time"
)

/***************************************
    This file contains functions to
    manipulate the delivery time of
           daily messages.
     Delivery time variables are
        defined in server.go
***************************************/

var (
	timePattern = regexp.MustCompile(`^([0-1]?[0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$`)

	// delivery* variables are used to define the time
	// for sending daily messages to users.
	deliveryHour                   = 12
	deliveryMinute                 = 0
	deliverySecond                 = 0
	deliveryInterval time.Duration = 24 * time.Hour
)

// SetDeliveryTime changes the delivery time according to the given string in
// HH:MM:SS format. If the string is empty, it returns nil error and doesn't change the time.
// Otherwise, if the string doesn't match format, it returns an error and also doesn't change time.
func SetDeliveryTime(newDeliveryTime string) error {
	if newDeliveryTime == "" {
		return nil
	}
	if !timePattern.MatchString(newDeliveryTime) {
		return fmt.Errorf("New time doesn't match hh:mm:ss pattern.")
	}
	_, err := fmt.Sscanf(newDeliveryTime, "%d:%d:%d", &deliveryHour, &deliveryMinute, &deliverySecond)
	if err != nil {
		return err
	}
	return nil
}

// GetDeliveryTime returns current values of the hour, minute and second of the delivery time.
func GetDeliveryTime() (int, int, int) {
	return deliveryHour, deliveryMinute, deliverySecond
}

// SetDeliveryInterval changes the delivery interval using time.ParseDuration function with given string.
// If the string is empty, it returns nil error and doesn't change the interval.
// Otherwise, if time.ParseDuration returns an error, it also doesn't change the interval.
func SetDeliveryInterval(newDeliveryInterval string) error {
	if newDeliveryInterval == "" {
		return nil
	}
	newDuration, err := time.ParseDuration(newDeliveryInterval)
	if err != nil {
		return err
	}
	deliveryInterval = newDuration
	return nil
}

// GetDeliveryInterval returns current valye of the delivery interval.
func GetDeliveryInterval() time.Duration {
	return deliveryInterval
}
