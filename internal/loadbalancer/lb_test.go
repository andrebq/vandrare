package loadbalancer_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrebq/vandrare/internal/loadbalancer"
)

func TestLoadBalancer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	lb := loadbalancer.NewLB[int](ctx)
	w1, w2 := make(chan int, 1), make(chan int, 1)
	go lb.Run(ctx)
	lb.Add(w1)
	lb.Add(w2)

	select {
	case lb.Ch() <- 1:
	case <-ctx.Done():
		t.Fatal("too slow to read")
	}
	select {
	case lb.Ch() <- 2:
	case <-ctx.Done():
		t.Fatal("too slow to read")
	}

	select {
	case <-w1:
	case <-w2:
	case <-ctx.Done():
		t.Fatal("No work received")
	}

	select {
	case <-w1:
	case <-w2:
	case <-ctx.Done():
		t.Fatal("No work received")
	}
}
