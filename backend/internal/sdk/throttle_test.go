package sdk

import (
	"context"
	"testing"
	"time"
)

func TestAPIThrottleHonorsContext(t *testing.T) {
	throttle := &apiThrottle{interval: time.Second}
	if err := throttle.wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := throttle.wait(ctx); err == nil {
		t.Fatal("second request did not wait for the throttle interval")
	}
}
